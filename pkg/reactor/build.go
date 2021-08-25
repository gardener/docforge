// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reactor

import (
	"context"
	"sync"

	"github.com/gardener/docforge/pkg/api"
	"github.com/hashicorp/go-multierror"
	"k8s.io/klog/v2"
)

func tasks(nodes []*api.Node, t *[]interface{}) {
	for _, node := range nodes {
		*t = append(*t, &DocumentWorkTask{
			Node: node,
		})
		if node.Nodes != nil {
			tasks(node.Nodes, t)
		}
	}
}

var validationWaitGroup sync.WaitGroup
var validationQueue = make(chan *validationTask, 200)

func validator(ctx context.Context, tasks <-chan *validationTask) {
	for {
		select {
		case <-ctx.Done():
			return
		case t, ok := <-tasks:
			if ok {
				validateLink(ctx, t)
			} else {
				return
			}
		}
	}
}

// Build starts the build operation for a document structure root
// in a locality domain
func (r *Reactor) Build(ctx context.Context, documentationStructure []*api.Node) error {
	var errors *multierror.Error

	errCh := make(chan error)
	doneCh := make(chan struct{})
	downloadShutdownCh := make(chan struct{})
	documentShutdownCh := make(chan struct{})
	gitInfoShutdownCh := make(chan struct{})
	loop := true

	defer func() {
		close(errCh)
		close(downloadShutdownCh)
		close(gitInfoShutdownCh)
		close(documentShutdownCh)
		close(doneCh)
		klog.V(1).Infoln("Build finished")
	}()

	// start validators
	for i := 0; i < 10; i++ {
		go validator(ctx, validationQueue)
	}

	// start download controller
	go func() {
		klog.V(6).Infoln("Starting download controller")
		r.DownloadController.Start(ctx, errCh, downloadShutdownCh)
	}()
	// start githubinfo controller
	if r.GitInfoController != nil {
		go func() {
			klog.V(6).Infoln("Starting GitHub Info controller")
			r.GitInfoController.Start(ctx, errCh, gitInfoShutdownCh)
		}()

	}
	// start document controller with download scope
	// r.DocController.SetDownloadScope(localityDomain)
	go func() {
		klog.V(6).Infoln("Starting document controller")
		r.DocController.Start(ctx, errCh, documentShutdownCh)
	}()

	// wait for all workers to exit then signal
	// we are all done.
	go func() {
		stoppedControllers := 0
		controllersCount := 2
		if r.GitInfoController != nil {
			controllersCount = controllersCount + 1
		}
		for stoppedControllers < controllersCount {
			select {
			case <-downloadShutdownCh:
				{
					klog.V(6).Infoln("Download controller stopped")
					stoppedControllers++
				}
			case <-gitInfoShutdownCh:
				{
					klog.V(6).Infoln("GitHub Info controller stopped")
					stoppedControllers++
				}
			case <-documentShutdownCh:
				{
					klog.V(6).Infoln("Document controller stopped")
					stoppedControllers++
					// propagate the stop to the related download controller
					r.DocController.GetDownloadController().Stop(nil)
					// propagate the stop to the related git info controller
					if r.GitInfoController != nil {
						r.GitInfoController.Stop(nil)
					}
				}
			}
		}
		doneCh <- struct{}{}
	}()

	// Enqueue tasks for document controller and signal it
	// to exit when ready
	go func() {
		documentPullTasks := make([]interface{}, 0)
		tasks(documentationStructure, &documentPullTasks)
		for _, task := range documentPullTasks {
			r.DocController.Enqueue(ctx, task)
		}
		klog.V(6).Infoln("Tasks for document controller enqueued")
		r.DocController.Stop(nil)
	}()

	// wait until done, context interrupted or error (in case error
	// policy is fail fast)
	for loop {
		select {
		case <-doneCh:
			{
				loop = false
			}
		case err, ok := <-errCh:
			{
				if ok {
					errors = multierror.Append(errors, err)
					if r.FailFast {
						loop = false
					}
				}
			}
		case <-ctx.Done():
			{
				loop = false
			}
		}
	}
	// wait for validation routines to complete & close task queue
	validationWaitGroup.Wait()
	close(validationQueue)

	return errors.ErrorOrNil()
}
