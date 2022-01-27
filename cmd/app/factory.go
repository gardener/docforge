// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"github.com/gardener/docforge/cmd/configuration"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"k8s.io/klog/v2"

	"github.com/gardener/docforge/pkg/reactor"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/resourcehandlers/git"
	ghrs "github.com/gardener/docforge/pkg/resourcehandlers/github"
	"github.com/gardener/docforge/pkg/writers"
	"github.com/hashicorp/go-multierror"

	"github.com/google/go-github/v32/github"
	"golang.org/x/oauth2"
	"golang.org/x/time/rate"
)

const (
	headerRateLimit     = "X-RateLimit-Limit"
	headerRateRemaining = "X-RateLimit-Remaining"
	headerRateReset     = "X-RateLimit-Reset"
)

// Options is the set of parameters for creating reactor objects
type Options struct {
	Credentials     []*configuration.Credentials
	Hugo            *configuration.Hugo
	HomeDir         string
	LocalMappings   map[string]string
	DefaultBranches map[string]string
	LastNVersions   map[string]int
}

// NewReactor creates a Reactor from Options
func NewReactor(f *cmdFlags, o *Options, rhs []resourcehandlers.ResourceHandler) (*reactor.Reactor, error) {
	opt := &reactor.Options{
		DocumentWorkersCount:         f.documentWorkersCount,
		ValidationWorkersCount:       f.validationWorkersCount,
		FailFast:                     f.failFast,
		DestinationPath:              f.destinationPath,
		ResourcesPath:                f.resourcesPath,
		ResourceDownloadWorkersCount: f.resourceDownloadWorkersCount,
		RewriteEmbedded:              f.rewriteEmbedded,
		ResourceHandlers:             rhs,
		Resolve:                      f.resolve,
		ManifestPath:                 f.documentationManifestPath,
		Hugo:                         o.Hugo,
		DefaultBranches:              o.DefaultBranches,
		LastNVersions:                o.LastNVersions,
	}

	if f.dryRun {
		opt.DryRunWriter = writers.NewDryRunWritersFactory(os.Stdout)
		opt.Writer = opt.DryRunWriter.GetWriter(f.destinationPath)
		opt.ResourceDownloadWriter = opt.DryRunWriter.GetWriter(filepath.Join(f.destinationPath, f.resourcesPath))
	} else {
		opt.Writer = &writers.FSWriter{
			Root: f.destinationPath,
			Hugo: opt.Hugo.Enabled,
		}
		opt.ResourceDownloadWriter = &writers.FSWriter{
			Root: filepath.Join(f.destinationPath, f.resourcesPath),
		}
	}

	if len(f.ghInfoDestination) > 0 {
		opt.GitInfoWriter = &writers.FSWriter{
			Root: filepath.Join(f.destinationPath, f.ghInfoDestination),
			Ext:  "json",
		}
	}

	return reactor.NewReactor(opt)
}

func initResourceHandlers(ctx context.Context, f *cmdFlags, o *Options) ([]resourcehandlers.ResourceHandler, error) {
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

		client, httpClient, err := buildClient(ctx, *cred.OAuthToken, f.ghThrottling, instance)
		if err != nil {
			errs = multierror.Append(errs, err)
		}
		rh := newResourceHandler(u.Host, o.HomeDir, cred.Username, *cred.OAuthToken, client, httpClient, f.useGit, o.LocalMappings)
		rhs = append(rhs, rh)
	}

	return rhs, errs.ErrorOrNil()
}

func newResourceHandler(host, homeDir string, user *string, token string, client *github.Client, httpClient *http.Client, useGit bool, localMappings map[string]string) resourcehandlers.ResourceHandler {
	rawHost := "raw." + host
	if host == "github.com" {
		rawHost = "raw.githubusercontent.com"
	}

	if useGit {
		return git.NewResourceHandler(filepath.Join(homeDir, git.CacheDir), user, token, client, httpClient, []string{host, rawHost}, localMappings)
	}
	return ghrs.NewResourceHandler(client, httpClient, []string{host, rawHost})
}

func buildClient(ctx context.Context, accessToken string, withClientThrottling bool, host string) (*github.Client, *http.Client, error) {
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
			return nil, nil, fmt.Errorf("cannot create rate-limited client for GitHub instance %s: %w", host, err)
		}
		if rl == nil {
			return nil, nil, fmt.Errorf("cannot create rate-limited client for GitHub instance %s: rate limit exceeded", host)
		}
		// Wrap client transport instrumenting it for rate-limited requests
		oauthClient.Transport = WithClientRateLimit(oauthClient.Transport, rl)
	}
	// Wrap client transport instrumenting it for request/response logging
	oauthClient.Transport = WithClientHTTPLogging(oauthClient.Transport)

	var client *github.Client
	if host == "https://github.com" {
		if accessToken != "" {
			client = github.NewClient(oauthClient)
		} else {
			client = github.NewClient(nil)
		}
		return client, oauthClient, nil
	}
	client, err = github.NewEnterpriseClient(host, "", oauthClient)
	return client, oauthClient, err
}

// RoundTripperFunc defines implementation of http.RoundTripper interface
type RoundTripperFunc func(req *http.Request) (*http.Response, error)

// RoundTrip implements the RoundTripper interface.
func (rt RoundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return rt(r)
}

// WithClientHTTPLogging returns http.RoundTripper with logging
func WithClientHTTPLogging(next http.RoundTripper) RoundTripperFunc {
	return func(r *http.Request) (*http.Response, error) {
		var respStatus string
		resp, err := next.RoundTrip(r)
		requestLog := fmt.Sprintf("HTTP %s %s", r.Method, r.URL)
		if err == nil {
			respStatus = resp.Status
		}
		klog.V(6).Infof("%s %s", requestLog, respStatus)
		return resp, err
	}
}

// WithClientRateLimit returns http.RoundTripper with rate limit
func WithClientRateLimit(next http.RoundTripper, ratelimiter *rate.Limiter) RoundTripperFunc {
	ctx := context.Background()
	return func(r *http.Request) (*http.Response, error) {
		err := ratelimiter.Wait(ctx)
		if err != nil {
			return nil, err
		}
		resp, err := next.RoundTrip(r)
		return resp, err
	}
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
	var rt github.Rate
	if limit := r.Header.Get(headerRateLimit); limit != "" {
		rt.Limit, _ = strconv.Atoi(limit)
	}
	if remaining := r.Header.Get(headerRateRemaining); remaining != "" {
		rt.Remaining, _ = strconv.Atoi(remaining)
	}
	if reset := r.Header.Get(headerRateReset); reset != "" {
		if v, _ := strconv.ParseInt(reset, 10, 64); v != 0 {
			rt.Reset = github.Timestamp{
				Time: time.Unix(v, 0),
			}
		}
	}
	return rt
}
