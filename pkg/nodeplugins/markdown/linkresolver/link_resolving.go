// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package linkresolver

import (
	"cmp"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gardener/docforge/cmd/hugo"
	"github.com/gardener/docforge/pkg/internal/link"
	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	"k8s.io/klog/v2"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate -header ../../../../license_prefix.txt

// ErrStripLink is returned by ResolveResourceLink when a relative link points to a file
// not included in the manifest. The renderer should strip the link and keep only the label text.
type ErrStripLink struct {
	Destination string
}

func (e ErrStripLink) Error() string {
	return "link destination not in manifest: " + e.Destination
}

// Interface resolves links URLs
//
//counterfeiter:generate . Interface

// Interface represent link resolving interface
type Interface interface {
	ResolveResourceLink(destination string, node *manifest.Node, source string) (string, error)
}

// LinkResolver represents link resolving nessesary objects
type LinkResolver struct {
	Repositoryhosts registry.Interface
	SourceToNode    map[string][]*manifest.Node
	Hugo            hugo.Hugo
}

// New creates a new linkresolver given the manifest structure and a registry used for working with links
func New(structure []*manifest.Node, rhs registry.Interface, hugo hugo.Hugo) *LinkResolver {
	lr := &LinkResolver{
		Repositoryhosts: rhs,
		Hugo:            hugo,
		SourceToNode:    make(map[string][]*manifest.Node),
	}
	for _, node := range structure {
		if node.Source != "" {
			lr.SourceToNode[node.Source] = append(lr.SourceToNode[node.Source], node)
		} else if len(node.MultiSource) > 0 {
			for _, s := range node.MultiSource {
				lr.SourceToNode[s] = append(lr.SourceToNode[s], node)
			}
		}
	}
	return lr
}

// ResolveResourceLink resolves resource link from a given source
func (l *LinkResolver) ResolveResourceLink(resourceLink string, node *manifest.Node, source string) (string, error) {
	// fragment-only links (e.g. #heading) are same-document anchors — pass through unchanged
	if strings.HasPrefix(resourceLink, "#") {
		return resourceLink, nil
	}
	wasRelative := repositoryhost.IsRelative(resourceLink)
	// handle relative links to resources
	if wasRelative {
		var err error
		if srcURL, e := l.Repositoryhosts.ResourceURL(source); e == nil {
			resourceLink = ReAnchorRootAbsolute(resourceLink, srcURL.GetResourcePath(), l.Hugo.HugoStructuralDirs)
		}
		// making resourceLink to be resourceURL
		resourceLink, err = l.Repositoryhosts.ResolveRelativeLink(source, resourceLink)
		if err != nil {
			if _, ok := err.(repositoryhost.ErrResourceNotFound); ok {
				klog.V(6).Infof("stripping relative link %s from source %s — target not found in repository", resourceLink, source)
				return "", ErrStripLink{Destination: resourceLink}
			}
			return resourceLink, err
		}
	}
	destinationResource, err := l.Repositoryhosts.ResourceURL(resourceLink)
	if err != nil {
		return resourceLink, fmt.Errorf("error when parsing resource link %s in %s : %w", resourceLink, source, err)
	}
	destinationResourceURL := destinationResource.ResourceURL()
	destinationNode, err := l.resolveDestinationNode(destinationResourceURL, node)
	if destinationNode == nil {
		if err != nil {
			return resourceLink, err
		}
		if wasRelative {
			klog.V(6).Infof("stripping relative link %s (resolved to %s) — not found in manifest", resourceLink, destinationResourceURL)
			return "", ErrStripLink{Destination: resourceLink}
		}
		klog.V(6).Infof("passing through absolute link %s — not found in manifest", destinationResourceURL)
		return resourceLink, nil
	}

	// construct destination from node path
	websiteLink := destinationNode.NodePath()
	if l.Hugo.Enabled {
		websiteLink = destinationNode.HugoPrettyPath()
	}
	for _, structuralDir := range l.Hugo.HugoStructuralDirs {
		websiteLink = strings.TrimPrefix(websiteLink, structuralDir+"/")
	}
	if destinationResource.GetResourceSuffix() != "" {
		return link.Build("/", l.Hugo.BaseURL, websiteLink, destinationResource.GetResourceSuffix())
	}
	return link.Build("/", l.Hugo.BaseURL, websiteLink)
}

func (l *LinkResolver) resolveDestinationNode(destinationResourceURL string, node *manifest.Node) (*manifest.Node, error) {
	// check if link refers to a node
	nl, ok := l.SourceToNode[destinationResourceURL]
	if !ok {
		return nil, nil
	}
	// found nodes with this source -> find the shortest path from l.node to one of nodes
	destinationNode := slices.MinFunc(nl, func(a, b *manifest.Node) int {
		relPathBetweenNodeAndA, _ := filepath.Rel(node.Path, a.NodePath())
		relPathBetweenNodeAndB, _ := filepath.Rel(node.Path, b.NodePath())
		return cmp.Compare(strings.Count(relPathBetweenNodeAndA, "/"), strings.Count(relPathBetweenNodeAndB, "/"))
	})

	desiredPath, ok := node.LinkResolution[destinationResourceURL]
	if !ok {
		return destinationNode, nil
	}
	// resolve linkResolution override
	candidateNodes := slices.DeleteFunc(nl, func(element *manifest.Node) bool {
		return element.NodePath() != desiredPath
	})
	if len(candidateNodes) != 1 {
		return nil, fmt.Errorf("node with path %s's LinkResolution of %s field maps to %d nodes", node.NodePath(), destinationResourceURL, len(candidateNodes))
	}
	return candidateNodes[0], nil
}
