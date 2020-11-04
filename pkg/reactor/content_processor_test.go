// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reactor

import (
	"context"
	"strings"
	"testing"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/resourcehandlers/github"
	"github.com/gardener/docforge/pkg/util/tests"
	"github.com/stretchr/testify/assert"
)

func Test_processLink(t *testing.T) {
	nodeA := &api.Node{
		Name:   "node_A.md",
		Source: "https://github.com/gardener/gardener/blob/v1.10.0/docs/README.md",
	}
	nodeB := &api.Node{
		Name:   "node_B.md",
		Source: "https://github.com/gardener/gardener/blob/v1.10.0/docs/extensions/overview.md",
	}
	nodeA.Nodes = []*api.Node{nodeB}
	nodeA.SetParentsDownwards()

	testCases := []struct {
		name              string
		node              *api.Node
		destination       string
		contentSourcePath string
		wantDestination   *string
		wantText          *string
		wantTitle         *string
		wantDownload      *Download
		wantErr           error
		mutate            func(c *nodeContentProcessor)
	}{
		// skipped links
		{
			name:              "Internal document links are not processed",
			destination:       "#internal-link",
			contentSourcePath: "",
			wantDestination:   tests.StrPtr("#internal-link"),
			wantText:          nil,
			wantTitle:         nil,
			wantDownload:      nil,
			wantErr:           nil,
		},
		{
			name:              "mailto protocol is not processed",
			destination:       "mailto:a@b.com",
			contentSourcePath: "",
			wantDestination:   tests.StrPtr("mailto:a@b.com"),
			wantText:          nil,
			wantTitle:         nil,
			wantDownload:      nil,
			wantErr:           nil,
		},
		{
			name:              "Absolute links to releases not processed",
			destination:       "https://github.com/gardener/gardener/releases/tag/v1.4.0",
			contentSourcePath: nodeA.Source,
			wantDestination:   tests.StrPtr("https://github.com/gardener/gardener/releases/tag/v1.4.0"),
			wantText:          nil,
			wantTitle:         nil,
			wantDownload:      nil,
			wantErr:           nil,
		},
		{
			name:              "Relative links to releases not processed",
			destination:       "../../../releases/tag/v1.4.0",
			contentSourcePath: nodeA.Source,
			wantDestination:   tests.StrPtr("https://github.com/gardener/gardener/releases/tag/v1.4.0"),
			wantText:          nil,
			wantTitle:         nil,
			wantDownload:      nil,
			wantErr:           nil,
		},
		// links to resources
		{
			name:              "Relative link to resource NOT in download scope",
			destination:       "./image.png",
			contentSourcePath: "https://github.com/gardener/gardener/blob/v1.10.0/docs/README.md",
			wantDestination:   tests.StrPtr("https://github.com/gardener/gardener/blob/v1.10.0/docs/image.png"),
			wantText:          nil,
			wantTitle:         nil,
			wantDownload:      nil,
			wantErr:           nil,
		},
		{
			name: "Relative link to resource in download scope",
			node: &api.Node{
				Source: "https://github.com/gardener/gardener/blob/v1.10.0/docs/README.md",
			},
			destination:       "./image.png",
			contentSourcePath: "https://github.com/gardener/gardener/blob/v1.10.0/docs/README.md",
			wantDestination:   tests.StrPtr("/__resources"),
			wantText:          nil,
			wantTitle:         nil,
			wantDownload: &Download{
				"https://github.com/gardener/gardener/blob/v1.10.0/docs/image.png",
				"",
			},
			wantErr: nil,
			mutate: func(c *nodeContentProcessor) {
				c.globalLinksConfig = &api.Links{
					Rewrites: map[string]*api.LinkRewriteRule{
						"/gardener/gardener/": {
							Version: tests.StrPtr("v1.10.0"),
						},
					},
					Downloads: &api.Downloads{
						Scope: map[string]api.ResourceRenameRules{
							"/gardener/gardener/(blob|raw|wiki)/v1.10.0/docs": nil,
						},
					},
				}
			},
		},
		{
			name: "Relative link to resource NOT in download scope",
			node: &api.Node{
				Source: "https://github.com/gardener/gardener/blob/v1.10.0/docs/README.md",
			},
			destination:       "../image.png",
			contentSourcePath: "https://github.com/gardener/gardener/blob/v1.10.0/docs/README.md",
			wantDestination:   tests.StrPtr("https://github.com/gardener/gardener/blob/v1.10.0/image.png"),
			wantText:          nil,
			wantTitle:         nil,
			wantDownload:      nil,
			wantErr:           nil,
		},
		{
			name:              "Absolute link to resource NOT in download scope",
			node:              nodeA,
			destination:       "https://github.com/owner/repo/blob/master/docs/image.png",
			contentSourcePath: nodeA.Source,
			wantDestination:   tests.StrPtr("https://github.com/owner/repo/blob/master/docs/image.png"),
			wantText:          nil,
			wantTitle:         nil,
			wantDownload:      nil,
			wantErr:           nil,
		},
		{
			name: "Absolute link to resource in download scope",
			node: &api.Node{
				Source: "https://github.com/gardener/gardener/blob/v1.10.0/docs/README.md",
			},
			destination:       "https://github.com/gardener/gardener/blob/v1.10.0/docs/image.png",
			contentSourcePath: "https://github.com/gardener/gardener/blob/v1.10.0/docs/README.md",
			wantDestination:   tests.StrPtr("/__resources"),
			wantText:          nil,
			wantTitle:         nil,
			wantDownload: &Download{
				"https://github.com/gardener/gardener/blob/v1.10.0/docs/image.png",
				"",
			},
			wantErr: nil,
			mutate: func(c *nodeContentProcessor) {
				c.globalLinksConfig = &api.Links{
					Rewrites: map[string]*api.LinkRewriteRule{
						"/gardener/gardener/": {
							Version: tests.StrPtr("v1.10.0"),
						},
					},
					Downloads: &api.Downloads{
						Scope: map[string]api.ResourceRenameRules{
							"/gardener/gardener/(blob|raw|wiki)/v1.10.0/docs": nil,
						},
					},
				}
			},
		},
		// links to documents
		{
			name:              "Absolute link to document NOT in download scope and NOT from structure",
			node:              nodeA,
			destination:       "https://github.com/owner/repo/blob/master/docs/doc.md",
			contentSourcePath: nodeA.Source,
			wantDestination:   tests.StrPtr("https://github.com/owner/repo/blob/master/docs/doc.md"),
			wantText:          nil,
			wantTitle:         nil,
			wantDownload:      nil,
			wantErr:           nil,
		},
		{
			name:              "Absolute link to document in download scope and from structure",
			node:              nodeA,
			destination:       nodeB.Source,
			contentSourcePath: nodeA.Source,
			wantDestination:   tests.StrPtr("./node_B.md"),
			wantText:          nil,
			wantTitle:         nil,
			wantDownload:      nil,
			wantErr:           nil,
			mutate: func(c *nodeContentProcessor) {
				c.globalLinksConfig = &api.Links{
					Rewrites: map[string]*api.LinkRewriteRule{
						"/gardener/gardener/": {
							Version: tests.StrPtr("v1.10.0"),
						},
					},
					Downloads: &api.Downloads{
						Scope: map[string]api.ResourceRenameRules{
							"/gardener/gardener/(blob|raw|wiki)/v1.10.0/docs": nil,
						},
					},
				}
			},
		},
		{
			name:              "Relative link to document NOT in download scope and NOT from structure",
			node:              nodeA,
			destination:       "https://github.com/owner/repo/blob/master/docs/doc.md",
			contentSourcePath: nodeA.Source,
			wantDestination:   tests.StrPtr("https://github.com/owner/repo/blob/master/docs/doc.md"),
			wantText:          nil,
			wantTitle:         nil,
			wantDownload:      nil,
			wantErr:           nil,
		},
		{
			name:              "Relative link to document in download scope and NOT from structure",
			node:              nodeA,
			destination:       "./another.md",
			contentSourcePath: nodeA.Source,
			wantDestination:   tests.StrPtr("https://github.com/gardener/gardener/blob/v1.10.0/docs/another.md"),
			wantText:          nil,
			wantTitle:         nil,
			wantDownload:      nil,
			wantErr:           nil,
			mutate: func(c *nodeContentProcessor) {
				c.globalLinksConfig = &api.Links{
					Rewrites: map[string]*api.LinkRewriteRule{
						"/gardener/gardener/": {
							Version: tests.StrPtr("v1.10.0"),
						},
					},
					Downloads: &api.Downloads{
						Scope: map[string]api.ResourceRenameRules{
							"/gardener/gardener/(blob|raw|wiki)/v1.10.0/docs": nil,
						},
					},
				}
			},
		},
		// Version rewrite
		{
			name:              "Absolute link to document not in download scope version rewrite",
			node:              nodeA,
			destination:       "https://github.com/gardener/gardener/blob/master/sample.md",
			contentSourcePath: nodeA.Source,
			wantDestination:   tests.StrPtr("https://github.com/gardener/gardener/blob/v1.10.0/sample.md"),
			wantText:          nil,
			wantTitle:         nil,
			wantDownload:      nil,
			wantErr:           nil,
			mutate: func(c *nodeContentProcessor) {
				c.globalLinksConfig = &api.Links{
					Rewrites: map[string]*api.LinkRewriteRule{
						"/gardener/gardener/(blob)": {
							Version: tests.StrPtr("v1.10.0"),
						},
					},
				}
			},
		},
		{
			name:              "Absolute link to resource version rewrite",
			node:              nodeA,
			destination:       "https://github.com/gardener/gardener/blob/master/docs/image.png",
			contentSourcePath: nodeA.Source,
			wantDestination:   tests.StrPtr("/__resources"),
			wantText:          nil,
			wantTitle:         nil,
			wantDownload: &Download{
				"https://github.com/gardener/gardener/blob/v1.10.0/docs/image.png",
				"",
			},
			wantErr: nil,
			mutate: func(c *nodeContentProcessor) {
				c.globalLinksConfig = &api.Links{
					Rewrites: map[string]*api.LinkRewriteRule{
						"/gardener/gardener/": {
							Version: tests.StrPtr("v1.10.0"),
						},
					},
					Downloads: &api.Downloads{
						Scope: map[string]api.ResourceRenameRules{
							"/gardener/gardener/(blob|raw|wiki)/v1.10.0/docs": nil,
						},
					},
				}
			},
		},
		{
			name: "Relative link to resource in download scope with rewrites",
			node: &api.Node{
				Source: "https://github.com/gardener/gardener/blob/v1.10.0/docs/README.md",
				Links: &api.Links{
					Rewrites: map[string]*api.LinkRewriteRule{
						"https://github.com/gardener/gardener/blob/v1.10.0/docs/image.png": {
							Text:  tests.StrPtr("Test text"),
							Title: tests.StrPtr("Test title"),
						},
					},
				},
			},
			destination:       "./image.png",
			contentSourcePath: "https://github.com/gardener/gardener/blob/v1.10.0/docs/README.md",
			wantDestination:   tests.StrPtr("/__resources"),
			wantText:          tests.StrPtr("Test text"),
			wantTitle:         tests.StrPtr("Test title"),
			wantDownload: &Download{
				"https://github.com/gardener/gardener/blob/v1.10.0/docs/image.png",
				"",
			},
			wantErr: nil,
			mutate: func(c *nodeContentProcessor) {
				c.globalLinksConfig = &api.Links{
					Rewrites: map[string]*api.LinkRewriteRule{
						"/gardener/gardener/": {
							Version: tests.StrPtr("v1.10.0"),
						},
					},
					Downloads: &api.Downloads{
						Scope: map[string]api.ResourceRenameRules{
							"/gardener/gardener/(blob|raw|wiki)/v1.10.0/docs": nil,
						},
					},
				}
			},
		},
	}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			c := &nodeContentProcessor{
				resourceAbsLinks: make(map[string]string),
				resourcesRoot:    "/__resources",
				resourceHandlers: resourcehandlers.NewRegistry(github.NewResourceHandler(nil, []string{"github.com"})),
				rewriteEmbedded:  true,
			}
			if tc.mutate != nil {
				tc.mutate(c)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			gotDestination, gotText, gotTitle, gotDownload, gotErr := c.resolveLink(ctx, tc.node, tc.destination, tc.contentSourcePath)

			assert.Equal(t, tc.wantErr, gotErr)
			if gotDownload != nil {
				if tc.wantDownload != nil {
					tc.wantDownload.resourceName = gotDownload.resourceName
				}
			}
			assert.Equal(t, tc.wantDownload, gotDownload)
			var destination, text, title string
			if gotDestination != nil {
				destination = *gotDestination
			}
			if tc.wantDestination != nil {
				if !strings.HasPrefix(destination, *tc.wantDestination) {
					t.Errorf("expected destination starting with %s, was %s", *tc.wantDestination, destination)
					return
				}
			} else {
				assert.Equal(t, tc.wantDestination, gotDestination)
			}
			if gotText != nil {
				text = *gotText
			}
			if tc.wantText != nil {
				assert.Equal(t, *tc.wantText, text)
			} else {
				assert.Equal(t, tc.wantText, gotText)
			}
			if gotTitle != nil {
				title = *gotTitle
			}
			if tc.wantText != nil {
				assert.Equal(t, *tc.wantTitle, title)
			} else {
				assert.Equal(t, tc.wantTitle, gotTitle)
			}
		})
	}
}

func Test_matchHTMLLinks(t *testing.T) {
	testCases := []struct {
		in   []string
		want []string
	}{
		{
			in: []string{
				`<script src="abc/a.js" />`,
				`<script src=   "abc/a.js" />`,
				`<script src="   abc/a.js" />`,
				`<script SRC = "   abc/a.js" />`,
				`<script SRC = '   abc/a.js' />`,
				`<script SRC = '   abc/a.js  ' title="test" />`,
				`<script src= abc/a.js />`,
				`<script SRC = "   abc/a.js" `,
				`<img src="abc/a.js">`,
				`<a href="abc/a.js">A</a>`,
			},
			want: []string{
				`<script src="cde" />`,
				`<script src=   "cde" />`,
				`<script src="   cde" />`,
				`<script SRC = "   cde" />`,
				`<script SRC = '   cde' />`,
				`<script SRC = '   cde  ' title="test" />`,
				`<script src= cde />`,
				`<script SRC = "   cde" `,
				`<img src="cde">`,
				`<a href="cde">A</a>`,
			},
		},
		{
			in: []string{
				`< src="abc/a.js" />`,
				`<script src="   abc/a.js' />`,
				// FIXME: unbalanced quotation marks break the regex
				// `<script SRC = "   abc/a.js   title="test" />`,
			},
			want: []string{
				`< src="abc/a.js" />`,
				`<script src="   abc/a.js' />`,
				// `<script SRC = "   abc/a.js   title="test" />`,
			},
		},
	}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			c := &nodeContentProcessor{
				resourceAbsLinks: make(map[string]string),
				resourcesRoot:    "/__resources",
				resourceHandlers: resourcehandlers.NewRegistry(&testResourceHandler{}),
				rewriteEmbedded:  true,
				globalLinksConfig: &api.Links{
					Rewrites: map[string]*api.LinkRewriteRule{
						"abc": &api.LinkRewriteRule{
							Destination: tests.StrPtr("cde"),
						},
					},
				},
			}
			node := &api.Node{
				Name:   "node_A.md",
				Source: "https://github.com/gardener/gardener/blob/v1.10.0/docs/README.md",
			}
			var (
				b   []byte
				err error
			)
			for i, in := range tc.in {
				if b, err = c.reconcileHTMLLinks(nil, node, []byte(in), ""); err != nil {
					t.Fatal(err)
				}
				assert.Equal(t, tc.want[i], string(b))
			}
		})
	}
}

type testResourceHandler struct {
	hitCounter int
}

func (rh *testResourceHandler) Accept(uri string) bool {
	return true
}

func (rh *testResourceHandler) ResolveNodeSelector(ctx context.Context, node *api.Node, excludePaths []string, frontMatter map[string]interface{}, excludeFrontMatter map[string]interface{}, depth int32) ([]*api.Node, error) {
	return nil, nil
}

func (rh *testResourceHandler) ResolveDocumentation(ctx context.Context, uri string) (*api.Documentation, error) {
	return nil, nil
}

func (rh *testResourceHandler) Read(ctx context.Context, uri string) ([]byte, error) {
	return nil, nil
}
func (rh *testResourceHandler) ReadGitInfo(ctx context.Context, uri string) ([]byte, error) {
	return nil, nil
}
func (rh *testResourceHandler) Name(uri string) string {
	return ""
}
func (rh *testResourceHandler) ResourceName(uri string) (string, string) {
	return "", ""
}
func (rh *testResourceHandler) BuildAbsLink(source, relLink string) (string, error) {
	return relLink, nil
}
func (rh *testResourceHandler) SetVersion(link, version string) (string, error) {
	return link, nil
}

func (rh *testResourceHandler) GetRawFormatLink(absLink string) (string, error) {
	return absLink, nil
}
