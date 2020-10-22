package reactor

import (
	"github.com/gardener/docforge/pkg/jobs"
)

// DocumentController is the functional interface for a controller
// handling tasks for processing enqueued documents. It amends the
// jobs.Controller interface with specific methods.
type DocumentController interface {
	jobs.Controller
	// SetDownloadScope sets the scope for resources considered "local"
	// and therefore downloaded and relatively linked
	// SetDownloadScope(scope *localityDomain)
	// GetDownloadController is accessor for the DownloadController
	// working with this DocumentController
	GetDownloadController() DownloadController
}

type docController struct {
	jobs.Controller
	*jobs.Job
}

// NewDocumentController creates a controller for processing documents.
func NewDocumentController(worker *DocumentWorker, workersCount int, failfast bool) DocumentController {
	job := &jobs.Job{
		ID:                        "Document",
		MinWorkers:                workersCount,
		MaxWorkers:                workersCount,
		FailFast:                  failfast,
		Worker:                    worker,
		Queue:                     jobs.NewWorkQueue(2 * workersCount),
		IsWorkerExitsOnEmptyQueue: true,
	}
	return &docController{
		jobs.NewController(job),
		job,
	}
}
func (d *docController) Shutdown() {
	d.Controller.Shutdown()
	// propagate the shutdown to the related download controller
	d.Worker.(*DocumentWorker).NodeContentProcessor.GetDownloadController().Shutdown()
}

// func (d *docController) SetDownloadScope(scope *localityDomain) {
// 	d.Worker.(*DocumentWorker).NodeContentProcessor.localityDomain = scope
// }
func (d *docController) GetDownloadController() DownloadController {
	return d.Worker.(*DocumentWorker).NodeContentProcessor.GetDownloadController()
}
