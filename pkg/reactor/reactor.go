package reactor

import (
	"context"
	"fmt"
	// "reflect"
	"strings"

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

	resoucesData := make(map[string]*ResourceData)
	docWorker := r.ReplicateDocumentation.Worker.(*DocumentWorker)
	for working := true; working; {
		select {
		case rd := <-docWorker.RdCh:
			resoucesData[rd.Source] = rd
		case <-docCtx.Done():
			working = false
		case err := <-errCh:
			return err
		}
	}

	tasks := make([]interface{}, 0, len(resoucesData))
	for _, t := range resoucesData {
		tasks = append(tasks, t)
	}

	if err := r.ReplicateDocResources.Dispatch(ctx, tasks); err != nil {
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
}

// Returns the relative path between two nodes on the same tree, formatted
// with `..` for ancestors path if any and `.` for current node in relative
// path to descendant. The funciton can also calculate path to a node on another
// branch
func relativePath(from, to *api.Node) string {
	if from == to {
		return ""
	}
	fromPathToRoot := append(from.Parents(), from)
	toPathToRoot := append(to.Parents(), to)
	if intersection := intersect(fromPathToRoot, toPathToRoot); len(intersection) > 0 {
		// to is descendant
		if intersection[len(intersection)-1] == from {
			toPathToRoot = toPathToRoot[(len(intersection)-1):]
			s:= []string{}
			for _, n := range toPathToRoot {
				s = append(s, n.Name)
			}
			s[0] = "."
			return strings.Join(s, "/")
		}
		// to is ancestor
		if intersection[len(intersection)-1] == to {
			fromPathToRoot = fromPathToRoot[(len(intersection)-1):]
			s:= []string{}
			for _ = range toPathToRoot {
				s = append(s, "..")
			}
			s[len(s)-1] = fromPathToRoot[0].Name
			return strings.Join(s, "/")
		}
		fromPathToRoot = fromPathToRoot[(len(intersection)-1):]
		s:= []string{}
		for _ = range toPathToRoot {
			s = append(s, "..")
		}
		// to is on another branch
		toPathToRoot = toPathToRoot[len(intersection):]
		for _, n := range toPathToRoot {
			s = append(s, n.Name)
		}
		return strings.Join(s, "/")
	}
	return ""
}

func intersect(a, b []*api.Node) []*api.Node {
	intersection := make([]*api.Node, 0)
	hash := make(map[*api.Node]struct{})
	for _, v := range a {
		hash[v] = struct{}{}
	}
	for _, v := range b {
		if _, found := hash[v]; found {
			intersection = append(intersection, v)
		}
	}
	return intersection
}
