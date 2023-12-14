// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package downloader

import (
	"context"
	"fmt"
	"sync"

	"github.com/gardener/docforge/pkg/document"
	"github.com/gardener/docforge/pkg/reactor/jobs"
	"github.com/gardener/docforge/pkg/readers"
	"github.com/gardener/docforge/pkg/writers"
	"k8s.io/klog/v2"
)

type downloadScheduler struct {
	*DownloadWorker
	queue *jobs.JobQueue
}

// NewDownloadScheduler create a DownloadScheduler to schedule download resources
func New(workerCount int, failFast bool, wg *sync.WaitGroup, reader readers.Reader, writer writers.Writer) (document.DownloadScheduler, jobs.QueueController, error) {
	dWorker, err := NewDownloader(reader, writer)
	if err != nil {
		return nil, nil, err
	}
	queue, err := jobs.NewJobQueue("Download", workerCount, dWorker.ececute, failFast, wg)
	if err != nil {
		return nil, nil, err
	}
	downloader := &downloadScheduler{
		dWorker,
		queue,
	}
	return downloader, queue, nil
}

// Schedule enqueues and resource link for download
func (ds *downloadScheduler) Schedule(Source string, Target string, Referer string, Reference string) error {
	task := &downloadTask{Source, Target, Referer, Reference}
	klog.V(6).Infof("[%s] linked resource %s scheduled for download as %s\n", task.Referer, task.Reference, task.Target)
	if !ds.queue.AddTask(task) {
		return fmt.Errorf("scheduling download of %s referenced by %s failed", task.Reference, task.Referer)
	}
	return nil
}

func (d *DownloadWorker) ececute(ctx context.Context, task interface{}) error {
	dt, ok := task.(*downloadTask)
	if !ok {
		return fmt.Errorf("incorrect download task: %T", task)
	}
	return d.Download(ctx, dt.Source, dt.Target, dt.Referer, dt.Reference)
}

// DownloadTask holds information for source and target of linked document resources
type downloadTask struct {
	Source    string
	Target    string
	Referer   string
	Reference string
}
