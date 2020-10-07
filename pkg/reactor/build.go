package reactor

import (
	"context"
	"fmt"

	"github.com/gardener/docforge/pkg/api"
	"github.com/hashicorp/go-multierror"
)

func tasks(node *api.Node, t *[]interface{}) {
	n := node
	*t = append(*t, &DocumentWorkTask{
		Node: n,
	})
	if node.Nodes != nil {
		for _, n := range node.Nodes {
			tasks(n, t)
		}
	}
}

// Build starts the build operation for a document structure root
// in a locality domain
func (r *Reactor) Build(ctx context.Context, documentationRoot *api.Node, localityDomain localityDomain) error {
	var errors *multierror.Error

	errCh := make(chan error)
	doneCh := make(chan struct{})
	downloadShutdownCh := make(chan struct{})
	documentShutdownCh := make(chan struct{})
	loop := true

	defer func() {
		close(errCh)
		close(downloadShutdownCh)
		close(documentShutdownCh)
		close(doneCh)
		fmt.Println("Build finished")
	}()

	// start download controller
	go func() {
		fmt.Println("Starting download controller")
		r.DownloadController.Start(ctx, errCh, downloadShutdownCh)
	}()
	// start document controller with download scope
	r.DocController.SetDownloadScope(localityDomain)
	go func() {
		fmt.Println("Starting document controller")
		r.DocController.Start(ctx, errCh, documentShutdownCh)
	}()

	// wait for all workers to exit then signal
	// we are all done.
	go func() {
		stoppedControllers := 0
		for stoppedControllers < 2 {
			select {
			case <-downloadShutdownCh:
				{
					fmt.Println("Download controller stopped")
					stoppedControllers++
				}
			case <-documentShutdownCh:
				{
					fmt.Println("Document controller stopped")
					stoppedControllers++
					// propagate the stop to the related download controller
					r.DocController.GetDownloadController().Stop(nil)
				}
			}
		}
		doneCh <- struct{}{}
	}()

	// Enqueue tasks for document controller and signal it
	// to exit when ready
	go func() {
		documentPullTasks := make([]interface{}, 0)
		tasks(documentationRoot, &documentPullTasks)
		for _, task := range documentPullTasks {
			r.DocController.Enqueue(ctx, task)
		}
		fmt.Println("Tasks for document controller enqueued")
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
					errors = multierror.Append(err)
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

	return errors.ErrorOrNil()
}
