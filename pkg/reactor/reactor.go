package reactor

import (
	"context"
	"fmt"
	"sync"

	"github.com/gardener/docode/pkg/api"
	"github.com/gardener/docode/pkg/jobs"
	"github.com/gardener/docode/pkg/resourcehandlers"
)

// Reactor orchestrates the documentation build workflow
type Reactor struct {
	ReplicateDocumentation *jobs.Job
	ReplicateDocResources  *jobs.Job
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

// Run TODO:
func (r *Reactor) Run(ctx context.Context, docStruct *api.Documentation) error {
	if err := r.Resolve(ctx, docStruct.Root); err != nil {
		return err
	}

	// doc, _ := yaml.Marshal(docStruct.Root)
	docCtx, cancelF := context.WithCancel(ctx)
	errCh := make(chan error)
	go r.replicateDocumentation(docCtx, cancelF, docStruct.Root, errCh)

	resoucesData := make([]interface{}, 0)
	docWorker := r.ReplicateDocumentation.Worker.(*DocumentWorker)

	for working := true; working; {
		select {
		case x := <-docWorker.RdCh:
			resoucesData = append(resoucesData, x)
		case <-docCtx.Done():
			working = false
		case err := <-errCh:
			return err
		}
	}

	if err := r.ReplicateDocResources.Dispatch(ctx, resoucesData); err != nil {
		return err
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

func (r *Reactor) replicateDocumentation(ctx context.Context, cancelF context.CancelFunc, documentation *api.Node, errCh chan error) {
	defer cancelF()
	documentPullTasks := make([]interface{}, 0)
	tasks(documentation, &documentPullTasks)
	if err := r.ReplicateDocumentation.Dispatch(ctx, documentPullTasks); err != nil {
		errCh <- err
	}
	docWorker := r.ReplicateDocumentation.Worker.(*DocumentWorker)
	close(docWorker.RdCh)
	close(errCh)
}

type DownloadedResources struct {
	resources map[string]struct{}
	mutex     sync.RWMutex
}
