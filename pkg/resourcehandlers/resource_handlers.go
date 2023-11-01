// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package resourcehandlers

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate -header ../../license_prefix.txt

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/gardener/docforge/pkg/httpclient"
)

// ErrResourceNotFound indicated that a resource was not found
type ErrResourceNotFound string

func (e ErrResourceNotFound) Error() string {
	return fmt.Sprintf("resource %q not found", string(e))
}

// ResourceHandler does resource specific operations on a type of objects
// identified by an uri schema that it accepts to handle
//
//counterfeiter:generate . ResourceHandler
type ResourceHandler interface {
	//ManifestFromURL Gets the manifest content from a given url
	ManifestFromURL(url string) (string, error)
	//FileTreeFromURL Get files that are present in the given url tree
	FileTreeFromURL(url string) ([]string, error)
	//BuildAbsLink Builds the abs link given where it is referenced
	BuildAbsLink(source, link string) (string, error)
	// Accept accepts manifests if this ResourceHandler can manage the type of resources identified by the URI scheme of uri.
	Accept(uri string) bool
	// Read a resource content at uri into a byte array
	Read(ctx context.Context, uri string) ([]byte, error)
	// ReadGitInfo reads git info for the resource
	ReadGitInfo(ctx context.Context, uri string) ([]byte, error)
	// GetRawFormatLink returns a link to an embeddable object (image) in raw format.
	// If the provided link is not referencing an embeddable object, the function returns absLink without changes.
	GetRawFormatLink(absLink string) (string, error)
	// GetClient returns an HTTP client for accessing handler's resources
	GetClient() httpclient.Client
	// GetRateLimit returns rate limit and remaining API calls for the resource handler backend (e.g. GitHub RateLimit)
	// returns negative values if RateLimit is not applicable
	GetRateLimit(ctx context.Context) (int, int, time.Time, error)
}

// Registry can register and return resource handlers for an url
//
//counterfeiter:generate . Registry
type Registry interface {
	Load(rhs ...ResourceHandler)
	Get(uri string) ResourceHandler
	Remove(rh ...ResourceHandler)
}

type registry struct {
	handlers []ResourceHandler
}

// NewRegistry creates Registry object, optionally loading it with
// resourceHandlers if provided
func NewRegistry(resourceHandlers ...ResourceHandler) Registry {
	r := &registry{
		handlers: []ResourceHandler{},
	}
	if len(resourceHandlers) > 0 {
		r.Load(resourceHandlers...)
	}
	return r
}

// ResourceHandlerOptions options for the resource handler
type ResourceHandlerOptions struct {
	CacheHomeDir     string            `mapstructure:"cache-dir"`
	Credentials      map[string]string `mapstructure:"github-oauth-token-map"`
	ResourceMappings map[string]string `mapstructure:"resourceMappings"`
	Hugo             bool              `mapstructure:"hugo"`
}

// Credential holds repository credential data
type Credential struct {
	Host       string
	OAuthToken string
}

// Load loads a ResourceHandler into the Registry
func (r *registry) Load(rhs ...ResourceHandler) {
	r.handlers = append(r.handlers, rhs...)
}

// Get returns an appropriate handler for this type of URIs if anyone those registered accepts it (its Accepts method returns true).
func (r *registry) Get(uri string) ResourceHandler {
	for _, h := range r.handlers {
		if h.Accept(uri) {
			return h
		}
	}
	return nil
}

// Remove removes a ResourceHandler from registry. If no argument is provided
// the method will remove all registered handlers
func (r *registry) Remove(resourceHandlers ...ResourceHandler) {
	if len(resourceHandlers) == 0 {
		r.handlers = []ResourceHandler{}
	}
	var idx []int
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
