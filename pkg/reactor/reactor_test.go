package reactor

import (
	"context"
	"reflect"
	"testing"

	"github.com/gardener/docode/pkg/api"
	"github.com/gardener/docode/pkg/backend"
	"github.com/gardener/docode/pkg/jobs/worker"
	"github.com/gardener/docode/pkg/util/tests"
)

func init() {
	tests.SetGlogV(6)
}

var (
	apiRefNode = &api.Node{
		Name:  "apiRef",
		Title: "API Reference",
		Source: []string{
			"https://github.com/org/repo/tree/master/docs/architecture/apireference.md",
		},
	}

	archNode = &api.Node{
		Name: "arch",
		Source: []string{
			"https://github.com/org/repo/tree/master/docs/architecture",
		},
		Title: "Architecture",
		Nodes: []*api.Node{
			apiRefNode,
		},
	}

	blogNode = &api.Node{
		Name: "blog",
		Source: []string{
			"https://github.com/org/repo/tree/master/docs/blog/blog-part1.md",
			"https://github.com/org/repo/tree/master/docs/blog/blog-part2.md",
		},
		Title: "Blog",
	}

	tasksNode = &api.Node{
		Name:  "tasks",
		Title: "Tasks",
		Source: []string{
			"https://github.com/org/repo/tree/master/docs/tasks",
		},
	}

	documentation = &api.Documentation{
		Root: &api.Node{
			Name:  "rootNode",
			Title: "Root node!",
			Source: []string{
				"https://github.com/org/repo/tree/master/docs",
			},
			Nodes: []*api.Node{
				archNode,
				blogNode,
				tasksNode,
			},
		},
	}
)

func TestSource(t *testing.T) {
	resourcePathsSet := make(map[string]struct{})
	sources(documentation.Root, resourcePathsSet)
}

func Test_tasks(t *testing.T) {
	type args struct {
		node     *api.Node
		parent   *api.Node
		tasks    []interface{}
		handlers backend.ResourceHandlers
	}
	tests := []struct {
		name          string
		args          args
		expectedTasks []interface{}
	}{
		{
			name: "it creates tasks based on the provided doc",
			args: args{
				node:     documentation.Root,
				parent:   nil,
				handlers: backend.ResourceHandlers{&FakeResourceHandler{}},
				tasks:    []interface{}{},
			},
			expectedTasks: []interface{}{
				&worker.DocumentationTask{
					Node:     documentation.Root,
					Handlers: backend.ResourceHandlers{&FakeResourceHandler{}},
				},
				&worker.DocumentationTask{
					Node:     archNode,
					Handlers: backend.ResourceHandlers{&FakeResourceHandler{}},
				},
				&worker.DocumentationTask{
					Node:     apiRefNode,
					Handlers: backend.ResourceHandlers{&FakeResourceHandler{}},
				},
				&worker.DocumentationTask{
					Node:     blogNode,
					Handlers: backend.ResourceHandlers{&FakeResourceHandler{}},
				},
				&worker.DocumentationTask{
					Node:     blogNode,
					Handlers: backend.ResourceHandlers{&FakeResourceHandler{}},
				},
				&worker.DocumentationTask{
					Node:     tasksNode,
					Handlers: backend.ResourceHandlers{&FakeResourceHandler{}},
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tasks(tc.args.node, &tc.args.tasks, tc.args.handlers)
			if !reflect.DeepEqual(tc.args.tasks, tc.expectedTasks) {
				t.Error("expected tasks are not equal to actual")
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
