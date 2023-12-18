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
	"github.com/gardener/docforge/pkg/readers/repositoryhosts"
	"github.com/gardener/docforge/pkg/workers/downloader"
	"github.com/gardener/docforge/pkg/workers/linkresolver"
	"github.com/gardener/docforge/pkg/workers/linkvalidator"
	"github.com/gardener/docforge/pkg/workers/taskqueue"
	"github.com/gardener/docforge/pkg/writers"
	"k8s.io/klog/v2"
)

// DocumentWorker defines a structure for processing manifest.Node document content
type documentScheduler struct {
	*DocumentWorker
	queue taskqueue.Interface
}

type DocumentProcessor interface {
	ProcessNode(node *manifest.Node) bool
}

// New creates a new DocumentWorker
func New(workerCount int, failFast bool, wg *sync.WaitGroup, structure []*manifest.Node, resourcesRoot string, downloadJob downloader.Interface, validator linkvalidator.Interface, rh repositoryhosts.Registry, hugo hugo.Hugo, writer writers.Writer) (DocumentProcessor, taskqueue.QueueController, error) {
	lr := &linkresolver.LinkResolver{
		Repositoryhosts: rh,
		Hugo:            hugo,
		SourceToNode:    make(map[string][]*manifest.Node),
	}
	for _, node := range structure {
		if node.Source != "" {
			lr.SourceToNode[node.Source] = append(lr.SourceToNode[node.Source], node)
		} else if len(node.MultiSource) > 0 {
			for _, s := range node.MultiSource {
				lr.SourceToNode[s] = append(lr.SourceToNode[s], node)
			}
		}
	}
	worker := NewDocumentWorker(resourcesRoot, downloadJob, validator, lr, rh, hugo, writer)
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

func (w *DocumentWorker) execute(ctx context.Context, task interface{}) error {
	node, ok := task.(*manifest.Node)
	if !ok {
		return fmt.Errorf("incorrect document work task: %T", task)
	}
	return w.ProcessNode(ctx, node)
}
