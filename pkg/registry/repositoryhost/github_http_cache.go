// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package repositoryhost

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate -header ../../../license_prefix.txt

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/gardener/docforge/pkg/osfakes/httpclient"
	"github.com/google/go-github/v43/github"
	"k8s.io/klog/v2"
)

// ParsingOptions are options when parsing
type ParsingOptions struct {
	ContentFileFormats []string `mapstructure:"extracted-files-formats"`
	Hugo               bool     `mapstructure:"hugo"`
}

type ghc struct {
	hostName      string
	client        httpclient.Client
	git           Git
	rateLimit     RateLimitSource
	repositories  Repositories
	acceptedHosts []string

	options ParsingOptions

	repositoryFiles map[string]map[string]string
}

//counterfeiter:generate . RateLimitSource

// RateLimitSource is an interface needed for faking
type RateLimitSource interface {
	RateLimits(ctx context.Context) (*github.RateLimits, *github.Response, error)
}

//counterfeiter:generate . Repositories

// Repositories is an interface needed for faking
type Repositories interface {
	ListCommits(ctx context.Context, owner, repo string, opts *github.CommitsListOptions) ([]*github.RepositoryCommit, *github.Response, error)
	Get(ctx context.Context, owner, repo string) (*github.Repository, *github.Response, error)
}

//counterfeiter:generate . Git

// Git is an interface needed for faking
type Git interface {
	GetBlobRaw(ctx context.Context, owner, repo, sha string) ([]byte, *github.Response, error)
	GetTree(ctx context.Context, owner string, repo string, sha string, recursive bool) (*github.Tree, *github.Response, error)
}

// NewGHC creates new GHC resource handler
func NewGHC(hostName string, rateLimit RateLimitSource, repositories Repositories, git Git, client httpclient.Client, acceptedHosts []string, options ParsingOptions) Interface {
	return &ghc{
		hostName:        hostName,
		client:          client,
		git:             git,
		rateLimit:       rateLimit,
		repositories:    repositories,
		acceptedHosts:   acceptedHosts,
		options:         options,
		repositoryFiles: map[string]map[string]string{},
	}
}

func (p *ghc) LoadRepository(ctx context.Context, resourceURL string) error {
	resURL, err := new(resourceURL)
	if err != nil {
		return err
	}
	refURL := resURL.ReferenceURL()
	if _, ok := p.repositoryFiles[refURL.String()]; ok {
		return nil
	}
	dirContents, _, err := p.git.GetTree(ctx, resURL.GetOwner(), resURL.GetRepo(), resURL.GetRef(), true)
	if err != nil {
		return err
	}
	repoContent := map[string]string{}
	for _, entry := range dirContents.Entries {
		if strings.HasPrefix(entry.GetPath(), "vendor") {
			continue
		}
		resource, err := refURL.GetDifferentType(entry.GetType())
		if err != nil {
			klog.Infof("failed processing %s when loading repository: %s. Skipping it", entry.GetPath(), err.Error())
			continue
		}
		resourceURL := fmt.Sprintf("%s/%s", resource, entry.GetPath())
		repoContent[resourceURL] = entry.GetSHA()
	}
	p.repositoryFiles[refURL.String()] = repoContent
	klog.Infof("Loading reference %s with %d entries", refURL.String(), len(repoContent))
	return nil
}

func (p *ghc) Tree(r URL) ([]string, error) {
	if r.GetResourceType() != "tree" {
		return nil, fmt.Errorf("expected a tree url got %s", r.String())
	}
	out := []string{}
	refURL := r.ReferenceURL().String()
	filter, err := r.GetDifferentType("blob")
	if err != nil {
		return []string{}, err
	}
	filterString := filter + "/"
	for url := range p.repositoryFiles[refURL] {
		extract := slices.ContainsFunc(p.options.ContentFileFormats, func(extention string) bool {
			return strings.HasSuffix(url, extention)
		})
		if extract && strings.HasPrefix(url, filterString) {
			out = append(out, strings.TrimPrefix(url, filterString))
		}
	}
	return out, nil
}

func (p *ghc) ResourceURL(resourceURL string) (*URL, error) {
	resource, err := new(resourceURL)
	if err != nil {
		return nil, err
	}
	if _, ok := p.repositoryFiles[resource.ReferenceURL().String()][resource.ResourceURL()]; !ok {
		return nil, ErrResourceNotFound(resourceURL)
	}
	return resource, nil
}

func (p *ghc) ResolveRelativeLink(sourceResource URL, relativeLink string) (string, error) {
	blobURL, treeURL, err := sourceResource.ResolveRelativeLink(relativeLink)
	if err != nil {
		return "", err
	}
	if _, err := p.ResourceURL(treeURL); err == nil {
		return treeURL, nil
	}
	if _, err := p.ResourceURL(blobURL); err == nil {
		return blobURL, nil
	}
	return blobURL, ErrResourceNotFound(fmt.Sprintf("%s with source %s", relativeLink, sourceResource.String()))
}

func (p *ghc) Read(ctx context.Context, r URL) ([]byte, error) {
	if r.GetResourceType() != "blob" && r.GetResourceType() != "raw" {
		return nil, fmt.Errorf("not a blob/raw url: %s", r.String())
	}
	refURL := r.ReferenceURL().String()
	SHA := p.repositoryFiles[refURL][r.ResourceURL()]
	raw, resp, err := p.git.GetBlobRaw(ctx, r.GetOwner(), r.GetRepo(), SHA)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, ErrResourceNotFound(r.String())
		}
		return nil, err
	}
	if resp != nil && resp.StatusCode >= 400 {
		return nil, fmt.Errorf("reading blob %s fails with HTTP status: %d", r.String(), resp.StatusCode)
	}
	return raw, nil
}

// Name returns host name
func (p *ghc) Name() string {
	return p.hostName
}

func (p *ghc) Accept(link string) bool {
	r, err := url.Parse(link)
	if err != nil || r.Scheme != "https" {
		return false
	}
	for _, h := range p.acceptedHosts {
		if h == r.Host {
			return true
		}
	}
	return false
}

func (p *ghc) GetClient() httpclient.Client {
	return p.client
}

func (p *ghc) GetRateLimit(ctx context.Context) (int, int, time.Time, error) {
	r, _, err := p.rateLimit.RateLimits(ctx)
	if err != nil {
		return -1, -1, time.Now(), err
	}
	return r.Core.Limit, r.Core.Remaining, r.Core.Reset.Time, nil
}

func (p *ghc) Repositories() Repositories {
	return p.repositories
}
