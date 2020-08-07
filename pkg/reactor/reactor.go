package reactor

import (
	"context"
	"fmt"
	"log"

	"github.com/gardener/docode/pkg/api"
	"github.com/gardener/docode/pkg/backend"
	"github.com/gardener/docode/pkg/jobs"
	"github.com/gardener/docode/pkg/jobs/worker"
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
	if node.Nodes != nil {
		for _, n := range node.Nodes {
			if err := r.Resolve(ctx, n); err != nil {
				return err
			}
		}
	}
	return nil
}

func sources(node *api.Node, resourcePathsSet map[string]struct{}) {
	if len(node.Source) > 0 {
		for _, s := range node.Source {
			resourcePathsSet[s] = struct{}{}
		}
	}
	if node.Nodes != nil {
		for _, n := range node.Nodes {
			sources(n, resourcePathsSet)
		}
	}
}

// Serialize resolves and serializes
func (r *Reactor) Serialize(ctx context.Context, docs *api.Documentation) error {
	if err := r.Resolve(ctx, docs.Root); err != nil {
		return err
	}

	documentationTasks := make([]interface{}, 0)
	tasks(docs.Root, &documentationTasks, r.ResourceHandlers)
	log.Println(len(documentationTasks))
	if err := r.ReplicateDocumentation.Dispatch(ctx, documentationTasks); err != nil {
		log.Println("ReplicatedDocumnetation")
		return err
	}

	// w, ok := r.ReplicateDocResources.Worker.(*worker.DocWorker)
	// if !ok {
	// 	panic("cast failed")
	// }

	// resourcesDataMap := make(map[string]string)
	// for resourceData := range w.RdCh {
	// 	resourcesDataMap[resourceData.Source] = resourceData.Target
	// }

	resoucesData := make([]interface{}, 0)
	// for s, t := range resourcesDataMap {
	// 	resoucesData = append(resoucesData, &worker.ResourceData{Source: s, Target: t})
	// }

	if err := r.ReplicateDocResources.Dispatch(ctx, resoucesData); err != nil {
		log.Println("ReplicatedDocumnetation")
		return err
	}

	return nil
}

func tasks(node *api.Node, t *[]interface{}, handlers backend.ResourceHandlers) {
	n := node
	if len(n.Source) > 0 {
		*t = append(*t, &worker.DocumentationTask{
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
