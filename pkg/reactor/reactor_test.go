// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reactor

import (
	"context"
	"errors"
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/util/tests"
)

func init() {
	tests.SetKlogV(6)
}

var (
	apiRefNode = &api.Node{
		Name:             "apiRef",
		ContentSelectors: []api.ContentSelector{{Source: "https://github.com/org/repo/tree/master/docs/architecture/apireference.md"}},
	}

	archNode = &api.Node{
		Name:             "arch",
		ContentSelectors: []api.ContentSelector{{Source: "https://github.com/org/repo/tree/master/docs/architecture"}},
		Nodes: []*api.Node{
			apiRefNode,
		},
	}

	blogNode = &api.Node{
		Name:             "blog",
		ContentSelectors: []api.ContentSelector{{Source: "https://github.com/org/repo/tree/master/docs/blog/blog-part1.md"}},
	}

	tasksNode = &api.Node{
		Name:             "tasks",
		ContentSelectors: []api.ContentSelector{{Source: "https://github.com/org/repo/tree/master/docs/tasks"}},
	}
)

func createNewDocumentation() *api.Documentation {
	return &api.Documentation{
		Structure: []*api.Node{
			{
				Name:             "rootNode",
				ContentSelectors: []api.ContentSelector{{Source: "https://github.com/org/repo/tree/master/docs"}},
				Nodes: []*api.Node{
					archNode,
					blogNode,
					tasksNode,
				},
			},
		},
	}
}

type FakeResourceHandler struct{}

func (f *FakeResourceHandler) Accept(uri string) bool {
	return true
}

func (f *FakeResourceHandler) ResolveNodeSelector(ctx context.Context, node *api.Node, excludePaths []string, frontMatter map[string]interface{}, excludeFrontMatter map[string]interface{}, depth int32) ([]*api.Node, error) {
	return nil, nil
}

func (f *FakeResourceHandler) ResolveDocumentation(ctx context.Context, uri string) (*api.Documentation, error) {
	return nil, nil
}

func (f *FakeResourceHandler) Read(ctx context.Context, uri string) ([]byte, error) {
	return []byte(uri), nil
}
func (f *FakeResourceHandler) ReadGitInfo(ctx context.Context, uri string) ([]byte, error) {
	return []byte(""), nil
}

func (f *FakeResourceHandler) Name(uri string) string {
	return uri
}

func (f *FakeResourceHandler) ResourceName(uri string) (string, string) {
	return "", ""
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

func (f *FakeResourceHandler) GetRawFormatLink(link string) (string, error) {
	return link, nil
}

func Test_getNodeParentPath(t *testing.T) {
	type args struct {
		node     *api.Node
		parrents []*api.Node
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Pass nil parent node",
			args: args{node: nil},
			want: "root",
		},
		{
			name: "Pass node without parent",
			args: args{node: &api.Node{Name: "top"}},
			want: "top",
		},
		{
			name: "Pass node with one ancestor",
			args: args{
				node: &api.Node{Name: "father"},
				parrents: []*api.Node{
					{Name: "grandfather"},
				},
			},
			want: "grandfather.father",
		},
		{
			name: "Pass node with two ancestor",
			args: args{
				node: &api.Node{Name: "son"},
				parrents: []*api.Node{
					{Name: "grandfather"},
					{Name: "father"},
				},
			},
			want: "grandfather.father.son",
		},
	}
	for _, tt := range tests {
		setParents(tt.args.node, tt.args.parrents)
		t.Run(tt.name, func(t *testing.T) {
			if got := getNodeParentPath(tt.args.node); got != tt.want {
				t.Errorf("getNodeParentPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_buildNodeCollision(t *testing.T) {
	type args struct {
		nodes           []*api.Node
		parent          *api.Node
		collisionsNames []string
	}
	tests := []struct {
		name string
		args args
		want collision
	}{
		{
			name: "Pass nodes with one colission of two nodes",
			args: args{
				nodes: []*api.Node{
					{Name: "foo", Source: "foo/bar"},
					{Name: "foo", Source: "baz/bar/foo"},
				},
				parent:          &api.Node{Name: "parent"},
				collisionsNames: []string{"foo"},
			},
			want: collision{
				nodeParentPath: "parent",
				collidedNodes: map[string][]string{
					"foo": {"foo/bar", "baz/bar/foo"},
				},
			},
		},
		{
			name: "Pass nodes with one colission of three nodes",
			args: args{
				nodes: []*api.Node{
					{Name: "foo", Source: "foo/bar"},
					{Name: "foo", Source: "baz/bar/foo"},
					{Name: "foo", Source: "baz/bar/foo/fuz"},
				},
				parent:          &api.Node{Name: "parent"},
				collisionsNames: []string{"foo"},
			},
			want: collision{
				nodeParentPath: "parent",
				collidedNodes: map[string][]string{
					"foo": {"foo/bar", "baz/bar/foo", "baz/bar/foo/fuz"},
				},
			},
		},
		{
			name: "Pass nodes with two colission",
			args: args{
				nodes: []*api.Node{
					{Name: "foo", Source: "foo/bar"},
					{Name: "foo", Source: "baz/bar/foo"},
					{Name: "moo", Source: "moo/bar"},
					{Name: "moo", Source: "baz/bar/moo"},
					{Name: "normal", Source: "baz/bar/moo"},
				},
				parent:          &api.Node{Name: "parent"},
				collisionsNames: []string{"foo", "moo"},
			},
			want: collision{
				nodeParentPath: "parent",
				collidedNodes: map[string][]string{
					"foo": {"foo/bar", "baz/bar/foo"},
					"moo": {"moo/bar", "baz/bar/moo"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildNodeCollision(tt.args.nodes, tt.args.parent, tt.args.collisionsNames); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildNodeCollision() = %v, want %v", got, tt.want)
			}
		})
	}
}

func setParents(node *api.Node, parents []*api.Node) {
	currentNode := node
	for i := len(parents) - 1; i >= 0; i-- {
		parent := parents[i]
		currentNode.SetParent(parent)
		currentNode = parent
	}
}

func Test_checkNodesForCollision(t *testing.T) {
	type args struct {
		nodes      []*api.Node
		parent     *api.Node
		collisions []collision
	}
	tests := []struct {
		name string
		args args
		want []collision
	}{
		{
			name: "Nodes with one collision",
			args: args{
				nodes: []*api.Node{
					{Name: "foo", Source: "bar/baz"},
					{Name: "foo", Source: "baz/foo"},
					{Name: "normal", Source: "baz/foo"},
				},
				parent: &api.Node{
					Name: "parent",
				},
			},
			want: []collision{
				{
					nodeParentPath: "parent",
					collidedNodes: map[string][]string{
						"foo": {"bar/baz", "baz/foo"},
					},
				},
			},
		},
		{
			name: "Nodes with two collision",
			args: args{
				nodes: []*api.Node{
					{Name: "foo", Source: "bar/baz"},
					{Name: "foo", Source: "baz/foo"},
					{Name: "normal", Source: "baz/foo"},
					{Name: "moo", Source: "bar/baz"},
					{Name: "moo", Source: "baz/moo"},
				},
				parent: &api.Node{
					Name: "parent",
				},
			},
			want: []collision{
				{
					nodeParentPath: "parent",
					collidedNodes: map[string][]string{
						"foo": {"bar/baz", "baz/foo"},
						"moo": {"bar/baz", "baz/moo"},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkNodesForCollision(tt.args.nodes, tt.args.parent, tt.args.collisions)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("checkNodesForCollision() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_checkForCollisions(t *testing.T) {
	type args struct {
		nodes []*api.Node
	}
	tests := []struct {
		name string
		args args
		want error
	}{
		{
			name: "Test with one collision",
			args: args{
				nodes: []*api.Node{
					{
						Name: "grandfather",
						Nodes: []*api.Node{
							{
								Name: "parent",
								Nodes: []*api.Node{
									{Name: "son", Source: "https://foo/bar/son"},
									{Name: "son", Source: "https://foo/bar/bor"},
								},
							},
						},
					},
				},
			},
			want: errors.New("Node collisions detected.\nIn grandfather.parent container node. Node with name son appears 2 times for sources: https://foo/bar/son, https://foo/bar/bor."),
		},
		{
			name: "Test with many collisions",
			args: args{
				nodes: []*api.Node{
					{
						Name: "grandfather",
						Nodes: []*api.Node{
							{
								Name: "father",
								Nodes: []*api.Node{
									{Name: "son", Source: "https://foo/bar/son"},
									{Name: "son", Source: "https://foo/bar/bor"},
								},
							},
							{
								Name: "mother",
								Nodes: []*api.Node{
									{Name: "daughter", Source: "https://foo/bar/daughter"},
									{Name: "daughter", Source: "https://foo/daughter/bor"},
								},
							},
						},
					},
					{
						Name: "grandmother",
						Nodes: []*api.Node{
							{
								Name: "father",
								Nodes: []*api.Node{
									{Name: "son", Source: "https://foo/bar/son"},
									{Name: "son", Source: "https://foo/bar/bor"},
								},
							},
							{
								Name: "mother",
								Nodes: []*api.Node{
									{Name: "daughter", Source: "https://foo/bar/daughter"},
									{Name: "daughter", Source: "https://foo/daughter/bor"},
								},
							},
						},
					},
					{
						Name:   "grandmother",
						Source: "https://some/url/to/source",
					},
				},
			},
			want: errors.New("Node collisions detected.\nIn root container node. Node with name grandmother appears 2 times for sources: , https://some/url/to/source.\nIn grandfather.father container node. Node with name son appears 2 times for sources: https://foo/bar/son, https://foo/bar/bor.\nIn grandfather.mother container node. Node with name daughter appears 2 times for sources: https://foo/bar/daughter, https://foo/daughter/bor.\nIn grandmother.father container node. Node with name son appears 2 times for sources: https://foo/bar/son, https://foo/bar/bor.\nIn grandmother.mother container node. Node with name daughter appears 2 times for sources: https://foo/bar/daughter, https://foo/daughter/bor."),
		},
		{
			name: "Test without collision",
			args: args{
				nodes: []*api.Node{
					{
						Name: "l1",
						Nodes: []*api.Node{
							{
								Name: "l11",
								Nodes: []*api.Node{
									{Name: "l111", Source: "https://foo/bar/l111"},
									{Name: "l112", Source: "https://foo/bar/l112"},
								},
							},
						},
					},
					{
						Name: "l2",
						Nodes: []*api.Node{
							{
								Name: "l21",
							},
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "Test with nodes after collision",
			args: args{
				nodes: []*api.Node{
					{
						Name: "l1",
						Nodes: []*api.Node{
							{
								Name: "l11",
								Nodes: []*api.Node{
									{Name: "l111", Source: "https://foo/bar/l111"},
									{Name: "l111", Source: "https://foo/bar/l111"},
									{Name: "l112", Source: "https://foo/bar/l112"},
								},
							},
						},
					},
					{
						Name: "l2",
						Nodes: []*api.Node{
							{
								Name: "l21",
							},
						},
					},
				},
			},
			want: errors.New("Node collisions detected.\nIn l1.l11 container node. Node with name l111 appears 2 times for sources: https://foo/bar/l111, https://foo/bar/l111."),
		},
	}
	for _, tt := range tests {
		recursiveSetParents(tt.args.nodes, nil)
		t.Run(tt.name, func(t *testing.T) {
			got := checkForCollisions(tt.args.nodes)
			assert.Equal(t, tt.want, got)
		})
	}
}

func recursiveSetParents(nodes []*api.Node, parent *api.Node) {
	for _, node := range nodes {
		node.SetParent(parent)
		recursiveSetParents(node.Nodes, node)
	}
}
