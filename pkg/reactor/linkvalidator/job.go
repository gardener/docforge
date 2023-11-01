// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package linkvalidator

import (
	"context"
	"fmt"
	"net/url"
	"sync"

	"github.com/gardener/docforge/pkg/document"
	"github.com/gardener/docforge/pkg/httpclient"
	"github.com/gardener/docforge/pkg/reactor/jobs"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"k8s.io/klog/v2"
)

type validator struct {
	*ValidatorWorker
	queue *jobs.JobQueue
}

// New creates new Validator
func New(workerCount int, failFast bool, wg *sync.WaitGroup, client httpclient.Client, registry resourcehandlers.Registry) (document.Validator, jobs.QueueController, error) {
	vWorker, err := NewValidatorWorker(client, registry)
	if err != nil {
		return nil, nil, err
	}
	queue, err := jobs.NewJobQueue("Validator", workerCount, vWorker.execute, failFast, wg)
	if err != nil {
		return nil, nil, err
	}
	v := &validator{
		vWorker,
		queue,
	}
	return v, queue, nil
}

func (v *validator) ValidateLink(linkURL *url.URL, linkDestination, contentSourcePath string) bool {
	vTask := &validationTask{
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
type validationTask struct {
	LinkURL           *url.URL
	LinkDestination   string
	ContentSourcePath string
}

// Validate checks if validationTask.LinkUrl is available and if it cannot be reached, a warning is logged
func (v *ValidatorWorker) execute(ctx context.Context, task interface{}) error {
	vTask, ok := task.(*validationTask)
	if !ok {
		return fmt.Errorf("incorrect validation task: %T", task)
	}
	return v.Validate(ctx, vTask.LinkURL, vTask.LinkDestination, vTask.ContentSourcePath)
}
