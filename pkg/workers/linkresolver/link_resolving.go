package linkresolver

import (
	"fmt"
	"path"
	"path/filepath"
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
type Interface interface {
	ResolveLink(destination string, node *manifest.Node, source string) (string, bool, error)
}

// extends DocumentWorker with current source URI & node
type LinkResolver struct {
	Repositoryhosts repositoryhosts.Registry
	SourceToNode    map[string][]*manifest.Node
	Hugo            hugo.Hugo
}

// return empty string if failed
func (l *LinkResolver) ResolveLink(destination string, node *manifest.Node, source string) (string, bool, error) {
	newDestination := strings.ReplaceAll(destination, "/:v:/", "/%3Av%3A/")
	if newDestination != destination {
		klog.Warningf("escaping : for /:v:/ in link %s for source %s ", destination, source)
		destination = newDestination
	}

	destinationResource, err := link.NewResource(destination)
	if err != nil {
		return "", false, fmt.Errorf("error when parsing link in %s : %w", source, err)
	}
	// resolve outside links
	if destinationResource.IsAbs() {
		if _, err := l.Repositoryhosts.Get(destination); err != nil {
			// we don't have a handler for it. Leave it be.
			return destination, true, nil
		}
	}
	shouldValidate := true
	// convert destination to absolute link
	if !destinationResource.IsAbs() {
		docHandler, err := l.Repositoryhosts.Get(source)
		if err != nil {
			return "", false, fmt.Errorf("Unexpected error - can't get a handler for already read content: %w", err)
		}
		if destination, err = docHandler.ToAbsLink(source, destination); err != nil {
			if _, ok := err.(repositoryhosts.ErrResourceNotFound); !ok {
				return "", false, err
			}
			klog.Warningf("failed to validate absolute link for %s from source %s: %v\n", destination, source, err)
			shouldValidate = false
		}
	}
	destinationResource, err = link.NewResource(destination)
	if err != nil {
		return "", false, fmt.Errorf("error when parsing link in %s : %w", source, err)
	}
	// check if link refers to a node
	nl, ok := l.SourceToNode[destinationResource.GetResourceURL()]
	if !ok {
		return destination, shouldValidate, nil
	}
	// found nodes with this source -> find the shortest path from l.node to one of nodes
	minLength := -1
	var destinationNode *manifest.Node
	for _, n := range nl {
		// TODO: n = findVisibleNode(n) is relative link broken?
		relPathBetweenNodes, _ := filepath.Rel(node.Path, n.NodePath())
		pathLength := strings.Count(relPathBetweenNodes, "/")
		if pathLength < minLength || minLength == -1 {
			minLength = pathLength
			destinationNode = n
		}
	}
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
