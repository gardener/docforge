// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reactor

import (
	"testing"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/resourcehandlers/resourcehandlersfakes"

	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/util/tests"
)

func init() {
	tests.SetKlogV(6)
}

var (
	apiRefNode = &manifest.Node{
		FileType: manifest.FileType{File: "apiRef"},
	}

	archNode = &manifest.Node{
		DirType: manifest.DirType{
			Dir: "arch",
			Structure: []*manifest.Node{
				apiRefNode,
			}},
	}

	blogNode = &manifest.Node{
		FileType: manifest.FileType{File: "blog"},
	}

	tasksNode = &manifest.Node{
		FileType: manifest.FileType{File: "tasks"},
	}
)

func createNewDocumentation() *manifest.Node {
	return &manifest.Node{
		DirType: manifest.DirType{
			Structure: []*manifest.Node{
				{
					DirType: manifest.DirType{
						Dir: "rootNode",
						Structure: []*manifest.Node{
							archNode,
							blogNode,
							tasksNode,
						},
					},
				},
			},
		},
	}
}

func Test_tasks(t *testing.T) {
	newDoc := createNewDocumentation()
	type args struct {
		node  *manifest.Node
		tasks []interface{}
		// lds   localityDomain
	}
	tests := []struct {
		name          string
		args          args
		expectedTasks []*DocumentWorkTask
	}{
		{
			name: "it creates tasks based on the provided doc",
			args: args{
				node:  newDoc.Structure[0],
				tasks: []interface{}{},
			},
			expectedTasks: []*DocumentWorkTask{
				{
					Node: newDoc.Structure[0],
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
			fakeRH := resourcehandlersfakes.FakeResourceHandler{}
			rhs := resourcehandlers.NewRegistry(&fakeRH)
			tasks([]*manifest.Node{tc.args.node}, &tc.args.tasks)

			if len(tc.args.tasks) != len(tc.expectedTasks) {
				t.Errorf("expected number of tasks %d != %d", len(tc.expectedTasks), len(tc.args.tasks))
			}

			for i, task := range tc.args.tasks {
				if task.(*DocumentWorkTask).Node.Name() != tc.expectedTasks[i].Node.Name() {
					t.Errorf("expected task with Node name %s != %s", task.(*DocumentWorkTask).Node.Name(), tc.expectedTasks[i].Node.Name())
				}
			}
			rhs.Remove()
		})
	}
}
