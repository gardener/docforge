// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package linkresolver

import (
	"cmp"
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gardener/docforge/cmd/hugo"
	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	"k8s.io/klog/v2"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate -header ../../../license_prefix.txt

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

// ResolveResourceLink resolves resource link from a given source
func (l *LinkResolver) ResolveResourceLink(resourceLink string, node *manifest.Node, source string) (string, error) {
	// handle relative links to resources
	if repositoryhost.IsRelative(resourceLink) {
		var err error
		// making resourceLink to be resourceURL
		resourceLink, err = l.Repositoryhosts.ResolveRelativeLink(source, resourceLink)
		if err != nil {
			if _, ok := err.(repositoryhost.ErrResourceNotFound); ok {
				klog.Warningf("failed to validate absolute link for %s from source %s: %v\n", resourceLink, source, err)
				// don't process broken link and don't return error
				return resourceLink, nil
			}
			return resourceLink, err
		}
	}
	destinationResource, err := l.Repositoryhosts.ResourceURL(resourceLink)
	if err != nil {
		return resourceLink, fmt.Errorf("error when parsing resource link %s in %s : %w", resourceLink, source, err)
	}
	destinationResourceURL := destinationResource.ResourceURL()
	// check if link refers to a node
	nl, ok := l.SourceToNode[destinationResourceURL]
	if !ok {
		return resourceLink, nil
	}
	// found nodes with this source -> find the shortest path from l.node to one of nodes
	destinationNode := slices.MinFunc(nl, func(a, b *manifest.Node) int {
		relPathBetweenNodeAndA, _ := filepath.Rel(node.Path, a.NodePath())
		relPathBetweenNodeAndB, _ := filepath.Rel(node.Path, b.NodePath())
		return cmp.Compare(strings.Count(relPathBetweenNodeAndA, "/"), strings.Count(relPathBetweenNodeAndB, "/"))
	})
	// construct destination from node path
	websiteLink := strings.ToLower(destinationNode.NodePath())
	if l.Hugo.Enabled {
		websiteLink = strings.ToLower(destinationNode.HugoPrettyPath())
	}
	return fmt.Sprintf("/%s/%s", path.Join(l.Hugo.BaseURL, websiteLink), destinationResource.GetResourceSuffix()), nil
}
