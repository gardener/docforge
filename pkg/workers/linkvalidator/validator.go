// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package linkvalidator

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gardener/docforge/pkg/osfakes/httpclient"
	"github.com/gardener/docforge/pkg/registry"
	"k8s.io/klog/v2"
)

// ValidatorWorker holds nessesary objects ti validate URl
type ValidatorWorker struct {
	repository    registry.Interface
	validated     *linkSet
	hostsToReport []string
}

// NewValidatorWorker creates new ValidatorWorker
func NewValidatorWorker(repository registry.Interface, hostsToReport []string) (*ValidatorWorker, error) {
	if repository == nil || reflect.ValueOf(repository).IsNil() {
		return nil, errors.New("invalid argument: repositoryhosts is nil")
	}
	return &ValidatorWorker{
		repository,
		&linkSet{
			set: make(map[string]struct{}),
		},
		hostsToReport,
	}, nil
}

// Validate validates a link
func (v *ValidatorWorker) Validate(ctx context.Context, LinkDestination string, ContentSourcePath string) error {
	var (
		req  *http.Request
		resp *http.Response
	)
	LinkURL, err := url.Parse(strings.TrimSuffix(LinkDestination, "/"))
	if err != nil {
		return fmt.Errorf("error when parsing link in %s : %w", ContentSourcePath, err)
	}
	// ignore sample hosts e.g. localhost
	host := LinkURL.Hostname()
	if host == "localhost" || host == "127.0.0.1" {
		return nil
	}
	if slices.Contains(v.hostsToReport, LinkURL.Host) {
		return fmt.Errorf("%s has link %s with host to report", ContentSourcePath, LinkDestination)
	}
	// unify links destination by excluding query, fragment & user info
	u := &url.URL{
		Scheme: LinkURL.Scheme,
		Host:   LinkURL.Host,
		Path:   LinkURL.Path,
	}
	unifiedURL := u.String()
	if v.validated.exist(unifiedURL) {
		return nil
	}

	absLinkDestination := LinkURL.String()
	client := v.repository.Client(absLinkDestination)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// try HEAD
	if req, err = http.NewRequestWithContext(ctx, http.MethodHead, absLinkDestination, nil); err != nil {
		return fmt.Errorf("failed to prepare HEAD validation request: %v", err)
	}
	if resp, err = doValidation(req, client); err != nil {
		klog.Warningf("failed to validate absolute link for %s from source %s: %v\n", LinkDestination, ContentSourcePath, err)
	} else if resp.StatusCode >= 400 && resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusUnauthorized {
		// on error status code different from authorization errors
		// retry GET
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second) // reset the context for the GET request
		defer cancel()
		if req, err = http.NewRequestWithContext(ctx, http.MethodGet, absLinkDestination, nil); err != nil {
			return fmt.Errorf("failed to prepare GET validation request: %v", err)
		}
		if resp, err = doValidation(req, client); err != nil {
			klog.Warningf("failed to validate absolute link for %s from source %s: %v\n", LinkDestination, ContentSourcePath, err)
		} else if resp.StatusCode >= 400 && resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusUnauthorized {
			klog.Warningf("failed to validate absolute link for %s from source %s: %v\n", LinkDestination, ContentSourcePath, fmt.Errorf("HTTP Status %s", resp.Status))
		}
	}
	v.validated.add(unifiedURL)
	return nil
}

// doValidation performs several attempts to execute http request if http status code is 429
func doValidation(req *http.Request, client httpclient.Client) (*http.Response, error) {
	intervals := []int{1, 5, 10, 20}
	resp, err := client.Do(req)
	if err != nil {
		return resp, err
	}
	defer resp.Body.Close()
	attempts := 0
	for resp.StatusCode == http.StatusTooManyRequests && attempts < len(intervals)-1 {
		klog.Warningf("Retrying request!")
		sleep := intervals[attempts] + rand.Intn(attempts+1)
		// check for Retry-After Header and overwrite sleep time
		if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
			// support only value in seconds <= 5 min
			var after int
			if after, err = strconv.Atoi(retryAfter); err == nil && after <= 5*60 {
				sleep = after
			}
		}
		time.Sleep(time.Duration(sleep) * time.Second)
		resp, err = client.Do(req)
		if err != nil {
			return resp, err
		}
		attempts++
	}
	return resp, err
}

// linkSet holds link destinations that have been successfully validated
// used to avoid redundant checks & HTTP Status 429
type linkSet struct {
	set map[string]struct{}
	mux sync.RWMutex
}

func (l *linkSet) exist(dest string) bool {
	l.mux.RLock()
	defer l.mux.RUnlock()
	_, ok := l.set[dest]
	return ok
}

func (l *linkSet) add(dest string) {
	l.mux.Lock()
	defer l.mux.Unlock()
	l.set[dest] = struct{}{}
}
