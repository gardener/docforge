// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/gardener/docforge/pkg/api"
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

func initResourceHandlers(ctx context.Context, o resourcehandlers.ResourceHandlerOptions, options api.ParsingOptions) ([]resourcehandlers.ResourceHandler, error) {
	var rhs []resourcehandlers.ResourceHandler
	var errs *multierror.Error
	for host, oAuthToken := range o.Credentials {
		instance := host
		if !strings.HasPrefix(instance, "https://") && !strings.HasPrefix(instance, "http://") {
			instance = "https://" + instance
		}
		u, err := url.Parse(instance)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("couldn't parse url: %s", instance))
			continue
		}
		cachePath := filepath.Join(o.CacheHomeDir, "diskv", host)
		client, httpClient, err := buildClient(ctx, oAuthToken, instance, cachePath)
		if err != nil {
			errs = multierror.Append(errs, err)
		}
		rh := newResourceHandler(u.Host, client, httpClient, o.ResourceMappings, options)
		rhs = append(rhs, rh)
	}

	return rhs, errs.ErrorOrNil()
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

func newResourceHandler(host string, client *github.Client, httpClient *http.Client, localMappings map[string]string, options api.ParsingOptions) resourcehandlers.ResourceHandler {
	rawHost := "raw." + host
	if host == "github.com" {
		rawHost = "raw.githubusercontent.com"
	}
	return pg.NewPG(client, httpClient, &osshim.OsShim{}, []string{host, rawHost}, localMappings, options)
}

// manifest reads the resource at uri, resolves it as template applying vars,
// and finally parses it into api.Documentation model
func constructInitialManifest(ctx context.Context, uri string, resourceHandlers []resourcehandlers.ResourceHandler, options api.ParsingOptions) (*api.Documentation, error) {
	var (
		handler         resourcehandlers.ResourceHandler
		manifestContent []byte
	)
	uri = strings.TrimSpace(uri)
	registry := resourcehandlers.NewRegistry(resourceHandlers...)

	//check if uri is in file system
	fileInfo, err := os.Stat(uri)
	if err == nil {
		//uri is from file system

		if fileInfo.IsDir() {
			return nil, fmt.Errorf("top level manifest %s is a directory", uri)
		}

		if manifestContent, err = ioutil.ReadFile(uri); err != nil {
			return nil, err
		}

		doc, err := api.Parse(manifestContent, options)
		if err != nil {
			return nil, fmt.Errorf("failed to parse manifest: %s. %+v", uri, err)
		}

		return doc, nil
	}

	if handler = registry.Get(uri); handler == nil {
		return nil, fmt.Errorf("no suitable reader found for %s. Is this path correct?", uri)
	}
	return handler.ResolveDocumentation(ctx, uri)
}

// NewReactor creates a Reactor from Options
func newReactor(options reactor.Options, hugo reactor.Hugo, rhs []resourcehandlers.ResourceHandler) (*reactor.Reactor, error) {

	config := reactor.Config{
		Options:          options,
		ResourceHandlers: rhs,
		Hugo:             hugo,
	}

	if config.DryRun {
		config.DryRunWriter = writers.NewDryRunWritersFactory(os.Stdout)
		config.Writer = config.DryRunWriter.GetWriter(config.DestinationPath)
		config.ResourceDownloadWriter = config.DryRunWriter.GetWriter(filepath.Join(config.DestinationPath, config.ResourcesPath))
	} else {
		config.Writer = &writers.FSWriter{
			Root: config.DestinationPath,
			Hugo: config.Hugo.Enabled,
		}
		config.ResourceDownloadWriter = &writers.FSWriter{
			Root: filepath.Join(config.DestinationPath, config.ResourcesPath),
		}
	}

	if len(config.GhInfoDestination) > 0 {
		config.GitInfoWriter = &writers.FSWriter{
			Root: filepath.Join(config.DestinationPath, config.GhInfoDestination),
			Ext:  "json",
		}
	}

	return reactor.NewReactor(config)
}
