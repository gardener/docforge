// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reactor

import (
	"context"
	"errors"
	"fmt"
	"github.com/gardener/docforge/pkg/jobs"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/util/httpclient"
	"github.com/gardener/docforge/pkg/util/urls"
	"k8s.io/klog/v2"
	"math/rand"
	"net/http"
	"reflect"
	"strings"
	"time"
)

// Validator validates the links URLs
//counterfeiter:generate . Validator
type Validator interface {
	// ValidateLink checks if the link URL is available in a separate goroutine
	// returns true if the task was added for processing, false if it was skipped
	ValidateLink(linkURL *urls.URL, linkDestination, contentSourcePath string) bool
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

func (v *validator) ValidateLink(linkURL *urls.URL, linkDestination, contentSourcePath string) bool {
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
	LinkURL           *urls.URL
	LinkDestination   string
	ContentSourcePath string
}

type validatorWorker struct {
	httpClient       httpclient.Client
	resourceHandlers resourcehandlers.Registry
}

// Validate checks if validationTask.LinkUrl is available and if it cannot be reached, a warning is logged
func (v *validatorWorker) Validate(ctx context.Context, task interface{}) error {
	if vTask, ok := task.(*ValidationTask); ok {
		// ignore sample hosts e.g. localhost
		host := vTask.LinkURL.Hostname()
		if host == "localhost" || host == "127.0.0.1" || host == "1.2.3.4" || strings.Contains(host, "foo.bar") {
			return nil
		}
		client := v.httpClient
		// check if link absolute destination exists locally
		absLinkDestination := vTask.LinkURL.String()
		handler := v.resourceHandlers.Get(absLinkDestination)
		if handler != nil {
			if _, err := handler.BuildAbsLink(vTask.ContentSourcePath, absLinkDestination); err == nil {
				// no ErrResourceNotFound -> absolute destination exists locally
				return nil
			}
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
		if req, err = http.NewRequestWithContext(ctx, http.MethodHead, vTask.LinkURL.String(), nil); err != nil {
			return fmt.Errorf("failed to prepare HEAD validation request: %v", err)
		}
		if resp, err = doValidation(req, client); err != nil {
			klog.Warningf("failed to validate absolute link for %s from source %s: %v\n",
				vTask.LinkDestination, vTask.ContentSourcePath, err)
		} else if resp.StatusCode >= 400 && resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusUnauthorized {
			// on error status code different from authorization errors
			// retry GET
			if req, err = http.NewRequestWithContext(ctx, http.MethodGet, vTask.LinkURL.String(), nil); err != nil {
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
	} else {
		return fmt.Errorf("incorrect validation task: %T", task)
	}
	return nil
}

// doValidation performs several attempts to execute http request if http status code is 429
func doValidation(req *http.Request, client httpclient.Client) (*http.Response, error) {
	intervals := []int{1, 5, 10, 20}
	resp, err := client.Do(req)
	if err != nil {
		return resp, err
	}
	_ = resp.Body.Close()
	attempts := 0
	for resp.StatusCode == http.StatusTooManyRequests && attempts < len(intervals)-1 {
		time.Sleep(time.Duration(intervals[attempts]+rand.Intn(attempts+1)) * time.Second)
		resp, err = client.Do(req)
		if err != nil {
			return resp, err
		}
		_ = resp.Body.Close()
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
	}
	return vWorker.Validate, nil
}
