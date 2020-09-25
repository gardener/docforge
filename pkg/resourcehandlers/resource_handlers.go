package resourcehandlers

import (
	"context"
	"reflect"

	"github.com/gardener/docforge/pkg/api"
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

// Registry can register and return resource handlers
// for a url
type Registry interface {
	Load(rhs ...ResourceHandler)
	Get(uri string) ResourceHandler
	Remove(rh ...ResourceHandler)
}

type registry struct {
	handlers []ResourceHandler
}

// NewRegistry creates Registry object, optionally loading it with
// resurceHanbdlers if provided
func NewRegistry(resourceHandlers ...ResourceHandler) Registry {
	r := &registry{
		handlers: []ResourceHandler{},
	}
	if len(resourceHandlers) > 0 {
		r.Load(resourceHandlers...)
	}
	return r
}

// Load loads a ResourceHandler into the Registry
func (r *registry) Load(rhs ...ResourceHandler) {
	r.handlers = append(r.handlers, rhs...)
}

// Get returns an appropriate handler for this type of URIs if any
// one those registered accepts it (its Accepts method returns true).
func (r *registry) Get(uri string) ResourceHandler {
	for _, h := range r.handlers {
		if h.Accept(uri) {
			return h
		}
	}
	return nil
}

// Remove removes a ResourceHandler from regsitry. If no argument is provided
// themethod will remove all registered ahdnlers
func (r *registry) Remove(resourceHandlers ...ResourceHandler) {
	if len(resourceHandlers) == 0 {
		r.handlers = []ResourceHandler{}
	}
	idx := []int{}
	rhs := append([]ResourceHandler{}, resourceHandlers...)
	for _, rh := range rhs {
		if i := indexOf(rh, r.handlers); i > -1 {
			idx = append(idx, i)
		}
	}
	for _, i := range idx {
		remove(r.handlers, i)
	}
}

func indexOf(r ResourceHandler, rhs []ResourceHandler) int {
	var idx = -1
	for i, _r := range rhs {
		if reflect.DeepEqual(_r, r) {
			return i
		}
	}
	return idx
}

func remove(rhs []ResourceHandler, i int) []ResourceHandler {
	rhs[len(rhs)-1], rhs[i] = rhs[i], rhs[len(rhs)-1]
	return rhs[:len(rhs)-1]
}
