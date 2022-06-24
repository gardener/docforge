// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/gardener/docforge/pkg/reactor"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/resourcehandlers/pg"
	"github.com/gardener/docforge/pkg/util/osshim"
	"github.com/gardener/docforge/pkg/writers"
	"github.com/google/go-github/v43/github"
	"github.com/gregjones/httpcache"
	"github.com/gregjones/httpcache/diskcache"
	"github.com/hashicorp/go-multierror"
	"github.com/peterbourgon/diskv"
	"golang.org/x/oauth2"
)

// NewReactor creates a Reactor from Options
func NewReactor(o *Options, rhs []resourcehandlers.ResourceHandler) (*reactor.Reactor, error) {

	hugo := &reactor.Hugo{
		Enabled:        o.Hugo,
		PrettyURLs:     o.HugoPrettyUrls,
		BaseURL:        o.HugoBaseURL,
		IndexFileNames: o.FlagsHugoSectionFiles,
	}

	opt := &reactor.Options{
		DocumentWorkersCount:         o.DocumentWorkersCount,
		ValidationWorkersCount:       o.ValidationWorkersCount,
		FailFast:                     o.FailFast,
		DestinationPath:              o.DestinationPath,
		ResourcesPath:                o.ResourcesPath,
		ResourceDownloadWorkersCount: o.ResourceDownloadWorkersCount,
		ResourceHandlers:             rhs,
		Resolve:                      o.Resolve,
		ManifestPath:                 o.DocumentationManifestPath,
		Hugo:                         hugo,
	}

	if o.DryRun {
		opt.DryRunWriter = writers.NewDryRunWritersFactory(os.Stdout)
		opt.Writer = opt.DryRunWriter.GetWriter(opt.DestinationPath)
		opt.ResourceDownloadWriter = opt.DryRunWriter.GetWriter(filepath.Join(opt.DestinationPath, opt.ResourcesPath))
	} else {
		opt.Writer = &writers.FSWriter{
			Root: opt.DestinationPath,
			Hugo: opt.Hugo.Enabled,
		}
		opt.ResourceDownloadWriter = &writers.FSWriter{
			Root: filepath.Join(opt.DestinationPath, opt.ResourcesPath),
		}
	}

	if len(o.GhInfoDestination) > 0 {
		opt.GitInfoWriter = &writers.FSWriter{
			Root: filepath.Join(opt.DestinationPath, o.GhInfoDestination),
			Ext:  "json",
		}
	}

	return reactor.NewReactor(opt)
}

func initResourceHandlers(ctx context.Context, o *Options) ([]resourcehandlers.ResourceHandler, error) {
	var rhs []resourcehandlers.ResourceHandler
	var errs *multierror.Error
	for _, cred := range o.Credentials {
		instance := cred.Host
		if !strings.HasPrefix(instance, "https://") && !strings.HasPrefix(instance, "http://") {
			instance = "https://" + instance
		}
		u, err := url.Parse(instance)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("couldn't parse url: %s", instance))
			continue
		}
		cachePath := filepath.Join(o.CacheHomeDir, "diskv", cred.Host)
		client, httpClient, err := buildClient(ctx, cred.OAuthToken, instance, cachePath)
		if err != nil {
			errs = multierror.Append(errs, err)
		}
		rh := newResourceHandler(u.Host, o.CacheHomeDir, &cred.Username, cred.OAuthToken, client, httpClient, o.UseGit, o.ResourceMappings, o.Variables, o.Hugo)
		rhs = append(rhs, rh)
	}

	return rhs, errs.ErrorOrNil()
}

// TODO: remove unused params
func newResourceHandler(host, homeDir string, user *string, token string, client *github.Client, httpClient *http.Client, useGit bool, localMappings map[string]string, flagVars map[string]string, hugoEnabled bool) resourcehandlers.ResourceHandler {
	rawHost := "raw." + host
	if host == "github.com" {
		rawHost = "raw.githubusercontent.com"
	}

	//	if useGit { TODO: remove unused resource handlers
	//		return git.NewResourceHandler(filepath.Join(homeDir, git.CacheDir), user, token, client, httpClient, []string{host, rawHost}, localMappings, branchesMap, flagVars)
	//	}
	//	return ghrs.NewResourceHandler(client, httpClient, []string{host, rawHost}, branchesMap, flagVars)

	return pg.NewPG(client, httpClient, &osshim.OsShim{}, []string{host, rawHost}, localMappings, flagVars, hugoEnabled)
}

func buildClient(ctx context.Context, accessToken string, host string, cachePath string) (*github.Client, *http.Client, error) {
	base := http.DefaultTransport
	if len(accessToken) > 0 {
		// if token provided replace base RoundTripper
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: accessToken})
		base = oauth2.NewClient(ctx, ts).Transport
	}

	flatTransform := func(s string) []string { return []string{} }
	d := diskv.New(diskv.Options{
		BasePath:     cachePath,
		Transform:    flatTransform,
		CacheSizeMax: 1024 * 1024 * 1024,
	})

	cacheTransport := &httpcache.Transport{
		Transport:           base,
		Cache:               diskcache.NewWithDiskv(d),
		MarkCachedResponses: true,
	}

	httpClient := cacheTransport.Client()

	var (
		client *github.Client
		err    error
	)

	if host == "https://github.com" {
		client = github.NewClient(httpClient)
		return client, httpClient, nil
	}
	client, err = github.NewEnterpriseClient(host, "", httpClient)
	return client, httpClient, err
}
