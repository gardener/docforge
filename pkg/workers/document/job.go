// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"context"
	"fmt"
	"sync"

	"github.com/gardener/docforge/cmd/hugo"
	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/workers/linkresolver"
	"github.com/gardener/docforge/pkg/workers/linkvalidator"
	"github.com/gardener/docforge/pkg/workers/taskqueue"
	"github.com/gardener/docforge/pkg/writers"
	"k8s.io/klog/v2"
)

// Worker defines a structure for processing manifest.Node document content
type documentScheduler struct {
	*Worker
	queue taskqueue.Interface
}

// Processor represents document processor
type Processor interface {
	ProcessNode(node *manifest.Node) bool
}

// New creates a new Worker
func New(workerCount int, failFast bool, wg *sync.WaitGroup, structure []*manifest.Node, validator linkvalidator.Interface, rhs registry.Interface, hugo hugo.Hugo, writer writers.Writer, skipLinkValidation bool) (Processor, taskqueue.QueueController, error) {
	lr := linkresolver.New(structure, rhs, hugo)
	worker := NewDocumentWorker(validator, lr, rhs, hugo, writer, skipLinkValidation)
	queue, err := taskqueue.New("Document", workerCount, worker.execute, failFast, wg)
	if err != nil {
		return nil, nil, err
	}
	ds := &documentScheduler{
		worker,
		queue,
	}
	return ds, queue, nil
}

func (ds *documentScheduler) ProcessNode(node *manifest.Node) bool {
	added := ds.queue.AddTask(node)
	if !added {
		klog.Warningf("scheduling document write failed for node %v\n", node)
	}
	return added
}

func (w *Worker) execute(ctx context.Context, task interface{}) error {
	node, ok := task.(*manifest.Node)
	if !ok {
		return fmt.Errorf("incorrect document work task: %T", task)
	}
	return w.ProcessNode(ctx, node)
}
