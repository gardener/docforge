package reactor

import (
	"context"
	"reflect"
	"testing"

	"github.com/gardener/docode/pkg/api"
	"github.com/gardener/docode/pkg/resourcehandlers"
	"github.com/gardener/docode/pkg/util/tests"
)

func init() {
	tests.SetGlogV(6)
}

var (
	apiRefNode = &api.Node{
		Name:             "apiRef",
		Title:            "API Reference",
		ContentSelectors: []api.ContentSelector{{Source: "https://github.com/org/repo/tree/master/docs/architecture/apireference.md"}},
	}

	archNode = &api.Node{
		Name:             "arch",
		ContentSelectors: []api.ContentSelector{{Source: "https://github.com/org/repo/tree/master/docs/architecture"}},
		Title:            "Architecture",
		Nodes: []*api.Node{
			apiRefNode,
		},
	}

	blogNode = &api.Node{
		Name:             "blog",
		ContentSelectors: []api.ContentSelector{{Source: "https://github.com/org/repo/tree/master/docs/blog/blog-part1.md"}},
		Title:            "Blog",
	}

	tasksNode = &api.Node{
		Name:             "tasks",
		Title:            "Tasks",
		ContentSelectors: []api.ContentSelector{{Source: "https://github.com/org/repo/tree/master/docs/tasks"}},
	}

	documentation = &api.Documentation{
		Root: &api.Node{
			Name:             "rootNode",
			Title:            "Root node!",
			ContentSelectors: []api.ContentSelector{{Source: "https://github.com/org/repo/tree/master/docs"}},
			Nodes: []*api.Node{
				archNode,
				blogNode,
				tasksNode,
			},
		},
	}
)

func Test_tasks(t *testing.T) {
	type args struct {
		node   *api.Node
		parent *api.Node
		tasks  []interface{}
	}
	tests := []struct {
		name          string
		args          args
		expectedTasks []interface{}
	}{
		{
			name: "it creates tasks based on the provided doc",
			args: args{
				node:   documentation.Root,
				parent: nil,
				tasks:  []interface{}{},
			},
			expectedTasks: []interface{}{
				&DocumentWorkTask{
					Node: documentation.Root,
				},
				&DocumentWorkTask{
					Node: archNode,
				},
				&DocumentWorkTask{
					Node: apiRefNode,
				},
				&DocumentWorkTask{
					Node: blogNode,
				},
				&DocumentWorkTask{
					Node: tasksNode,
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resourcehandlers.Load(&FakeResourceHandler{})
			tasks(tc.args.node, &tc.args.tasks)
			if !reflect.DeepEqual(tc.args.tasks, tc.expectedTasks) {
				t.Errorf("expected tasks %v !=  %v", tc.expectedTasks, tc.args.tasks)
			}
		})
	}
}

type FakeResourceHandler struct{}

func (f *FakeResourceHandler) Accept(uri string) bool {
	return true
}

func (f *FakeResourceHandler) ResolveNodeSelector(ctx context.Context, node *api.Node) error {
	return nil
}

func (f *FakeResourceHandler) Read(ctx context.Context, uri string) ([]byte, error) {
	return []byte(uri), nil
}

func (f *FakeResourceHandler) Name(uri string) string {
	return uri
}

func (f *FakeResourceHandler) ResolveRelLink(source, relLink string) (string, bool) {
	return relLink, false
}
