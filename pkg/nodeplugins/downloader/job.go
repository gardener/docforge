// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package downloader

import (
	"context"
	"fmt"
	"sync"

	"github.com/gardener/docforge/pkg/nodeplugins/markdown/document/markdown"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/workers/taskqueue"
	"github.com/gardener/docforge/pkg/writers"
	"k8s.io/klog/v2"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate -header ../../../license_prefix.txt

// Interface encapsulates activities for asynchronous and parallel scheduling and download of resources
//
//counterfeiter:generate . Interface
type Interface interface {
	// Schedule is a typesafe wrapper for enqueuing download tasks. An error is returned if scheduling fails.
	Schedule(source string, destinationPath string, trusted bool) error
}

type downloadScheduler struct {
	*ResourceDownloadWorker
	queue taskqueue.Interface
}

// New create a DownloadScheduler to schedule download resources
func New(workerCount int, failFast bool, wg *sync.WaitGroup, registry registry.Interface, writer writers.Writer, failOnStripped bool) (Interface, taskqueue.QueueController, error) {
	dWorker, err := NewDownloader(registry, writer)
	if err != nil {
		return nil, nil, err
	}
	dWorker.failOnStripped = failOnStripped
	queue, err := taskqueue.New("Download", workerCount, dWorker.execute, failFast, wg)
	if err != nil {
		return nil, nil, err
	}
	downloader := &downloadScheduler{
		dWorker,
		queue,
	}
	return downloader, queue, nil
}

// Schedule enqueues a resource link for download
func (ds *downloadScheduler) Schedule(source string, destinationPath string, trusted bool) error {
	task := &downloadTask{source, destinationPath, trusted}
	if !ds.queue.AddTask(task) {
		return fmt.Errorf("scheduling download of %s failed", task.source)
	}
	return nil
}

func (d *ResourceDownloadWorker) execute(ctx context.Context, task interface{}) error {
	dt, ok := task.(*downloadTask)
	if !ok {
		return fmt.Errorf("incorrect download task: %T", task)
	}
	if !dt.trusted {
		var pol markdown.DefaultPolicy
		switch pol.NonMarkdownDecision(dt.source) {
		case markdown.Refuse:
			if d.failOnStripped {
				return fmt.Errorf("content policy: refusing download of %s (blocked file type)", dt.source)
			}
			klog.Warningf("content policy: refusing download of %s (blocked file type)", dt.source)
			return nil
		case markdown.WarnNonMarkdown:
			klog.Warningf("content policy: downloading non-markdown source %s (include with warning)", dt.source)
		}
	}
	return d.Download(ctx, dt.source, dt.destinationPath)
}

// downloadTask holds information for source and destinationPath of linked document resources
type downloadTask struct {
	source          string
	destinationPath string
	trusted         bool
}
