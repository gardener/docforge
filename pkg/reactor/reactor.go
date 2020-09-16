package reactor

import (
	"context"
	"fmt"
	"sync"

	"github.com/gardener/docode/pkg/api"
	"github.com/gardener/docode/pkg/jobs"
	"github.com/gardener/docode/pkg/resourcehandlers"

	"github.com/hashicorp/go-multierror"
)

// Reactor orchestrates the documentation build workflow
type Reactor struct {
	ReplicateDocumentation *jobs.Job
}

// Run starts build operation on docStruct
func (r *Reactor) Run(ctx context.Context, docStruct *api.Documentation, dryRun bool) error {
	var err error
	if err := r.Resolve(ctx, docStruct.Root); err != nil {
		return err
	}

	localityDomain := docStruct.LocalityDomain
	if localityDomain == nil || len(localityDomain) == 0 {
		if localityDomain, err = defineLocalityDomains(docStruct.Root); err != nil {
			return err
		}
		docStruct.LocalityDomain = localityDomain
	}

	if dryRun {
		s, err := api.Serialize(docStruct)
		if err != nil {
			return err
		}
		fmt.Println(s)
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	fmt.Printf("Building documentation structure\n\n")
	if err = r.Build(ctx, docStruct.Root, localityDomain); err != nil {
		return err
	}

	return nil
}

// Resolve builds the subnodes hierarchy of a node based on the natural nodes
// hierarchy and on rules such as those in NodeSelector.
// The node hierarchy is resolved by an appropriate handler selected based
// on the NodeSelector path URI
// The resulting model is the actual flight plan for replicating resources.
func (r *Reactor) Resolve(ctx context.Context, node *api.Node) error {
	node.SetParentsDownwards()
	if node.NodeSelector != nil {
		var handler resourcehandlers.ResourceHandler
		if handler = resourcehandlers.Get(node.NodeSelector.Path); handler == nil {
			return fmt.Errorf("No suitable handler registered for path %s", node.NodeSelector.Path)
		}
		if err := handler.ResolveNodeSelector(ctx, node); err != nil {
			return err
		}
	}
	if len(node.Nodes) > 0 {
		for _, n := range node.Nodes {
			if err := r.Resolve(ctx, n); err != nil {
				return err
			}
		}
	}
	return nil
}

func tasks(node *api.Node, t *[]interface{}) {
	n := node
	if len(n.ContentSelectors) > 0 {
		*t = append(*t, &DocumentWorkTask{
			Node: n,
		})
	}
	if node.Nodes != nil {
		for _, n := range node.Nodes {
			tasks(n, t)
		}
	}
}

// Build starts the build operation for a document structure root
// in a locality domain
func (r *Reactor) Build(ctx context.Context, documentationRoot *api.Node, localityDomain LocalityDomain) error {
	var (
		errors    *multierror.Error
		docWorker = r.ReplicateDocumentation.Worker.(*DocumentWorker)
		wg        sync.WaitGroup
	)

	errCh := make(chan error)
	doneCh := make(chan struct{})
	shutdownCh := make(chan struct{})
	defer func() {
		close(errCh)
		close(doneCh)
		close(shutdownCh)
		wg.Wait()

		fmt.Println("Build finished")
	}()

	go docWorker.NodeContentProcessor.DownloadJob.Start(ctx, errCh, shutdownCh, &wg)

	go func() {
		docWorker.NodeContentProcessor.LocalityDomain = localityDomain
		documentPullTasks := make([]interface{}, 0)
		tasks(documentationRoot, &documentPullTasks)
		if err := r.ReplicateDocumentation.Dispatch(ctx, documentPullTasks); err != nil {
			errCh <- err
		}
		doneCh <- struct{}{}
	}()

	for {
		select {
		case <-doneCh:
			{
				shutdownCh <- struct{}{}
				return nil
			}
		case err, ok := <-errCh:
			{
				if !ok {
					return nil
				}
				fmt.Printf("Error received %v\n", err)
				// TODO: fault tolerant vs failfast
				errors = multierror.Append(err)
				return errors.ErrorOrNil()
			}
		case <-ctx.Done():
			{
				fmt.Println("Context cancelled")
				return nil
			}
		}
	}

}
