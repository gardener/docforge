// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package linkresolver

import (
	"cmp"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gardener/docforge/cmd/hugo"
	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/readers/repositoryhosts"
	"github.com/gardener/docforge/pkg/readers/resource"
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
func (l *LinkResolver) ResolveLink(link string, node *manifest.Node, source string) (string, bool, error) {
	escapedEmoji := strings.ReplaceAll(link, "/:v:/", "/%3Av%3A/")
	if escapedEmoji != link {
		klog.Warningf("escaping : for /:v:/ in link %s for source %s ", link, source)
		link = escapedEmoji
	}
	linkURL, err := url.Parse(link)
	if err != nil {
		return "", false, fmt.Errorf("error when parsing link in %s : %w", source, err)
	}
	shouldValidate := true
	// resolve outside links
	if linkURL.IsAbs() {
		if _, err := l.Repositoryhosts.Get(link); err != nil {
			// we don't have a handler for it. Leave it be.
			return link, true, nil
		}
	} else {
		// convert destination to absolute link
		docHandler, err := l.Repositoryhosts.Get(source)
		if err != nil {
			return "", false, fmt.Errorf("unexpected error - can't get a handler for already read content: %w", err)
		}
		if link, err = docHandler.ToAbsLink(source, link); err != nil {
			if _, ok := err.(repositoryhosts.ErrResourceNotFound); !ok {
				return "", false, err
			}
			klog.Warningf("failed to validate absolute link for %s from source %s: %v\n", link, source, err)
			shouldValidate = false
		}
	}
	// destination is absolute URL from a repository host
	if !resource.IsResourceURL(link) {
		// link is from repository host but not a resource
		return link, shouldValidate, nil
	}
	// destination is resource URL
	linkURL, err = url.Parse(link)
	if err != nil {
		return "", false, fmt.Errorf("unexpected error when parsing link %s in %s : %w", link, source, err)
	}
	destinationResource, err := resource.FromURL(linkURL)
	if err != nil {
		return "", false, fmt.Errorf("unexpected error when parsing link %s in %s : %w", link, source, err)
	}
	destinationResourceURL := destinationResource.String()
	// check if link refers to a node
	nl, ok := l.SourceToNode[destinationResourceURL]
	if !ok {
		return link, shouldValidate, nil
	}
	// found nodes with this source -> find the shortest path from l.node to one of nodes
	destinationNode := slices.MinFunc(nl, func(a, b *manifest.Node) int {
		relPathBetweenNodeAndA, _ := filepath.Rel(node.Path, a.NodePath())
		relPathBetweenNodeAndB, _ := filepath.Rel(node.Path, a.NodePath())
		return cmp.Compare(strings.Count(relPathBetweenNodeAndA, "/"), strings.Count(relPathBetweenNodeAndB, "/"))
	})
	// construct destination from node path
	link = strings.ToLower(destinationNode.NodePath())
	if l.Hugo.Enabled {
		link = strings.ToLower(destinationNode.HugoPrettyPath())
	}
	link = fmt.Sprintf("/%s/", path.Join(l.Hugo.BaseURL, link))
	if linkURL.ForceQuery || linkURL.RawQuery != "" {
		link = fmt.Sprintf("%s?%s", link, linkURL.RawQuery)
	}
	if linkURL.Fragment != "" {
		link = fmt.Sprintf("%s#%s", link, linkURL.Fragment)
	}
	return link, true, nil
}
