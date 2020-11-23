// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reactor

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/resourcehandlers/testhandler"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/pointer"
)

const testPath string = "test-nodesSelector-path"
const testPath2 string = "test-nodesSelector-path-2"

var defaultCtxWithTimeout, _ = context.WithTimeout(context.TODO(), 5*time.Second)
var testNodeSelector = api.NodeSelector{Path: testPath}
var testNodeSelector2 = api.NodeSelector{Path: testPath2}
var testNode = api.Node{Name: "testNodeName", Source: "testNodeSource"}
var testNode2 = api.Node{Name: "testNodeName2", Source: "testNodeSource2"}

func TestResolveManifest(t *testing.T) {
	type args struct {
		ctx                     context.Context
		resolveNodeSelectorFunc func(ctx context.Context, node *api.Node, excludePaths []string,
			frontMatter map[string]interface{}, excludeFrontMatter map[string]interface{}, depth int32) ([]*api.Node, error)
		manifestPath      string
		testDocumentation *api.Documentation
	}
	tests := []struct {
		name                  string
		description           string
		args                  args
		wantErr               bool
		expectedDocumentation *api.Documentation
	}{
		{
			name:        "returns_err_when_missing_nodeSelector_and_structure",
			description: "has error when there are no nodes after NodeSelector resolving and there are no nodes defined in Documentation.Structure",
			args: args{
				ctx: defaultCtxWithTimeout,
				resolveNodeSelectorFunc: func(ctx context.Context, node *api.Node, excludePaths []string,
					frontMatter map[string]interface{}, excludeFrontMatter map[string]interface{}, depth int32) ([]*api.Node, error) {
					return []*api.Node{}, nil
				},
				testDocumentation: &api.Documentation{
					NodeSelector: &testNodeSelector,
				},
			},
			wantErr: true,
		},
		{
			name:        "succeeds_to_append_resolved_nodeSelector_nodes_to_structure",
			description: "should resolve documentation nodeSelector on root level and append nodes to Documentation.Structure",
			args: args{
				ctx: defaultCtxWithTimeout,
				resolveNodeSelectorFunc: func(ctx context.Context, node *api.Node, excludePaths []string,
					frontMatter map[string]interface{}, excludeFrontMatter map[string]interface{}, depth int32) ([]*api.Node, error) {
					return []*api.Node{&testNode}, nil
				},
				testDocumentation: &api.Documentation{
					NodeSelector: &testNodeSelector,
				},
			},
			wantErr: false,
			expectedDocumentation: &api.Documentation{
				Structure: []*api.Node{&testNode},
			},
		},
		{
			name:        "succeeds_to_resolve_manifest",
			description: "should resolve manifest and add the resolved nodesSelector nodes to existing structure",
			args: args{
				ctx: defaultCtxWithTimeout,
				resolveNodeSelectorFunc: func(ctx context.Context, node *api.Node, excludePaths []string,
					frontMatter map[string]interface{}, excludeFrontMatter map[string]interface{}, depth int32) ([]*api.Node, error) {
					return []*api.Node{&testNode}, nil
				},
				testDocumentation: &api.Documentation{
					NodeSelector: &testNodeSelector,
					Structure:    []*api.Node{{Name: "existing"}},
				},
			},
			wantErr: false,
			expectedDocumentation: &api.Documentation{
				Structure: []*api.Node{
					{
						Name: "existing",
					},
					&testNode,
				},
			},
		},
		{
			name:        "resolves_child_node_nodeSelector",
			description: "should resolve Node.NodeSelector nodes and append to the Node",
			args: args{
				ctx: defaultCtxWithTimeout,
				resolveNodeSelectorFunc: func(ctx context.Context, node *api.Node, excludePaths []string,
					frontMatter map[string]interface{}, excludeFrontMatter map[string]interface{}, depth int32) ([]*api.Node, error) {
					return []*api.Node{&testNode}, nil
				},
				testDocumentation: &api.Documentation{
					Structure: []*api.Node{
						{
							Name:         "testNodeStructure",
							NodeSelector: &testNodeSelector,
						},
					},
				},
			},
			wantErr: false,
			expectedDocumentation: &api.Documentation{
				Structure: []*api.Node{
					{
						Name: "testNodeStructure",
						Nodes: []*api.Node{
							&testNode,
						},
					},
				},
			},
		},
		{
			// TODO: this test case demonstrates design flaw where we generate anonmyous node
			name:        "resolve_module_on_root",
			description: "resolve module specified on root level and append to structure",
			args: args{
				ctx: defaultCtxWithTimeout,
				resolveNodeSelectorFunc: func(ctx context.Context, node *api.Node, excludePaths []string,
					frontMatter map[string]interface{}, excludeFrontMatter map[string]interface{}, depth int32) ([]*api.Node, error) {
					if node.NodeSelector.Path == testNodeSelector.Path {
						return []*api.Node{{NodeSelector: &api.NodeSelector{Path: "module.yaml"}}}, nil
					}
					return []*api.Node{{Name: "moduleNode"}}, nil
				},
				testDocumentation: &api.Documentation{
					NodeSelector: &testNodeSelector,
					Structure: []*api.Node{
						&testNode,
					},
				},
			},
			wantErr: false,
			expectedDocumentation: &api.Documentation{
				Structure: []*api.Node{
					&testNode,
					{
						Nodes: []*api.Node{{Name: "moduleNode"}},
					},
				},
			},
		},
		{
			name:        "break_recursive_module",
			description: "breaks recursive import of modules for example if the documentation imports A that imports B it should stop resolvin if B imports A",
			args: args{
				ctx: defaultCtxWithTimeout,
				resolveNodeSelectorFunc: func(ctx context.Context, node *api.Node, excludePaths []string,
					frontMatter map[string]interface{}, excludeFrontMatter map[string]interface{}, depth int32) ([]*api.Node, error) {
					if node.NodeSelector.Path == testNodeSelector.Path {
						return []*api.Node{{NodeSelector: &api.NodeSelector{Path: "moduleA.yaml"}}}, nil
					}
					if node.NodeSelector.Path == "moduleA.yaml" {
						return []*api.Node{{NodeSelector: &testNodeSelector}}, nil
					}
					return []*api.Node{{Name: "resolvedNestedNode"}}, nil
				},
				testDocumentation: &api.Documentation{NodeSelector: &testNodeSelector},
			},
			wantErr: false,
			// TODO: circular dependency should clear empty nodes
			expectedDocumentation: &api.Documentation{
				Structure: []*api.Node{
					{
						Nodes: []*api.Node{
							{
								Nodes: []*api.Node{
									{},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "succeeds_to_break_recursive_module",
			args: args{
				ctx: defaultCtxWithTimeout,
				resolveNodeSelectorFunc: func(ctx context.Context, node *api.Node, excludePaths []string,
					frontMatter map[string]interface{}, excludeFrontMatter map[string]interface{}, depth int32) ([]*api.Node, error) {
					if node.NodeSelector.Path == testNodeSelector.Path {
						return []*api.Node{{NodeSelector: &api.NodeSelector{Path: "moduleA.yaml"}}}, nil
					}
					if node.NodeSelector.Path == "moduleA.yaml" {
						return []*api.Node{{NodeSelector: &testNodeSelector}}, nil
					}
					return []*api.Node{{Name: "resolvedNestedNode"}}, nil
				},
				testDocumentation: &api.Documentation{
					Structure: []*api.Node{
						&testNode,
						{
							Name:         "moduleA",
							NodeSelector: &testNodeSelector,
						},
					},
				},
			},
			wantErr: false,
			// TODO: circular dependency should clear empty nodes
			expectedDocumentation: &api.Documentation{
				Structure: []*api.Node{
					&testNode,
					{
						Name: "moduleA",
						Nodes: []*api.Node{
							{
								Nodes: []*api.Node{
									{},
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectedDocumentation != nil {
				for _, node := range tt.expectedDocumentation.Structure {
					node.SetParentsDownwards()
				}
			}
			rh := new(testhandler.TestResourceHandler).WithResolveNodeSelector(tt.args.resolveNodeSelectorFunc)
			resourcehandlers.NewRegistry(rh)
			if err := ResolveManifest(tt.args.ctx, tt.args.testDocumentation, resourcehandlers.NewRegistry(rh), tt.args.manifestPath); (err != nil) != tt.wantErr {
				t.Errorf("ResolveManifest() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			assert.Equal(t, tt.expectedDocumentation, tt.args.testDocumentation)
		})
	}
}

func Test_resolveNodeSelector(t *testing.T) {
	type args struct {
		ctx               context.Context
		rhRegistry        resourcehandlers.Registry
		node              *api.Node
		visited           map[string]bool
		globalLinksConfig *api.Links
	}
	tests := []struct {
		name                     string
		description              string
		args                     args
		acceptFunc               func(uri string) bool
		resolveDocumentationFunc func(ctx context.Context, uri string) (*api.Documentation, error)
		resolveNodeSelectorFunc  func(ctx context.Context, node *api.Node, excludePaths []string,
			frontMatter map[string]interface{}, excludeFrontMatter map[string]interface{}, depth int32) ([]*api.Node, error)
		want    *api.Node
		wantErr bool
	}{
		{
			name:        "missing_resource_handler",
			description: "not suitable resource handler for path returns error",
			args: args{
				node: &api.Node{
					NodeSelector: &testNodeSelector,
				},
				visited:           make(map[string]bool),
				globalLinksConfig: &api.Links{},
			},
			acceptFunc: func(uri string) bool {
				return !(uri == testNodeSelector.Path)
			},

			want:    nil,
			wantErr: true,
		},
		{
			name:        "handler_resolve_documnetation_fails",
			description: "the error when handler fails to resolve Documentation from NodeSelector`s path is propageted to the clien function",
			args: args{
				node: &api.Node{
					NodeSelector: &testNodeSelector,
				},
				visited:           make(map[string]bool),
				globalLinksConfig: &api.Links{},
			},
			resolveDocumentationFunc: func(ctx context.Context, uri string) (*api.Documentation, error) {
				return nil, fmt.Errorf("error that should be thrown for this test case")
			},
			want:    nil,
			wantErr: true,
		},
		{
			name:        "succeeds_to_resolve_structure_with_nodeSelector_on_root",
			description: "the test successfully resolves the api.Documentation.NodeSelector which path refers to another documentation structure",
			args: args{
				node: &api.Node{
					NodeSelector: &testNodeSelector,
				},
				visited:           make(map[string]bool),
				globalLinksConfig: &api.Links{},
			},
			resolveDocumentationFunc: func(ctx context.Context, uri string) (*api.Documentation, error) {
				module := &api.Documentation{
					Structure: []*api.Node{
						&testNode,
					},
				}
				return module, nil
			},
			want: &api.Node{
				Nodes: []*api.Node{
					&testNode,
				},
			},
			wantErr: false,
		},
		{
			name:        "succeeds_to_resolve_with_nodeSelector_and_strucutre_on_root",
			description: "returns a node that combines nodes returned from the nodeSelector and refers to another documentation structure with nodeSelector and Structure nodes",
			args: args{
				node: &api.Node{
					NodeSelector: &testNodeSelector,
				},
				visited:           make(map[string]bool),
				globalLinksConfig: &api.Links{},
			},
			resolveNodeSelectorFunc: func(ctx context.Context, node *api.Node, excludePaths []string, frontMatter, excludeFrontMatter map[string]interface{}, depth int32) ([]*api.Node, error) {
				if node.NodeSelector.Path == testNodeSelector2.Path {
					return []*api.Node{&testNode2}, nil
				}
				return nil, nil
			},
			resolveDocumentationFunc: func(ctx context.Context, uri string) (*api.Documentation, error) {
				if uri == testPath {
					module := &api.Documentation{
						NodeSelector: &testNodeSelector2,
						Structure: []*api.Node{
							&testNode,
						},
					}
					return module, nil
				}
				return nil, nil
			},
			want: &api.Node{
				Nodes: []*api.Node{
					&testNode,
					&testNode2,
				},
			},
			wantErr: false,
		},
		{
			name:        "succeeds_to_merge_links_that_not_override_globalLinks",
			description: "only return rewrite links that doesn't override the global Download links",
			args: args{
				node: &api.Node{
					NodeSelector: &testNodeSelector,
				},
				visited: make(map[string]bool),
				globalLinksConfig: &api.Links{Rewrites: map[string]*api.LinkRewriteRule{
					"example.com/sources": {
						Destination: pointer.StringPtr("A"),
					},
				}},
			},
			resolveDocumentationFunc: func(ctx context.Context, uri string) (*api.Documentation, error) {
				if uri == testPath {
					module := &api.Documentation{
						Structure: []*api.Node{
							&testNode,
						},
						Links: &api.Links{
							Rewrites: map[string]*api.LinkRewriteRule{
								"example.com/sources": {
									Destination: pointer.StringPtr("shouldn't override A"),
								},
								"example.com/blogs": {
									Destination: pointer.StringPtr("should be added"),
								},
							},
						},
					}
					return module, nil
				}
				return nil, nil
			},
			want: &api.Node{
				Nodes: []*api.Node{
					&testNode,
				},
				Links: &api.Links{
					Rewrites: map[string]*api.LinkRewriteRule{
						"example.com/blogs": {
							Destination: pointer.StringPtr("should be added"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:        "succeeds_to_merge_links_that_not_override_parent_links",
			description: "only return rewrite links that doesn't override the parent or the global rewrite links ",
			args: args{
				node: &api.Node{
					NodeSelector: &testNodeSelector,
					Links: &api.Links{Rewrites: map[string]*api.LinkRewriteRule{
						"example.com/sources": {
							Destination: pointer.StringPtr("parent node sources link rewrite"),
						},
						"example.com/blogs": {
							Destination: pointer.StringPtr("parent node blogs link rewrite"),
						},
					},
					},
				},
				visited: make(map[string]bool),
				globalLinksConfig: &api.Links{Rewrites: map[string]*api.LinkRewriteRule{
					"example.com/sources": {
						Destination: pointer.StringPtr("sources global destination rewrite"),
					},
					"example.com/news": {
						Destination: pointer.StringPtr("news global destination rewrite"),
					},
				}},
			},
			resolveDocumentationFunc: func(ctx context.Context, uri string) (*api.Documentation, error) {
				if uri == testPath {
					module := &api.Documentation{
						Structure: []*api.Node{
							&testNode,
						},
						Links: &api.Links{
							Rewrites: map[string]*api.LinkRewriteRule{
								"example.com/sources": {
									Destination: pointer.StringPtr("sources module destination rewrite"),
								},
								"example.com/fish": {
									Destination: pointer.StringPtr("fish module destination rewrite"),
								},
								"example.com/news": {
									Destination: pointer.StringPtr("news module destination rewrite"),
								},
								"example.com/blogs": {
									Destination: pointer.StringPtr("blogs module destination rewrite"),
								},
							},
						},
					}
					return module, nil
				}
				return nil, nil
			},
			want: &api.Node{
				Nodes: []*api.Node{
					&testNode,
				},
				Links: &api.Links{
					Rewrites: map[string]*api.LinkRewriteRule{
						"example.com/fish": {
							Destination: pointer.StringPtr("fish module destination rewrite"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:        "succeeds_to_merge_download_links_that_not_override_parent_links",
			description: "only return download links that doesn't override the parent or the global Download links",
			args: args{
				node: &api.Node{
					NodeSelector: &testNodeSelector,
					Links: &api.Links{
						Downloads: &api.Downloads{
							Renames: api.ResourceRenameRules{
								"example.com/images/image.png":           "parent node download renames that exist in module",
								"example.com/images/overridesglobal.png": "parent node download renames that overridesglobal and exist in module",
							},
							Scope: map[string]api.ResourceRenameRules{
								"example.com/own/repo": {
									"images/": "",
								},
							},
						},
					},
				},
				visited: make(map[string]bool),
				globalLinksConfig: &api.Links{
					Downloads: &api.Downloads{
						Renames: api.ResourceRenameRules{
							"example.com/images/overridesglobal.png":  "global download renames that overridesglobal and exist in module",
							"example.com/images/notexistonparent.png": "global download rename that doesn't exist on parent node",
						},
					},
				},
			},
			resolveDocumentationFunc: func(ctx context.Context, uri string) (*api.Documentation, error) {
				if uri == testPath {
					module := &api.Documentation{
						Structure: []*api.Node{
							&testNode,
						},
						Links: &api.Links{
							Downloads: &api.Downloads{
								Renames: api.ResourceRenameRules{
									"example.com/images/image.png":            "parent node download renames that exist in module",
									"example.com/images/overridesglobal.png":  "parent node download renames that overridesglobal and exist in module",
									"example.com/images/notexistonparent.png": "global download rename that doesn't exist on parent node",
									"example.com/images/frommodule.png":       "renames from module",
								},
								Scope: map[string]api.ResourceRenameRules{
									"example.com/own/repo": {
										"images/": "images/2.0",
									},
									"example.com/notown/notrepo": {
										"images/": "images/2.0",
									},
								},
							},
						},
					}
					return module, nil
				}
				return nil, nil
			},
			want: &api.Node{
				Nodes: []*api.Node{
					&testNode,
				},
				Links: &api.Links{
					Downloads: &api.Downloads{
						Renames: api.ResourceRenameRules{

							"example.com/images/frommodule.png": "renames from module",
						},
						Scope: map[string]api.ResourceRenameRules{
							"example.com/notown/notrepo": {
								"images/": "images/2.0",
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.want != nil {
				for _, node := range tt.want.Nodes {
					node.SetParentsDownwards()
				}
			}
			rh := new(testhandler.TestResourceHandler).WithAccept(tt.acceptFunc).WithResolveDocumentation(tt.resolveDocumentationFunc).WithResolveNodeSelector(tt.resolveNodeSelectorFunc)
			rhRegistry := resourcehandlers.NewRegistry(rh)
			got, err := resolveNodeSelector(defaultCtxWithTimeout, rhRegistry, tt.args.node, tt.args.visited, tt.args.globalLinksConfig)
			if (err != nil) != tt.wantErr {
				t.Errorf("resolveNodeSelector() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !assert.Equal(t, tt.want, got) {
				t.Errorf("resolveNodeSelector() = %v, want %v", got, tt.want)
			}
		})
	}
}
