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
	"github.com/gardener/docforge/pkg/readers/link"
	"github.com/gardener/docforge/pkg/readers/repositoryhosts"
	"k8s.io/klog/v2"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate -header ../../../license_prefix.txt

// Interface resolves links URLs
//
//counterfeiter:generate . Interface

// Interface represent link resolving interface
type Interface interface {
	ResolveLink(destination string, node *manifest.Node, source string) (string, bool, error)
}

// LinkResolver represents link resolving nessesary objects
type LinkResolver struct {
	Repositoryhosts repositoryhosts.Registry
	SourceToNode    map[string][]*manifest.Node
	Hugo            hugo.Hugo
}

// ResolveLink resolves link
func (l *LinkResolver) ResolveLink(destination string, node *manifest.Node, source string) (string, bool, error) {
	escapedEmoji := strings.ReplaceAll(destination, "/:v:/", "/%3Av%3A/")
	if escapedEmoji != destination {
		klog.Warningf("escaping : for /:v:/ in link %s for source %s ", destination, source)
		destination = escapedEmoji
	}
	destinationResource, err := link.NewResource(destination)
	if err != nil {
		return "", false, fmt.Errorf("error when parsing link in %s : %w", source, err)
	}
	shouldValidate := true
	// resolve outside links
	if destinationResource.IsAbs() {
		if _, err := l.Repositoryhosts.Get(destination); err != nil {
			// we don't have a handler for it. Leave it be.
			return destination, true, nil
		}
	} else {
		// convert destination to absolute link
		docHandler, err := l.Repositoryhosts.Get(source)
		if err != nil {
			return "", false, fmt.Errorf("unexpected error - can't get a handler for already read content: %w", err)
		}
		if destination, err = docHandler.ToAbsLink(source, destination); err != nil {
			if _, ok := err.(repositoryhosts.ErrResourceNotFound); !ok {
				return "", false, err
			}
			klog.Warningf("failed to validate absolute link for %s from source %s: %v\n", destination, source, err)
			shouldValidate = false
		}
	}
	// destination is absolute URL from a repository host
	destinationResource, err = link.NewResource(destination)
	if err != nil {
		return "", false, fmt.Errorf("error when parsing link in %s : %w", source, err)
	}
	destinationResourceURL, err := destinationResource.ToResourceURL()
	if err != nil {
		// link is from repository host but not a resource
		return destination, shouldValidate, nil
	}
	// check if link refers to a node
	nl, ok := l.SourceToNode[destinationResourceURL]
	if !ok {
		return destination, shouldValidate, nil
	}
	// found nodes with this source -> find the shortest path from l.node to one of nodes
	destinationNode := slices.MinFunc(nl, func(a, b *manifest.Node) int {
		relPathBetweenNodeAndA, _ := filepath.Rel(node.Path, a.NodePath())
		relPathBetweenNodeAndB, _ := filepath.Rel(node.Path, a.NodePath())
		return cmp.Compare(strings.Count(relPathBetweenNodeAndA, "/"), strings.Count(relPathBetweenNodeAndB, "/"))
	})
	// construct destination from node path
	destination = strings.ToLower(destinationNode.NodePath())
	if l.Hugo.Enabled {
		destination = strings.ToLower(destinationNode.HugoPrettyPath())
	}
	destination = fmt.Sprintf("/%s/", path.Join(l.Hugo.BaseURL, destination))
	if destinationResource.ForceQuery || destinationResource.RawQuery != "" {
		destination = fmt.Sprintf("%s?%s", destination, destinationResource.RawQuery)
	}
	if destinationResource.Fragment != "" {
		destination = fmt.Sprintf("%s#%s", destination, destinationResource.Fragment)
	}
	return destination, true, nil
}
