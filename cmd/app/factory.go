// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"k8s.io/klog/v2"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/hugo"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/writers"
	"github.com/hashicorp/go-multierror"

	"github.com/gardener/docforge/pkg/processors"
	"github.com/gardener/docforge/pkg/reactor"
	"github.com/gardener/docforge/pkg/resourcehandlers/fs"
	"github.com/gardener/docforge/pkg/resourcehandlers/git"
	ghrs "github.com/gardener/docforge/pkg/resourcehandlers/github"

	"github.com/google/go-github/v32/github"
	"golang.org/x/oauth2"
	"golang.org/x/time/rate"
)

const (
	headerRateLimit     = "X-RateLimit-Limit"
	headerRateRemaining = "X-RateLimit-Remaining"
	headerRateReset     = "X-RateLimit-Reset"
)

// Options is the set of parameters for creating
// reactor objects
type Options struct {
	MaxWorkersCount              int
	MinWorkersCount              int
	FailFast                     bool
	DestinationPath              string
	ResourcesPath                string
	ManifestAbsPath              string
	ResourceDownloadWorkersCount int
	RewriteEmbedded              bool
	Credentials                  []*Credentials
	GitHubClientThrottling       bool
	Metering                     *Metering
	GitHubInfoPath               string
	DryRunWriter                 io.Writer
	Resolve                      bool
	Hugo                         *hugo.Options
	UseGit                       bool
	HomeDir                      string
	LocalMappings                map[string]string
}

type Credentials struct {
	Host       string
	Username   *string
	OAuthToken string
}

// Metering encapsulates options for setting up client-side
// metering
type Metering struct {
	Enabled bool
}

// NewReactor creates a Reactor from Options
func NewReactor(ctx context.Context, options *Options, globalLinksCfg *api.Links) (*reactor.Reactor, error) {
	dryRunWriters := writers.NewDryRunWritersFactory(options.DryRunWriter)

	rhs, err := initResourceHandlers(ctx, options)
	if err != nil {
		return nil, err
	}

	o := &reactor.Options{
		MaxWorkersCount:              options.MaxWorkersCount,
		MinWorkersCount:              options.MinWorkersCount,
		FailFast:                     options.FailFast,
		DestinationPath:              options.DestinationPath,
		ResourcesPath:                options.ResourcesPath,
		ResourceDownloadWorkersCount: options.ResourceDownloadWorkersCount,
		RewriteEmbedded:              options.RewriteEmbedded,
		Processor:                    nil,
		ResourceHandlers:             rhs,
		DryRunWriter:                 dryRunWriters,
		Resolve:                      options.Resolve,
		GlobalLinksConfig:            globalLinksCfg,
		ManifestAbsPath:              options.ManifestAbsPath,
	}

	if options.DryRunWriter != nil {
		o.Writer = dryRunWriters.GetWriter(options.DestinationPath)
		o.ResourceDownloadWriter = dryRunWriters.GetWriter(filepath.Join(options.DestinationPath, options.ResourcesPath))
	} else {
		o.Writer = &writers.FSWriter{
			Root: options.DestinationPath,
		}
		o.ResourceDownloadWriter = &writers.FSWriter{
			Root: filepath.Join(options.DestinationPath, options.ResourcesPath),
		}
	}

	if len(options.GitHubInfoPath) > 0 {
		o.GitInfoWriter = &writers.FSWriter{
			Root: filepath.Join(options.DestinationPath, options.GitHubInfoPath),
			Ext:  "json",
		}
	}

	if options.Hugo != nil {
		WithHugo(o, options)
	}

	return reactor.NewReactor(o), nil
}

// WithHugo adapts the reactor.Options object with Hugo-specific
// settings for writer and processor
func WithHugo(reactorOptions *reactor.Options, o *Options) {
	hugoOptions := o.Hugo
	reactorOptions.Processor = &processors.ProcessorChain{
		Processors: []processors.Processor{
			&processors.FrontMatter{
				IndexFileNames: hugoOptions.IndexFileNames,
			},
			hugo.NewProcessor(hugoOptions),
		},
	}
	if o.DryRunWriter != nil {
		hugoOptions.Writer = reactorOptions.Writer
	} else {
		hugoOptions.Writer = &writers.FSWriter{
			Root: filepath.Join(o.DestinationPath),
		}
	}
	reactorOptions.Writer = hugo.NewWriter(hugoOptions)
}

func initResourceHandlers(ctx context.Context, o *Options) ([]resourcehandlers.ResourceHandler, error) {
	rhs := []resourcehandlers.ResourceHandler{
		fs.NewFSResourceHandler(),
	}
	var errs *multierror.Error
	for _, creds := range o.Credentials {
		instance := creds.Host
		if !strings.HasPrefix(instance, "https://") && !strings.HasPrefix(instance, "http://") {
			instance = "https://" + instance
		}

		u, err := url.Parse(instance)
		if err != nil {
			multierror.Append(errs, fmt.Errorf("couldn't parse url: %s", instance))
			continue
		}

		client, err := buildClient(ctx, creds.OAuthToken, o.GitHubClientThrottling, instance)
		if err != nil {
			multierror.Append(errs, err)
		}
		rh := newResouceHandler(u.Host, o.HomeDir, creds.Username, creds.OAuthToken, client, o.UseGit, o.LocalMappings)
		rhs = append(rhs, rh)
	}

	return rhs, errs.ErrorOrNil()
}

func newResouceHandler(host, homeDir string, user *string, token string, client *github.Client, useGit bool, localMappings map[string]string) resourcehandlers.ResourceHandler {
	rawHost := "raw." + host
	if host == "github.com" {
		rawHost = "raw.githubusercontent.com"
	}

	if useGit {
		return git.NewResourceHandler(filepath.Join(homeDir, git.CacheDir), user, token, client, []string{host, rawHost}, localMappings)
	}
	return ghrs.NewResourceHandler(client, []string{host, rawHost})
}

func buildClient(ctx context.Context, accessToken string, withClientThrottling bool, host string) (*github.Client, error) {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: accessToken})
	oauthClient := oauth2.NewClient(ctx, ts)
	var (
		err     error
		rl      *rate.Limiter
		apiHost string
	)

	if withClientThrottling {
		if host == "https://github.com" {
			apiHost = "https://api.github.com"
		} else {
			apiHost = fmt.Sprintf("%s/%s", host, "api")
		}
		if rl, err = rateLimitForClient(oauthClient, apiHost); err != nil {
			return nil, fmt.Errorf("cannot create rate-limited client for GitHub instance %s: %w", host, err)
		}
		if rl == nil {
			return nil, fmt.Errorf("cannot create rate-limited client for GitHub instance %s: rate limit exceeded", host)
		}
		// Wrap client transport instrumenting it for rate-limited requests
		oauthClient.Transport = WithClientRateLimit(oauthClient.Transport, rl)
	}
	// Wrap client transport instrumenting it for request/response logging
	oauthClient.Transport = WithClientHTTPLogging(oauthClient.Transport)

	if host == "https://github.com" {
		client := github.NewClient(oauthClient)
		return client, nil
	}

	return github.NewEnterpriseClient(host, "", oauthClient)
}

type RoundTripperFunc func(req *http.Request) (*http.Response, error)

// RoundTrip implements the RoundTripper interface.
func (rt RoundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return rt(r)
}

func WithClientHTTPLogging(next http.RoundTripper) RoundTripperFunc {
	return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
		var respStatus string
		resp, err := next.RoundTrip(r)
		requestLog := fmt.Sprintf("HTTP %s %s", r.Method, r.URL)
		if err == nil {
			respStatus = resp.Status
		}
		klog.V(6).Infof("%s %s", requestLog, respStatus)
		return resp, err
	})
}

func WithClientRateLimit(next http.RoundTripper, ratelimiter *rate.Limiter) RoundTripperFunc {
	ctx := context.Background()
	return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
		ratelimiter.Wait(ctx)
		resp, err := next.RoundTrip(r)
		return resp, err
	})
}

func rateLimitForClient(client *http.Client, host string) (*rate.Limiter, error) {
	var (
		req *http.Request
		err error
	)
	rateEndpointURL := fmt.Sprintf("%s/%s", host, "rate_limit")
	if req, err = http.NewRequest("GET", rateEndpointURL, nil); err != nil {
		return nil, err
	}
	res, err := client.Do(req)
	if res == nil {
		return nil, err
	}
	r := parseRate(res)
	rT1 := time.Now()
	rD := r.Reset.Sub(rT1)
	klog.V(6).Infof("client rate limiting reset in %f seconds", rD.Seconds())
	klog.V(6).Infof("client rate limiting remaining requests %d", r.Remaining)
	if r.Remaining > 0 {
		reqRate := float64(r.Remaining) / rD.Seconds()
		klog.V(6).Infof("client rate limiting requests interval for %s: %v", host, time.Duration(reqRate*float64(time.Second)).Truncate(time.Second))
		rl := rate.NewLimiter(rate.Limit(reqRate), 1)
		return rl, nil
	}
	return nil, nil
}

// parseRate parses the rate related headers.
func parseRate(r *http.Response) github.Rate {
	var rate github.Rate
	if limit := r.Header.Get(headerRateLimit); limit != "" {
		rate.Limit, _ = strconv.Atoi(limit)
	}
	if remaining := r.Header.Get(headerRateRemaining); remaining != "" {
		rate.Remaining, _ = strconv.Atoi(remaining)
	}
	if reset := r.Header.Get(headerRateReset); reset != "" {
		if v, _ := strconv.ParseInt(reset, 10, 64); v != 0 {
			rate.Reset = github.Timestamp{
				Time: time.Unix(v, 0),
			}
		}
	}
	return rate
}
