package reactor

import (
	"context"
	"strings"
	"testing"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/resourcehandlers/github"
	"github.com/stretchr/testify/assert"
)

func Test_processLink(t *testing.T) {
	nodeA := &api.Node{
		Name: "node_A.md",
		ContentSelectors: []api.ContentSelector{
			api.ContentSelector{
				Source: "https://github.com/gardener/gardener/blob/v1.10.0/docs/README.md",
			},
		},
	}
	nodeB := &api.Node{
		Name: "node_B.md",
		ContentSelectors: []api.ContentSelector{
			api.ContentSelector{
				Source: "https://github.com/gardener/gardener/blob/v1.10.0/docs/extensions/overview.md",
			},
		},
	}
	nodeA.Nodes = []*api.Node{nodeB}
	nodeA.SetParentsDownwards()

	tests := []struct {
		name              string
		node              *api.Node
		destination       string
		contentSourcePath string
		wantDestination   string
		wantDownloadURL   string
		wantResourceName  string
		wantErr           error
		mutate            func(c *NodeContentProcessor)
	}{
		// skipped links
		{
			name:              "Internal document links are not processed",
			destination:       "#internal-link",
			contentSourcePath: "",
			wantDestination:   "#internal-link",
			wantDownloadURL:   "",
			wantResourceName:  "",
			wantErr:           nil,
		},
		{
			name:              "mailto protocol is not processed",
			destination:       "mailto:a@b.com",
			contentSourcePath: "",
			wantDestination:   "mailto:a@b.com",
			wantDownloadURL:   "",
			wantResourceName:  "",
			wantErr:           nil,
		},
		{
			name:              "Absolute links to releases not processed",
			destination:       "https://github.com/gardener/gardener/releases/tag/v1.4.0",
			contentSourcePath: nodeA.ContentSelectors[0].Source,
			wantDestination:   "https://github.com/gardener/gardener/releases/tag/v1.4.0",
			wantDownloadURL:   "",
			wantResourceName:  "",
			wantErr:           nil,
		},
		{
			name:              "Relative links to releases not processed",
			destination:       "../../../releases/tag/v1.4.0",
			contentSourcePath: nodeA.ContentSelectors[0].Source,
			wantDestination:   "https://github.com/gardener/gardener/releases/tag/v1.4.0",
			wantDownloadURL:   "",
			wantResourceName:  "",
			wantErr:           nil,
		},
		// links to resources
		{
			name:              "Relative link to resource NOT in locality domain",
			destination:       "./image.png",
			contentSourcePath: "https://github.com/gardener/gardener/blob/v1.10.0/docs/README.md",
			wantDestination:   "https://github.com/gardener/gardener/blob/v1.10.0/docs/image.png",
			wantDownloadURL:   "",
			wantResourceName:  "",
			wantErr:           nil,
		},
		{
			name: "Relative link to resource in locality domain",
			node: &api.Node{
				ContentSelectors: []api.ContentSelector{
					api.ContentSelector{
						Source: "https://github.com/gardener/gardener/blob/v1.10.0/docs/README.md",
					},
				},
			},
			destination:       "./image.png",
			contentSourcePath: "https://github.com/gardener/gardener/blob/v1.10.0/docs/README.md",
			wantDestination:   "/__resources",
			wantDownloadURL:   "https://github.com/gardener/gardener/blob/v1.10.0/docs/image.png",
			wantResourceName:  "",
			wantErr:           nil,
			mutate: func(c *NodeContentProcessor) {
				c.localityDomain = &localityDomain{
					mapping: map[string]*localityDomainValue{
						"github.com/gardener/gardener": &localityDomainValue{
							"v1.10.0",
							"gardener/gardener/docs",
							nil,
							nil,
						},
					},
				}
			},
		},
		{
			name: "Relative link to resource NOT in locality domain",
			node: &api.Node{
				ContentSelectors: []api.ContentSelector{
					api.ContentSelector{
						Source: "https://github.com/gardener/gardener/blob/v1.10.0/docs/README.md",
					},
				},
			},
			destination:       "../image.png",
			contentSourcePath: "https://github.com/gardener/gardener/blob/v1.10.0/docs/README.md",
			wantDestination:   "https://github.com/gardener/gardener/blob/v1.10.0/image.png",
			wantDownloadURL:   "",
			wantResourceName:  "",
			wantErr:           nil,
		},
		{
			name:              "Absolute link to resource NOT in locality domain",
			node:              nodeA,
			destination:       "https://github.com/owner/repo/blob/master/docs/image.png",
			contentSourcePath: nodeA.ContentSelectors[0].Source,
			wantDestination:   "https://github.com/owner/repo/blob/master/docs/image.png",
			wantDownloadURL:   "",
			wantResourceName:  "",
			wantErr:           nil,
		},
		{
			name: "Absolute link to resource in locality domain",
			node: &api.Node{
				ContentSelectors: []api.ContentSelector{
					api.ContentSelector{
						Source: "https://github.com/gardener/gardener/blob/v1.10.0/docs/README.md",
					},
				},
			},
			destination:       "https://github.com/gardener/gardener/blob/v1.10.0/docs/image.png",
			contentSourcePath: "https://github.com/gardener/gardener/blob/v1.10.0/docs/README.md",
			wantDestination:   "/__resources",
			wantDownloadURL:   "https://github.com/gardener/gardener/blob/v1.10.0/docs/image.png",
			wantResourceName:  "",
			wantErr:           nil,
			mutate: func(c *NodeContentProcessor) {
				c.localityDomain = &localityDomain{
					mapping: map[string]*localityDomainValue{
						"github.com/gardener/gardener": &localityDomainValue{
							"v1.10.0",
							"gardener/gardener/docs",
							nil,
							nil,
						},
					},
				}
			},
		},
		// links to documents
		{
			name:              "Absolute link to document NOT in locality domain and NOT from structure",
			node:              nodeA,
			destination:       "https://github.com/owner/repo/blob/master/docs/doc.md",
			contentSourcePath: nodeA.ContentSelectors[0].Source,
			wantDestination:   "https://github.com/owner/repo/blob/master/docs/doc.md",
			wantDownloadURL:   "",
			wantResourceName:  "",
			wantErr:           nil,
		},
		{
			name:              "Absolute link to document in locality domain and from structure",
			node:              nodeA,
			destination:       nodeB.ContentSelectors[0].Source,
			contentSourcePath: nodeA.ContentSelectors[0].Source,
			wantDestination:   "./node_B.md",
			wantDownloadURL:   "",
			wantResourceName:  "",
			wantErr:           nil,
			mutate: func(c *NodeContentProcessor) {
				c.localityDomain = &localityDomain{
					mapping: map[string]*localityDomainValue{
						"github.com/gardener/gardener": &localityDomainValue{
							"v1.10.0",
							"gardener/gardener/docs",
							nil,
							nil,
						},
					},
				}
			},
		},
		{
			name:              "Relative link to document NOT in locality domain and NOT from structure",
			node:              nodeA,
			destination:       "https://github.com/owner/repo/blob/master/docs/doc.md",
			contentSourcePath: nodeA.ContentSelectors[0].Source,
			wantDestination:   "https://github.com/owner/repo/blob/master/docs/doc.md",
			wantDownloadURL:   "",
			wantResourceName:  "",
			wantErr:           nil,
		},
		{
			name:              "Relative link to document in locality domain and NOT from structure",
			node:              nodeA,
			destination:       "./another.md",
			contentSourcePath: nodeA.ContentSelectors[0].Source,
			wantDestination:   "https://github.com/gardener/gardener/blob/v1.10.0/docs/another.md",
			wantDownloadURL:   "",
			wantResourceName:  "",
			wantErr:           nil,
			mutate: func(c *NodeContentProcessor) {
				c.localityDomain = &localityDomain{
					mapping: map[string]*localityDomainValue{
						"github.com/gardener/gardener": &localityDomainValue{
							"v1.10.0",
							"gardener/gardener/docs",
							nil,
							nil,
						},
					},
				}
			},
		},
		// Version rewrite
		{
			name:              "Absolute link to document not in locality domain version rewrite",
			node:              nodeA,
			destination:       "https://github.com/gardener/gardener/blob/master/sample.md",
			contentSourcePath: nodeA.ContentSelectors[0].Source,
			wantDestination:   "https://github.com/gardener/gardener/blob/v1.10.0/sample.md",
			wantDownloadURL:   "",
			wantResourceName:  "",
			wantErr:           nil,
			mutate: func(c *NodeContentProcessor) {
				c.localityDomain = &localityDomain{
					mapping: map[string]*localityDomainValue{
						"github.com/gardener/gardener": &localityDomainValue{
							"v1.10.0",
							"gardener/gardener/docs",
							nil,
							nil,
						},
					},
				}
			},
		},
		{
			name:              "Absolute link to resource version rewrite",
			node:              nodeA,
			destination:       "https://github.com/gardener/gardener/blob/master/docs/image.png",
			contentSourcePath: nodeA.ContentSelectors[0].Source,
			wantDestination:   "/__resources",
			wantDownloadURL:   "https://github.com/gardener/gardener/blob/v1.10.0/docs/image.png",
			wantResourceName:  "",
			wantErr:           nil,
			mutate: func(c *NodeContentProcessor) {
				c.localityDomain = &localityDomain{
					mapping: map[string]*localityDomainValue{
						"github.com/gardener/gardener": &localityDomainValue{
							"v1.10.0",
							"gardener/gardener/docs",
							nil,
							nil,
						},
					},
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &NodeContentProcessor{
				resourceAbsLinks: make(map[string]string),
				localityDomain: &localityDomain{
					mapping: map[string]*localityDomainValue{},
				},
				resourcesRoot:    "/__resources",
				ResourceHandlers: resourcehandlers.NewRegistry(github.NewResourceHandler(nil, []string{"github.com"})),
			}
			if tt.mutate != nil {
				tt.mutate(c)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			gotDestination, _, _, gotDownload, gotErr := c.resolveLink(ctx, tt.node, tt.destination, tt.contentSourcePath)

			if gotErr != tt.wantErr {
				t.Errorf("expected err %s != %s", gotErr, tt.wantErr)
			}
			if len(tt.wantDownloadURL) > 0 {
				if len(tt.wantDestination) == 0 && gotDestination != tt.wantDestination {
					t.Errorf("expected destination %s != %s", tt.wantDestination, gotDestination)
				} else if !strings.HasPrefix(gotDestination, tt.wantDestination) {
					t.Errorf("expected destination starting with %s, was %s", tt.wantDestination, gotDestination)
				}
				if gotDownload.url != tt.wantDownloadURL {
					t.Errorf("expected downloadURL %s != %s", tt.wantDownloadURL, gotDownload.url)
				}
				if len(gotDownload.resourceName) == 0 {
					t.Error("expected resource name != \"\"\n", gotDownload.resourceName)
				}
			} else {
				if gotDestination != tt.wantDestination {
					t.Errorf("expected destination %s != %s", tt.wantDestination, gotDestination)
				}
				if gotDownload.resourceName != tt.wantDownloadURL {
					t.Errorf("expected downloadURL %s != %s", tt.wantDownloadURL, gotDownload.url)
				}
				if gotDownload.resourceName != tt.wantResourceName {
					t.Errorf("expected resourceName %s != %s", tt.wantResourceName, gotDownload.resourceName)
				}
			}
		})
	}
}

func Test_Substitute(t *testing.T) {
	cda := "cda"
	testCases := []struct {
		link            string
		substitutes     map[string]*api.LinkSubstitute
		wantDestination string
		wantOK          bool
		wantText        *string
		wantTitle       *string
	}{
		{
			"abc",
			map[string]*api.LinkSubstitute{
				"abc": &api.LinkSubstitute{
					Destination: &cda,
				},
			},
			"cda",
			true,
			&cda,
			&cda,
		},
		{
			"abc",
			map[string]*api.LinkSubstitute{},
			"abc",
			true,
			&cda,
			&cda,
		},
		{
			"",
			map[string]*api.LinkSubstitute{
				"abc": &api.LinkSubstitute{
					Destination: &cda,
				},
			},
			"",
			true,
			nil,
			nil,
		},
	}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			n := &api.Node{
				LinksSubstitutes: tc.substitutes,
			}
			var (
				gotOK                             bool
				gotDestination, gotText, gotTitle *string
			)
			gotOK, gotDestination, gotText, gotTitle = substitute(tc.link, n)
			assert.Equal(t, tc.wantOK, gotOK)
			assert.Equal(t, tc.wantDestination, gotDestination)
			assert.Equal(t, tc.wantText, gotText)
			assert.Equal(t, tc.wantTitle, gotTitle)
		})
	}
}
