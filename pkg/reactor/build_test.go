package reactor

import (
	"testing"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/resourcehandlers"
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
