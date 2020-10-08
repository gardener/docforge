package reactor

import (
	"context"
	// "fmt"

	"testing"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/util/tests"
)

func init() {
	tests.SetKlogV(6)
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
)

func Test_tasks(t *testing.T) {
	newDoc := createNewDocumentation()
	type args struct {
		node  *api.Node
		tasks []interface{}
		lds   localityDomain
	}
	tests := []struct {
		name          string
		args          args
		expectedTasks []*DocumentWorkTask
	}{
		{
			name: "it creates tasks based on the provided doc",
			args: args{
				node:  newDoc.Root,
				tasks: []interface{}{},
			},
			expectedTasks: []*DocumentWorkTask{
				{
					Node: newDoc.Root,
				},
				{
					Node: archNode,
				},
				{
					Node: apiRefNode,
				},
				{
					Node: blogNode,
				},
				{
					Node: tasksNode,
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rhs := resourcehandlers.NewRegistry(&FakeResourceHandler{})
			tasks(tc.args.node, &tc.args.tasks)

			if len(tc.args.tasks) != len(tc.expectedTasks) {
				t.Errorf("expected number of tasks %d != %d", len(tc.expectedTasks), len(tc.args.tasks))
			}

			for i, task := range tc.args.tasks {
				if task.(*DocumentWorkTask).Node.Name != tc.expectedTasks[i].Node.Name {
					t.Errorf("expected task with Node name %s != %s", task.(*DocumentWorkTask).Node.Name, tc.expectedTasks[i].Node.Name)
				}
			}
			rhs.Remove()
		})
	}
}

func createNewDocumentation() *api.Documentation {
	return &api.Documentation{
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

func (f *FakeResourceHandler) BuildAbsLink(source, relLink string) (string, error) {
	return relLink, nil
}

func (f *FakeResourceHandler) GetLocalityDomainCandidate(source string) (string, string, string, error) {
	return source, source, "", nil
}

func (f *FakeResourceHandler) SetVersion(link, version string) (string, error) {
	return link, nil
}
