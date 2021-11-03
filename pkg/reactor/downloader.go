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
	"github.com/gardener/docforge/pkg/writers"
	"k8s.io/klog/v2"
	"reflect"
	"sync"
)

// DownloadScheduler encapsulates activities for asynchronous and parallel scheduling and download of resources
//counterfeiter:generate . DownloadScheduler
type DownloadScheduler interface {
	// Schedule is a typesafe wrapper for enqueuing download tasks. An error is returned if scheduling fails.
	Schedule(task *DownloadTask) error
}

type downloadScheduler struct {
	queue *jobs.JobQueue
}

// NewDownloadScheduler create a DownloadScheduler to schedule download resources
func NewDownloadScheduler(queue *jobs.JobQueue) DownloadScheduler {
	return &downloadScheduler{
		queue: queue,
	}
}

// Schedule enqueues and resource link for download
func (ds *downloadScheduler) Schedule(task *DownloadTask) error {
	klog.V(6).Infof("[%s] linked resource %s scheduled for download as %s\n", task.Referer, task.Reference, task.Target)
	if !ds.queue.AddTask(task) {
		return fmt.Errorf("scheduling download of %s referenced by %s failed", task.Reference, task.Referer)
	}
	return nil
}

// DownloadTask holds information for source and target of linked document resources
type DownloadTask struct {
	Source    string
	Target    string
	Referer   string
	Reference string
}

type downloadWorker struct {
	// reader for resources
	reader Reader
	// writer for resources
	writer writers.Writer
	// lock for accessing the downloadedResources map
	mux sync.Mutex
	// map with downloaded resources
	downloadedResources map[string][]*DownloadTask
}

func (d *downloadWorker) Download(ctx context.Context, task interface{}) error {
	if dt, ok := task.(*DownloadTask); ok {
		if d.shouldDownload(dt) {
			if err := d.download(ctx, dt); err != nil {
				dErr := fmt.Errorf("downloading %s as %s and reference %s from referer %s failed: %v", dt.Source, dt.Target, dt.Reference, dt.Referer, err)
				if _, ok = err.(resourcehandlers.ErrResourceNotFound); ok {
					// for missing resources just log warning
					klog.Warning(dErr.Error())
					return nil
				}
				return dErr
			}
		}
	} else {
		return fmt.Errorf("incorrect download task: %T", task)
	}
	return nil
}

// shouldDownload checks whether a download task for the same DownloadTask.Source is being processed
func (d *downloadWorker) shouldDownload(dt *DownloadTask) bool {
	d.mux.Lock()
	defer d.mux.Unlock()
	if val, ok := d.downloadedResources[dt.Source]; ok {
		// there is already a task for this source, so just append the current one
		d.downloadedResources[dt.Source] = append(val, dt)
		return false
	}
	// add the task and starts downloading
	d.downloadedResources[dt.Source] = []*DownloadTask{dt}
	return true
}

func (d *downloadWorker) download(ctx context.Context, dt *DownloadTask) error {
	klog.V(6).Infof("downloading %s as %s\n", dt.Source, dt.Target)
	blob, err := d.reader.Read(ctx, dt.Source)
	if err != nil {
		return err
	}
	if err = d.writer.Write(dt.Target, "", blob, nil); err != nil {
		return err
	}
	return nil
}

// DownloadWorkFunc returns Download worker func
func DownloadWorkFunc(reader Reader, writer writers.Writer) (jobs.WorkerFunc, error) {
	if reader == nil || reflect.ValueOf(reader).IsNil() {
		return nil, errors.New("invalid argument: reader is nil")
	}
	if writer == nil || reflect.ValueOf(writer).IsNil() {
		return nil, errors.New("invalid argument: writer is nil")
	}
	dWorker := &downloadWorker{
		reader:              reader,
		writer:              writer,
		downloadedResources: make(map[string][]*DownloadTask),
	}
	return dWorker.Download, nil
}
