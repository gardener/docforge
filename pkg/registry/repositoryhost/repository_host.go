// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package repositoryhost

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate -header ../../../license_prefix.txt

import (
	"context"
	"fmt"
	"time"

	"github.com/gardener/docforge/pkg/osfakes/httpclient"
)

// ErrResourceNotFound indicated that a resource was not found
type ErrResourceNotFound string

// Error returns "resource r not found" error
func (e ErrResourceNotFound) Error() string {
	return fmt.Sprintf("resource %q not found", string(e))
}

// Interface does resource specific operations on a type of objects
// identified by an uri schema that it accepts to handle
//
//counterfeiter:generate . Interface
type Interface interface {
	// ResourceURL returns a valid resource url object from a string url
	ResourceURL(resourceURL string) (*URL, error)
	// ResolveRelativeLink resolves a relative link given a source resource url
	ResolveRelativeLink(source URL, relativeLink string) (string, error)
	// LoadRepository loads the content of the repository of a given url
	LoadRepository(ctx context.Context, resourceURL string) error
	// Tree returns files that are present in the given url tree
	Tree(resource URL) ([]string, error)
	// Accept accepts manifests if this RepositoryHost can manage the type of resources identified by the URI scheme of uri.
	Accept(link string) bool
	// Read a resource content at uri into a byte array
	Read(ctx context.Context, resource URL) ([]byte, error)
	// Name of repository host
	Name() string
	// Repositories returns the repositories object
	Repositories() Repositories
	// GetClient returns an HTTP client for accessing handler's resources
	GetClient() httpclient.Client
	// GetRateLimit returns rate limit and remaining API calls for the resource handler backend (e.g. GitHub RateLimit)
	// returns negative values if RateLimit is not applicable
	GetRateLimit(ctx context.Context) (int, int, time.Time, error)
}

// InitOptions options for the resource handler
type InitOptions struct {
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
