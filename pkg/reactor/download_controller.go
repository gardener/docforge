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
	Source string
	Target string
}

type worker interface {
	download(ctx context.Context, dt *DownloadTask) error
}

// DownloadController encapsulates activities for asyncronous
// and parallel scheduling and download of resources
type DownloadController interface {
	jobs.Controller
	// Schedule is a typesafe wrtapper around Controller#Enqueue
	// for enquing download tasks
	Schedule(ctx context.Context, link, resourceName string)
}

// downloadController implements reactor#DownloadController
type downloadController struct {
	jobs.Controller
	worker
	rwlock              sync.RWMutex
	downloadedResources map[string]struct{}
	job                 *jobs.Job
	jobs.Worker
}

type downloadWorker struct {
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

	d := &downloadWorker{
		Reader: reader,
		Writer: writer,
	}

	job := &jobs.Job{
		ID:                        "Download",
		FailFast:                  failFast,
		MaxWorkers:                workersCount,
		MinWorkers:                workersCount,
		Queue:                     jobs.NewWorkQueue(100),
		IsWorkerExitsOnEmptyQueue: true,
	}
	controller := &downloadController{
		Controller:          jobs.NewController(job),
		worker:              d,
		downloadedResources: make(map[string]struct{}),
		job:                 job,
	}
	controller.job.Worker = withController(d, controller)
	return controller
}

func (d *downloadWorker) download(ctx context.Context, dt *DownloadTask) error {
	klog.V(6).Infof("Downloading %s as %s\n", dt.Source, dt.Target)
	blob, err := d.Reader.Read(ctx, dt.Source)
	if err != nil {
		return err
	}

	if err := d.Writer.Write(dt.Target, "", blob, nil); err != nil {
		return err
	}
	return nil
}

func withController(downloadWorker *downloadWorker, ctrl *downloadController) jobs.WorkerFunc {
	return func(ctx context.Context, task interface{}, wq jobs.WorkQueue) *jobs.WorkerError {
		return downloadWorker.Work(ctx, ctrl, task, wq)
	}
}

func (d *downloadWorker) Work(ctx context.Context, ctrl *downloadController, task interface{}, wq jobs.WorkQueue) *jobs.WorkerError {
	if task, ok := task.(*DownloadTask); ok {
		if !ctrl.isDownloaded(task) {
			if err := d.download(ctx, task); err != nil {
				return jobs.NewWorkerError(err, 0)
			}
			ctrl.setDownloaded(task)
		}
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
	c.downloadedResources[dt.Source] = struct{}{}
}

// Schedule enqeues and resource link for download
func (c *downloadController) Schedule(ctx context.Context, link, resourceName string) {
	task := &DownloadTask{
		Source: link,
		Target: resourceName,
	}
	c.Enqueue(ctx, task)
}

func (c *downloadController) Stop(shutdownCh chan struct{}) {
	// CheckExit immediately if nothing in queue and blocked waiting
	if c.job.Queue.Count() == 0 {
		c.Controller.Shutdown()
	}
	c.Controller.Stop(shutdownCh)
}
