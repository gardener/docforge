// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reactor

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gardener/docforge/pkg/jobs"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/util/httpclient"
	"k8s.io/klog/v2"
)

// Validator validates the links URLs
//
//counterfeiter:generate . Validator
type Validator interface {
	// ValidateLink checks if the link URL is available in a separate goroutine
	// returns true if the task was added for processing, false if it was skipped
	ValidateLink(linkURL *url.URL, linkDestination, contentSourcePath string) bool
}

type validator struct {
	queue *jobs.JobQueue
}

// NewValidator creates new Validator
func NewValidator(queue *jobs.JobQueue) Validator {
	return &validator{
		queue: queue,
	}
}

func (v *validator) ValidateLink(linkURL *url.URL, linkDestination, contentSourcePath string) bool {
	vTask := &ValidationTask{
		LinkURL:           linkURL,
		LinkDestination:   linkDestination,
		ContentSourcePath: contentSourcePath,
	}
	added := v.queue.AddTask(vTask)
	if !added {
		klog.Warningf("link validation failed for task %v\n", vTask)
	}
	return added
}

// ValidationTask represents a task for validating LinkURL
type ValidationTask struct {
	LinkURL           *url.URL
	LinkDestination   string
	ContentSourcePath string
}

type validatorWorker struct {
	httpClient       httpclient.Client
	resourceHandlers resourcehandlers.Registry
	validated        *linkSet
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

// Validate checks if validationTask.LinkUrl is available and if it cannot be reached, a warning is logged
func (v *validatorWorker) Validate(ctx context.Context, task interface{}) error {
	if vTask, ok := task.(*ValidationTask); ok {
		// ignore sample hosts e.g. localhost
		host := vTask.LinkURL.Hostname()
		if host == "localhost" || host == "127.0.0.1" || host == "1.2.3.4" || strings.Contains(host, "foo.bar") {
			return nil
		}
		// unify links destination by excluding query, fragment & user info
		u := &url.URL{
			Scheme: vTask.LinkURL.Scheme,
			Host:   vTask.LinkURL.Host,
			Path:   vTask.LinkURL.Path,
		}
		unifiedURL := u.String()
		if v.validated.exist(unifiedURL) {
			return nil
		}
		client := v.httpClient
		// check for handler HTTP Client
		absLinkDestination := vTask.LinkURL.String()
		if handler := v.resourceHandlers.Get(absLinkDestination); handler != nil {
			// get appropriate http client, if any
			if handlerClient := handler.GetClient(); handlerClient != nil {
				client = handlerClient
			}
		}
		var (
			req  *http.Request
			resp *http.Response
			err  error
		)
		// try HEAD
		if req, err = http.NewRequestWithContext(ctx, http.MethodHead, absLinkDestination, nil); err != nil {
			return fmt.Errorf("failed to prepare HEAD validation request: %v", err)
		}
		if resp, err = doValidation(req, client); err != nil {
			klog.Warningf("failed to validate absolute link for %s from source %s: %v\n",
				vTask.LinkDestination, vTask.ContentSourcePath, err)
		} else if resp.StatusCode >= 400 && resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusUnauthorized {
			// on error status code different from authorization errors
			// retry GET
			if req, err = http.NewRequestWithContext(ctx, http.MethodGet, absLinkDestination, nil); err != nil {
				return fmt.Errorf("failed to prepare GET validation request: %v", err)
			}
			if resp, err = doValidation(req, client); err != nil {
				klog.Warningf("failed to validate absolute link for %s from source %s: %v\n",
					vTask.LinkDestination, vTask.ContentSourcePath, err)
			} else if resp.StatusCode >= 400 && resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusUnauthorized {
				klog.Warningf("failed to validate absolute link for %s from source %s: %v\n",
					vTask.LinkDestination, vTask.ContentSourcePath, fmt.Errorf("HTTP Status %s", resp.Status))
			}
		}
		v.validated.add(unifiedURL)
		return nil
	}
	return fmt.Errorf("incorrect validation task: %T", task)
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

// ValidateWorkerFunc returns Validate worker func
func ValidateWorkerFunc(httpClient httpclient.Client, resourceHandlers resourcehandlers.Registry) (jobs.WorkerFunc, error) {
	if httpClient == nil || reflect.ValueOf(httpClient).IsNil() {
		return nil, errors.New("invalid argument: httpClient is nil")
	}
	if resourceHandlers == nil || reflect.ValueOf(resourceHandlers).IsNil() {
		return nil, errors.New("invalid argument: resourceHandlers is nil")
	}
	vWorker := &validatorWorker{
		httpClient:       httpClient,
		resourceHandlers: resourceHandlers,
		validated: &linkSet{
			set: make(map[string]struct{}),
		},
	}
	return vWorker.Validate, nil
}
