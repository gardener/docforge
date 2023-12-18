// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package repositoryhosts

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate -header ../../../license_prefix.txt

import (
	"context"
	"fmt"
	"time"

	"github.com/gardener/docforge/pkg/osfakes/httpclient"
)

// ErrResourceNotFound indicated that a resource was not found
type ErrResourceNotFound string

func (e ErrResourceNotFound) Error() string {
	return fmt.Sprintf("resource %q not found", string(e))
}

// RepositoryHost does resource specific operations on a type of objects
// identified by an uri schema that it accepts to handle
//
//counterfeiter:generate . RepositoryHost
type RepositoryHost interface {
	//ManifestFromURL Gets the manifest content from a given url
	ManifestFromURL(url string) (string, error)
	//FileTreeFromURL Get files that are present in the given url tree
	FileTreeFromURL(url string) ([]string, error)
	//ToAbsLink Builds the abs link given where it is referenced
	ToAbsLink(source, link string) (string, error)
	// Accept accepts manifests if this RepositoryHost can manage the type of resources identified by the URI scheme of uri.
	Accept(uri string) bool
	// Read a resource content at uri into a byte array
	Read(ctx context.Context, uri string) ([]byte, error)
	// ReadGitInfo reads git info for the resource
	ReadGitInfo(ctx context.Context, uri string) ([]byte, error)
	// GetRawFormatLink returns a link to an embeddable object (image) in raw format.
	// If the provided link is not referencing an embeddable object, the function returns absLink without changes.
	GetRawFormatLink(absLink string) (string, error)
	// Name of repository host
	Name() string
	// GetClient returns an HTTP client for accessing handler's resources
	GetClient() httpclient.Client
	// GetRateLimit returns rate limit and remaining API calls for the resource handler backend (e.g. GitHub RateLimit)
	// returns negative values if RateLimit is not applicable
	GetRateLimit(ctx context.Context) (int, int, time.Time, error)
}

// RepositoryHostOptions options for the resource handler
type RepositoryHostOptions struct {
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
