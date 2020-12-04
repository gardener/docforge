// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reactor

import (
	"github.com/gardener/docforge/pkg/jobs"
	"k8s.io/klog/v2"
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
		ID:         "Document",
		MinWorkers: workersCount,
		MaxWorkers: workersCount,
		FailFast:   failfast,
		Worker:     worker,
		Queue:      jobs.NewWorkQueue(2 * workersCount),
	}
	job.SetIsWorkerExitsOnEmptyQueue(true)
	return &docController{
		jobs.NewController(job),
		job,
	}
}
func (d *docController) Shutdown() {
	// propagate the shutdown to the related download controller
	defer d.Worker.(*DocumentWorker).NodeContentProcessor.GetDownloadController().Shutdown()
	klog.Warning("Shutting down Doc controller")
	d.Controller.Shutdown()
}
func (d *docController) GetDownloadController() DownloadController {
	return d.Worker.(*DocumentWorker).NodeContentProcessor.GetDownloadController()
}
