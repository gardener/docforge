// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reactor

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	githubHandler "github.com/gardener/docforge/pkg/resourcehandlers/github"
	"github.com/gardener/docforge/pkg/resourcehandlers/resourcehandlersfakes"
	"github.com/gardener/docforge/pkg/util/tests"
	"github.com/gardener/docforge/pkg/util/urls"
	"github.com/google/go-github/v32/github"
	"github.com/stretchr/testify/assert"
)

// TODO: This is a flaky test. In the future the ResourceHandler should be mocked.
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
		wantErr           error
		mutate            func(c *nodeContentProcessor)
		embeddable        bool
		sourceLocations   map[string][]*api.Node
	}{
		// skipped links
		{
			name:              "Internal document links are not processed",
			destination:       "#internal-link",
			contentSourcePath: "",
			wantDestination:   tests.StrPtr("#internal-link"),
			wantErr:           nil,
		},
		{
			name:              "mailto protocol is not processed",
			destination:       "mailto:a@b.com",
			contentSourcePath: "",
			wantDestination:   tests.StrPtr("mailto:a@b.com"),
			wantErr:           nil,
		},
		{
			name:              "Absolute links to releases not processed",
			destination:       "https://github.com/gardener/gardener/releases/tag/v1.4.0",
			contentSourcePath: nodeA.Source,
			wantDestination:   tests.StrPtr("https://github.com/gardener/gardener/releases/tag/v1.4.0"),
			wantErr:           nil,
		},
		{
			name:              "Relative links to releases not processed",
			destination:       "../../../releases/tag/v1.4.0",
			contentSourcePath: nodeA.Source,
			wantDestination:   tests.StrPtr("https://github.com/gardener/gardener/releases/tag/v1.4.0"),
			wantErr:           nil,
		},
		// links to resources
		{
			name:              "Relative link to resource NOT in embeddable",
			destination:       "./image.png",
			contentSourcePath: "https://github.com/gardener/gardener/blob/v1.10.0/docs/README.md",
			wantDestination:   tests.StrPtr("https://github.com/gardener/gardener/blob/v1.10.0/docs/image.png"),
			wantErr:           nil,
		},
		{
			name: "Relative link to resource is embeddable",
			node: &api.Node{
				Source: "https://github.com/gardener/gardener/blob/v1.10.0/docs/README.md",
			},
			destination:       "./image.png",
			contentSourcePath: "https://github.com/gardener/gardener/blob/v1.10.0/docs/README.md",
			wantDestination:   tests.StrPtr("/__resources"),
			wantErr:           nil,
			embeddable:        true,
			mutate: func(c *nodeContentProcessor) {
				h := resourcehandlersfakes.FakeResourceHandler{}
				h.AcceptReturns(true)
				h.BuildAbsLinkReturns("https://github.com/gardener/gardener/blob/v1.10.0/docs/image.png", nil)
				h.SetVersionStub = func(absLink, version string) (string, error) {
					return absLink, nil
				}
				c.resourceHandlers = resourcehandlers.NewRegistry(&h)
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
			wantErr:           nil,
		},
		{
			name:              "Absolute link to resource NOT in download scope",
			node:              nodeA,
			destination:       "https://github.com/owner/repo/blob/master/docs/image.png",
			contentSourcePath: nodeA.Source,
			wantDestination:   tests.StrPtr("https://github.com/owner/repo/blob/master/docs/image.png"),
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
			wantErr:           nil,
			embeddable:        true,
		},
		// links to documents
		{
			name:              "Absolute link to document NOT in download scope and NOT from structure",
			node:              nodeA,
			destination:       "https://github.com/owner/repo/blob/master/docs/doc.md",
			contentSourcePath: nodeA.Source,
			wantDestination:   tests.StrPtr("https://github.com/owner/repo/blob/master/docs/doc.md"),
			wantErr:           nil,
		},
		{
			name:              "Absolute link to document in download scope and from structure",
			node:              nodeA,
			destination:       nodeB.Source,
			contentSourcePath: nodeA.Source,
			wantDestination:   tests.StrPtr("./node_B.md"),
			wantErr:           nil,
			sourceLocations:   map[string][]*api.Node{nodeA.Source: {nodeA}, nodeB.Source: {nodeB}},
		},
		{
			name:              "Relative link to document NOT in download scope and NOT from structure",
			node:              nodeA,
			destination:       "https://github.com/owner/repo/blob/master/docs/doc.md",
			contentSourcePath: nodeA.Source,
			wantDestination:   tests.StrPtr("https://github.com/owner/repo/blob/master/docs/doc.md"),
			wantErr:           nil,
		},
		{
			name:              "Relative link to document in download scope and NOT from structure",
			node:              nodeA,
			destination:       "./another.md",
			contentSourcePath: nodeA.Source,
			wantDestination:   tests.StrPtr("https://github.com/gardener/gardener/blob/v1.10.0/docs/another.md"),
			wantErr:           nil,
		},
		{
			name: "Relative link to resource in download scope with rewrites",
			node: &api.Node{
				Source: "https://github.com/gardener/gardener/blob/v1.10.0/docs/README.md",
			},
			destination:       "./image.png",
			contentSourcePath: "https://github.com/gardener/gardener/blob/v1.10.0/docs/README.md",
			wantDestination:   tests.StrPtr("/__resources"),
			embeddable:        true,
			wantErr:           nil,
			mutate: func(c *nodeContentProcessor) {
				h := resourcehandlersfakes.FakeResourceHandler{}
				h.AcceptReturns(true)
				h.BuildAbsLinkReturns("https://github.com/gardener/gardener/blob/v1.10.0/docs/image.png", nil)
				h.SetVersionStub = func(absLink, version string) (string, error) {
					return absLink, nil
				}
				c.resourceHandlers = resourcehandlers.NewRegistry(&h)
			},
		},
	}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			c := &nodeContentProcessor{
				resourcesRoot:    "/__resources",
				resourceHandlers: resourcehandlers.NewRegistry(githubHandler.NewResourceHandler(github.NewClient(nil), http.DefaultClient, []string{"github.com"})),
				rewriteEmbedded:  true,
				validator:        &fakeValidator{},
				downloader:       &fakeDownload{},
				sourceLocations:  tc.sourceLocations,
			}
			if tc.mutate != nil {
				tc.mutate(c)
			}
			lr := &linkResolver{
				nodeContentProcessor: c,
				node:                 tc.node,
				source:               tc.contentSourcePath,
			}

			link, gotErr := lr.resolveBaseLink(tc.destination, tc.embeddable)

			assert.Equal(t, tc.wantErr, gotErr)
			var destination /*, text, title*/ string
			if link.Destination != nil {
				destination = *link.Destination
			}
			if tc.wantDestination != nil {
				if !strings.HasPrefix(destination, *tc.wantDestination) {
					t.Errorf("expected destination starting with %s, was %s", *tc.wantDestination, destination)
					return
				}
			} else {
				assert.Equal(t, tc.wantDestination, link.Destination)
			}
		})
	}
}

type fakeValidator struct{}

func (f *fakeValidator) ValidateLink(linkUrl *urls.URL, linkDestination, contentSourcePath string) bool {
	return true
}

type fakeDownload struct{}

func (f fakeDownload) Schedule(task *DownloadTask) error {
	return nil
}
