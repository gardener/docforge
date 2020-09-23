package resourcehandlers

import (
	"context"

	"github.com/gardener/docode/pkg/api"
)

// ResourceHandler does resource specific operations on a type of objects
// identified by an uri schem that it accepts to handle
type ResourceHandler interface {
	// Accepts manifests if this ResourceHandler can manage the type of resources
	// identified by the URI scheme of uri.
	Accept(uri string) bool
	// ResolveNodeSelector resolves the NodeSelector rules of a Node into subnodes
	// hierarchy (Node.Nodes)
	ResolveNodeSelector(ctx context.Context, node *api.Node) error
	// Read a resource content at uri into a byte array
	Read(ctx context.Context, uri string) ([]byte, error)
	// Name resolves the name of the resource from a URI
	// Example: https://github.com/owner/repo/tree/master/a/b/c.md -> c.md
	Name(uri string) string
	// ResolveRelLink should return an absolute path of a relative link in regards of the provided
	// source
	BuildAbsLink(source, link string) (string, error)
	// GetLocalityDomainCandidate ...
	GetLocalityDomainCandidate(source string) (key, path, version string, err error)
	// SetVersion
	SetVersion(absLink, version string) (string, error)
}

// ResourceHandlers is a registry for ResourceHandler objects
var resourceHandlers []ResourceHandler

// Get returns an appropriate handler for this type of URIs if any
// one those registered accepts it (its Accepts method returns true).
func Get(uri string) ResourceHandler {
	for _, h := range resourceHandlers {
		if h.Accept(uri) {
			return h
		}
	}
	return nil
}

// Load loads a ResourceHandler into the registry
func Load(r ...ResourceHandler) {
	resourceHandlers = append(resourceHandlers, r...)
}
