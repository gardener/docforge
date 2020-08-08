package reactor

import (
	"context"
	"fmt"

	"github.com/gardener/docode/pkg/api"
	"github.com/gardener/docode/pkg/backend"
	"github.com/gardener/docode/pkg/jobs"
)

// Reactor orchestrates the documentation build workflow
type Reactor struct {
	ResourceHandlers       backend.ResourceHandlers //TODO: think of global registry
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
	if &node.NodeSelector != nil {
		var handler backend.ResourceHandler
		if handler = r.ResourceHandlers.Get(node.NodeSelector.Path); handler == nil {
			return fmt.Errorf("No suitable handler registered for path %s", node.NodeSelector.Path)
		}
		if err := handler.ResolveNodeSelector(ctx, node); err != nil {
			return err
		}
		return nil
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

// func sources(node *api.Node, resourcePathsSet map[string]struct{}) {
// 	if len(node.Source) > 0 {
// 		for _, s := range node.Source {
// 			resourcePathsSet[s] = struct{}{}
// 		}
// 	}
// 	if node.Nodes != nil {
// 		for _, n := range node.Nodes {
// 			sources(n, resourcePathsSet)
// 		}
// 	}
// }

func tasks(node *api.Node, t *[]interface{}, handlers backend.ResourceHandlers) {
	n := node
	if len(n.Source) > 0 {
		*t = append(*t, &DocumentWorkTask{
			Node:     n,
			Handlers: handlers,
		})
	}
	if node.Nodes != nil {
		for _, n := range node.Nodes {
			tasks(n, t, handlers)
		}
	}
}

func (r *Reactor) Run(ctx context.Context, docStruct *api.Documentation) error {
	if err := r.Resolve(ctx, docStruct.Root); err != nil {
		return err
	}

	documentPullTasks := make([]interface{}, 0)
	tasks(docStruct.Root, &documentPullTasks, r.ResourceHandlers)
	if err := r.ReplicateDocumentation.Dispatch(ctx, documentPullTasks); err != nil {
		return err
	}

	// resoucesData := make([]interface{}, 0)
	// docWorker := r.ReplicateDocumentation.Worker.(*DocumentWorker)

	// for rd := range docWorker.RdCh {
	// 	resoucesData = append(resoucesData, rd)
	// }

	// if err := r.ReplicateDocResources.Dispatch(ctx, resoucesData); err != nil {
	// 	return err
	// }

	return nil
}
