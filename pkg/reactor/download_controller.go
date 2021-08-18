// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reactor

import (
	"context"
	"fmt"
	"sync"

	"github.com/gardener/docforge/pkg/jobs"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/writers"
	"k8s.io/klog/v2"
)

// DownloadTask holds information for source and target of linked document resources
type DownloadTask struct {
	Source    string
	Target    string
	Referer   string
	Reference string
}

type downloadWorker interface {
	download(ctx context.Context, dt *DownloadTask) error
}

// DownloadController encapsulates activities for asynchronous
// and parallel scheduling and download of resources
type DownloadController interface {
	jobs.Controller
	// Schedule is a typesafe wrapper around Controller#Enqueue
	// for enqueuing download tasks. An error is returned if
	// scheduling fails.
	Schedule(ctx context.Context, task *DownloadTask) error
}

// downloadController implements reactor#DownloadController
type downloadController struct {
	jobs.Controller
	downloadWorker
	rwlock              sync.RWMutex
	downloadedResources map[string][]*DownloadTask
	job                 *jobs.Job
	jobs.Worker
}

type _downloadWorker struct {
	writers.Writer
	Reader
}

// NewDownloadController creates DownloadController object
func NewDownloadController(reader Reader, writer writers.Writer, workersCount int, failFast bool, rhs resourcehandlers.Registry) DownloadController {
	if reader == nil {
		reader = &GenericReader{
			ResourceHandlers: rhs,
		}
	}
	if writer == nil {
		panic(fmt.Sprint("Invalid argument: writer is nil"))
		//writer = &writers.FSWriter{}
	}

	d := &_downloadWorker{
		Reader: reader,
		Writer: writer,
	}

	job := &jobs.Job{
		ID:         "Download",
		FailFast:   failFast,
		MaxWorkers: workersCount,
		MinWorkers: workersCount,
		Queue:      jobs.NewWorkQueue(100),
	}
	job.SetIsWorkerExitsOnEmptyQueue(true)

	controller := &downloadController{
		Controller:          jobs.NewController(job),
		downloadWorker:      d,
		downloadedResources: make(map[string][]*DownloadTask),
		job:                 job,
	}
	controller.job.Worker = withDownloadController(d, controller)
	return controller
}

func (d *_downloadWorker) download(ctx context.Context, dt *DownloadTask) error {
	klog.V(6).Infof("Downloading %s as %s\n", dt.Source, dt.Target)
	blob, err := d.Reader.Read(ctx, dt.Source)
	if err != nil {
		if _, ok := err.(resourcehandlers.ErrResourceNotFound); ok {
			return err
		}
		return fmt.Errorf("downloading %s as %s failed: %w", dt.Source, dt.Target, err)
	}

	if err := d.Writer.Write(dt.Target, "", blob, nil); err != nil {
		return err
	}
	return nil
}

func withDownloadController(downloadWorker *_downloadWorker, ctrl *downloadController) jobs.WorkerFunc {
	return func(ctx context.Context, task interface{}, wq jobs.WorkQueue) *jobs.WorkerError {
		return downloadWorker.Work(ctx, ctrl, task, wq)
	}
}

func (d *_downloadWorker) Work(ctx context.Context, ctrl *downloadController, task interface{}, wq jobs.WorkQueue) *jobs.WorkerError {
	if task, ok := task.(*DownloadTask); ok {
		if !ctrl.isDownloaded(task) {
			if err := d.download(ctx, task); err != nil {
				refs := fmt.Sprintf("Reference %s from referer %s", task.Reference, task.Referer)
				klog.Warningf("%s : %s\n", refs, err.Error())
				if _, ok = err.(resourcehandlers.ErrResourceNotFound); ok {
					return nil
				}
				err = fmt.Errorf("%s : %w", refs, err)
				return jobs.NewWorkerError(err, 0)
			}
		}
		ctrl.setDownloaded(task)
	}
	return nil
}

func (c *downloadController) isDownloaded(dt *DownloadTask) bool {
	c.rwlock.Lock()
	defer c.rwlock.Unlock()
	_, ok := c.downloadedResources[dt.Source]
	return ok
}

func (c *downloadController) setDownloaded(dt *DownloadTask) {
	c.rwlock.Lock()
	defer c.rwlock.Unlock()
	tasks := c.downloadedResources[dt.Source]
	if len(tasks) == 0 {
		tasks = []*DownloadTask{dt}
	} else {
		tasks = append(tasks, dt)
	}
	c.downloadedResources[dt.Source] = tasks
}

// Schedule enqueues and resource link for download
func (c *downloadController) Schedule(ctx context.Context, downloadTask *DownloadTask) error {
	klog.V(6).Infof("[%s] Linked resource %s scheduled for download as %s\n", downloadTask.Referer, downloadTask.Reference, downloadTask.Target)
	if !c.Enqueue(ctx, downloadTask) {
		return fmt.Errorf("scheduling download of %s referenced by %s failed", downloadTask.Reference, downloadTask.Referer)
	}
	return nil
}

func (c *downloadController) Stop(shutdownCh chan struct{}) {
	// Check and exit immediately if nothing in queue and blocked waiting
	if c.job.Queue.Count() == 0 {
		c.Controller.Shutdown()
	}
	c.Controller.Stop(shutdownCh)
}
