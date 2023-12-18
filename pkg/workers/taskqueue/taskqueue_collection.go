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

type QueueControllerCollection struct {
	waitGroup *sync.WaitGroup
	queues    []QueueController
}

func NewQueueControllerCollection(waitGroup *sync.WaitGroup, queues ...QueueController) *QueueControllerCollection {
	return &QueueControllerCollection{waitGroup, queues}
}

func (q *QueueControllerCollection) Add(queue QueueController) {
	q.queues = append(q.queues, queue)
}

func (q *QueueControllerCollection) Start(ctx context.Context) {
	for _, queue := range q.queues {
		queue.Start(ctx)
	}
}

func (q *QueueControllerCollection) Stop() {
	for _, queue := range q.queues {
		queue.Stop()
	}
}

func (q *QueueControllerCollection) Wait() {
	q.waitGroup.Wait()
}

func (q *QueueControllerCollection) GetErrorList() *multierror.Error {
	var errors *multierror.Error
	for _, queue := range q.queues {
		errors = multierror.Append(errors, queue.GetErrorList())
	}
	return errors
}

func (q *QueueControllerCollection) LogTaskProcessed() {
	for _, queue := range q.queues {
		klog.Infof("%s tasks processed: %d\n", queue.Name(), queue.GetProcessedTasksCount())
	}
}
