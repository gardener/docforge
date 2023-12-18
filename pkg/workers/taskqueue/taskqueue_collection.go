// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package taskqueue

import (
	"context"
	"sync"

	"github.com/hashicorp/go-multierror"
	"k8s.io/klog/v2"
)

// QueueControllerCollection holds multiple queue controllers
type QueueControllerCollection struct {
	waitGroup *sync.WaitGroup
	queues    []QueueController
}

// NewQueueControllerCollection creates a new QueueControllerCollection
func NewQueueControllerCollection(waitGroup *sync.WaitGroup, queues ...QueueController) *QueueControllerCollection {
	return &QueueControllerCollection{waitGroup, queues}
}

// Add adds a controller
func (q *QueueControllerCollection) Add(queue QueueController) {
	q.queues = append(q.queues, queue)
}

// Start starts all queues
func (q *QueueControllerCollection) Start(ctx context.Context) {
	for _, queue := range q.queues {
		queue.Start(ctx)
	}
}

// Stop stops all queues
func (q *QueueControllerCollection) Stop() {
	for _, queue := range q.queues {
		queue.Stop()
	}
}

// Wait waits for all queues to finish
func (q *QueueControllerCollection) Wait() {
	q.waitGroup.Wait()
}

// GetErrorList returns error list
func (q *QueueControllerCollection) GetErrorList() *multierror.Error {
	var errors *multierror.Error
	for _, queue := range q.queues {
		errors = multierror.Append(errors, queue.GetErrorList())
	}
	return errors
}

// LogTaskProcessed logs task processed
func (q *QueueControllerCollection) LogTaskProcessed() {
	for _, queue := range q.queues {
		klog.Infof("%s tasks processed: %d\n", queue.Name(), queue.GetProcessedTasksCount())
	}
}
