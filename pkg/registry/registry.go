// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gardener/docforge/pkg/osfakes/httpclient"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	"k8s.io/klog/v2"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate -header ../../license_prefix.txt

// Interface can register and return resource repoHosts for an url
//
//counterfeiter:generate . Interface
type Interface interface {
	// ResolveRelativeLink resolves a relative link from a given source
	ResolveRelativeLink(source string, relativeLink string) (string, error)
	// LoadRepository loads the repository content from a given resource url
	LoadRepository(ctx context.Context, resourceURL string) error
	// Tree returns files that are present in the given url tree
	Tree(resourceURL string) ([]string, error)
	// Read a resource content at uri into a byte array
	Read(ctx context.Context, resourceURL string) ([]byte, error)
	// ReadGitInfo reads the git info for a given resource URL
	ReadGitInfo(ctx context.Context, resourceURL string) ([]byte, error)
	// Client returns an HTTP client for accessing the given url
	Client(url string) httpclient.Client
	// ResourceURL returns a valid resource url object from a string url
	ResourceURL(resourceURL string) (*repositoryhost.URL, error)
	// LogRateLimits logs rate limit and remaining API calls for all resource handler backends
	LogRateLimits(ctx context.Context)
}

type registry struct {
	repoHosts []repositoryhost.Interface
}

// NewRegistry creates Registry object, optionally loading it with resourcerepoHosts if provided
func NewRegistry(resourcerepoHosts ...repositoryhost.Interface) Interface {
	return &registry{repoHosts: resourcerepoHosts}
}

func (r *registry) Client(url string) httpclient.Client {
	rh, _, err := r.anyRepositoryHost(url)
	if err != nil {
		return http.DefaultClient
	}
	return rh.GetClient()
}

func (r *registry) Tree(resourceURL string) ([]string, error) {
	rh, url, err := r.anyRepositoryHost(resourceURL)
	if err != nil {
		return []string{}, err
	}
	return rh.Tree(*url)
}

func (r *registry) Read(ctx context.Context, resourceURL string) ([]byte, error) {
	rh, url, err := r.anyRepositoryHost(resourceURL)
	if err != nil {
		return []byte{}, err
	}
	return rh.Read(ctx, *url)
}

func (r *registry) ResolveRelativeLink(source string, relativeLink string) (string, error) {
	rh, url, err := r.anyRepositoryHost(source)
	if err != nil {
		return "", err
	}
	return rh.ResolveRelativeLink(*url, relativeLink)
}

func (r *registry) ReadGitInfo(ctx context.Context, resourceURL string) ([]byte, error) {
	rh, url, err := r.githubRepositoryHost(resourceURL)
	if err != nil {
		return []byte{}, err
	}
	return repositoryhost.ReadGitInfo(ctx, rh.Repositories(), *url)
}

func (r *registry) LoadRepository(ctx context.Context, resourceURL string) error {
	rh, err := r.acceptGithubRH(resourceURL)
	if err != nil {
		if err.Error() == fmt.Sprintf("no sutiable repository host for %s", resourceURL) {
			return nil
		}
		return err
	}
	return rh.LoadRepository(ctx, resourceURL)
}

func (r *registry) anyRepositoryHost(resourceURL string) (repositoryhost.Interface, *repositoryhost.URL, error) {
	rh, err := r.acceptAnyRH(resourceURL)
	if err != nil {
		return nil, nil, err
	}
	url, err := rh.ResourceURL(resourceURL)
	if err != nil {
		return nil, nil, err
	}
	return rh, url, nil
}

func (r *registry) ResourceURL(resourceURL string) (*repositoryhost.URL, error) {
	_, url, err := r.anyRepositoryHost(resourceURL)
	return url, err
}

func (r *registry) githubRepositoryHost(resourceURL string) (repositoryhost.Interface, *repositoryhost.URL, error) {
	rh, err := r.acceptGithubRH(resourceURL)
	if err != nil {
		return nil, nil, err
	}
	//rh.LoadRepository(context.TODO(),resourceURL)
	url, err := rh.ResourceURL(resourceURL)
	if err != nil {
		return nil, nil, err
	}
	return rh, url, nil
}

func (r *registry) acceptAnyRH(uri string) (repositoryhost.Interface, error) {
	for _, h := range r.repoHosts {
		if h.Accept(uri) {
			return h, nil
		}
	}
	return nil, fmt.Errorf("no sutiable repository host for %s", uri)
}

func (r *registry) acceptGithubRH(uri string) (repositoryhost.Interface, error) {
	for _, h := range r.repoHosts {
		if h.Repositories() != nil && h.Accept(uri) {
			return h, nil
		}
	}
	return nil, fmt.Errorf("no sutiable repository host for %s", uri)
}

func (r *registry) LogRateLimits(ctx context.Context) {
	for _, repoHost := range r.repoHosts {
		l, rr, rt, err := repoHost.GetRateLimit(ctx)
		if err != nil && err.Error() != "not implemented" {
			klog.Warningf("Error getting RateLimit for %s: %v\n", repoHost.Name(), err)
		} else if l > 0 && rr > 0 {
			klog.Infof("%s RateLimit: %d requests per hour, Remaining: %d, Reset after: %s\n", repoHost.Name(), l, rr, time.Until(rt).Round(time.Second))
		}
	}
}
