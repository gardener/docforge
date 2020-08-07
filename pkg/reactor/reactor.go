package reactor

import (
	"context"
	"fmt"
	"strings"

	"github.com/gardener/docode/pkg/api"
	"github.com/gardener/docode/pkg/backend"
	"github.com/gardener/docode/pkg/jobs"
)

// Reactor orchestrates the documentation build workflow
type Reactor struct {
	ResourceHandlers backend.ResourceHandlers
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

func tasks(node *api.Node, parent *api.Node, t []interface{}, handlers backend.ResourceHandlers) {
	if node.Nodes != nil {
		for _, n := range node.Nodes {
			if len(n.Source) > 0 {
				// for _, s := range n.Source {
				parents := node.Parents()
				pathSegments := make([]string, len(parents))
				for _, p := range parents {
					pathSegments = append(pathSegments, p.Name)
				}
				path := strings.Join(pathSegments, "/")
				t = append(t, &Task{
					node:      n,
					localPath: path,
				})
				// }
			}
			tasks(n, node, t, handlers)
		}
	}
}

func (r *Reactor) Serialize(ctx context.Context, docs *api.Documentation) error {

	rWorker := &Worker{
		ResourceHandlers: r.ResourceHandlers,
	}

	job := &jobs.Job{
		MaxWorkers: 50,
		MinWorkers: 1,
		FailFast:   false,
		Worker:     rWorker,
	}

	t := make([]interface{}, 0)
	tasks(docs.Root, nil, t, r.ResourceHandlers)

	return job.Dispatch(ctx, t)
}

func (r *Reactor) Run(ctx context.Context, docs *api.Documentation) {
	r.Resolve(ctx, docs.Root)
	r.Serialize(ctx, docs)
}
