// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package repositoryhosts

import (
	"context"
	"fmt"
	"time"

	"k8s.io/klog/v2"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate -header ../../../license_prefix.txt

// Registry can register and return resource repoHosts for an url
//
//counterfeiter:generate . Registry
type Registry interface {
	Get(uri string) (RepositoryHost, error)
	LogRateLimits(ctx context.Context)
}

type registry struct {
	repoHosts []RepositoryHost
}

// NewRegistry creates Registry object, optionally loading it with
// resourcerepoHosts if provided
func NewRegistry(resourcerepoHosts ...RepositoryHost) Registry {
	return &registry{repoHosts: resourcerepoHosts}
}

// Get returns an appropriate handler for this type of URIs if anyone those registered accepts it (its Accepts method returns true).
func (r *registry) Get(uri string) (RepositoryHost, error) {
	for _, h := range r.repoHosts {
		if h.Accept(uri) {
			return h, nil
		}
	}
	return nil, fmt.Errorf("no sutiable repository host for %s", uri)
}

func (r *registry) LogRateLimits(ctx context.Context) {
	for _, repoHost := range r.repoHosts {
		l, rr, rt, err := repoHost.GetRateLimit(ctx)
		if err != nil {
			klog.Warningf("Error getting RateLimit for %s: %v\n", repoHost.Name(), err)
		} else if l > 0 && rr > 0 {
			klog.Infof("%s RateLimit: %d requests per hour, Remaining: %d, Reset after: %s\n", repoHost.Name(), l, rr, time.Until(rt).Round(time.Second))
		}
	}
}
