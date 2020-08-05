package backend

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
	// Read a node content into a byte array ready for serialization
	Read(ctx context.Context, node *api.Node) ([]byte, error)
	// Name resolves the name of the resource from a URI
	// Example: https://github.com/owner/repo/tree/master/a/b/c.md -> c.md
	Name(uri string) string
}

// ResourceHandlers is a registry for ResourceHandler objects
type ResourceHandlers []ResourceHandler

// Get returns an appropriate handler for this type of URIs if any
// one those registered accepts it (its Accepts method returns true).
func (r ResourceHandlers) Get(uri string) ResourceHandler {
	for _, h := range r {
		if h.Accept(uri) {
			return h
		}
	}
	return nil
}
