// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package linkvalidator

import (
	"context"
	"fmt"
	"sync"

	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/workers/taskqueue"
	"k8s.io/klog/v2"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate -header ../../../../license_prefix.txt

// Interface validates the links URLs
//
//counterfeiter:generate . Interface
type Interface interface {
	// ValidateLink checks if the link URL is available in a separate goroutine
	// returns true if the task was added for processing, false if it was skipped
	ValidateLink(linkDestination, contentSourcePath string) bool
}

type validator struct {
	*ValidatorWorker
	queue taskqueue.Interface
}

// New creates new Validator
func New(workerCount int, failFast bool, wg *sync.WaitGroup, registry registry.Interface, hostsToReport []string) (Interface, taskqueue.QueueController, error) {
	vWorker, err := NewValidatorWorker(registry, hostsToReport)
	if err != nil {
		return nil, nil, err
	}
	queue, err := taskqueue.New("Validator", workerCount, vWorker.execute, failFast, wg)
	if err != nil {
		return nil, nil, err
	}
	v := &validator{
		vWorker,
		queue,
	}
	return v, queue, nil
}

func (v *validator) ValidateLink(linkDestination, contentSourcePath string) bool {
	vTask := &validationTask{
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
	LinkDestination   string
	ContentSourcePath string
}

// Validate checks if validationTask.LinkUrl is available and if it cannot be reached, a warning is logged
func (v *ValidatorWorker) execute(ctx context.Context, task interface{}) error {
	vTask, ok := task.(*validationTask)
	if !ok {
		return fmt.Errorf("incorrect validation task: %T", task)
	}
	return v.Validate(ctx, vTask.LinkDestination, vTask.ContentSourcePath)
}
