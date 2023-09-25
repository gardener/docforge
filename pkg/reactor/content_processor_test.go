// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reactor

import (
	"net/url"
	"strings"
	"testing"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/resourcehandlers/resourcehandlersfakes"
	"github.com/stretchr/testify/assert"
)

// TODO: This is a flaky test. In the future the ResourceHandler should be mocked.
func Test_processLink(t *testing.T) {

	nodeB := &manifest.Node{
		FileType: manifest.FileType{
			File:   "node_B.md",
			Source: "https://github.com/gardener/gardener/blob/v1.10.0/docs/extensions/overview.md",
		},
		Type: "file",
		Path: ".",
	}
	nodeA := &manifest.Node{
		FileType: manifest.FileType{
			File:   "node_A.md",
			Source: "https://github.com/gardener/gardener/blob/v1.10.0/docs/README.md",
		},
		Type: "file",
		Path: ".",
	}

	testCases := []struct {
		name              string
		node              *manifest.Node
		destination       string
		contentSourcePath string
		wantDestination   string
		wantErr           error
		mutate            func(c *nodeContentProcessor)
		embeddable        bool
		sourceLocations   map[string][]*manifest.Node
	}{
		// skipped links
		{
			name:              "Internal document links are not processed",
			destination:       "#internal-link",
			contentSourcePath: "",
			wantDestination:   "#internal-link",
			wantErr:           nil,
		},
		{
			name:              "mailto protocol is not processed",
			destination:       "mailto:a@b.com",
			contentSourcePath: "",
			wantDestination:   "mailto:a@b.com",
			wantErr:           nil,
		},
		{
			name:              "Absolute links to releases not processed",
			destination:       "https://github.com/gardener/gardener/releases/tag/v1.4.0",
			contentSourcePath: nodeA.Source,
			wantDestination:   "https://github.com/gardener/gardener/releases/tag/v1.4.0",
			wantErr:           nil,
		},
		{
			name:              "Relative links to releases not processed",
			destination:       "../../../releases/tag/v1.4.0",
			contentSourcePath: nodeA.Source,
			wantDestination:   "https://github.com/gardener/gardener/releases/tag/v1.4.0",
			wantErr:           nil,
			mutate: func(c *nodeContentProcessor) {
				h := resourcehandlersfakes.FakeResourceHandler{}
				h.AcceptReturns(true)
				h.BuildAbsLinkReturns("https://github.com/gardener/gardener/releases/tag/v1.4.0", nil)
				c.resourceHandlers = resourcehandlers.NewRegistry(&h)
			},
		},
		// links to resources
		{
			name:              "Relative link to resource NOT in embeddable",
			destination:       "./image.png",
			contentSourcePath: "https://github.com/gardener/gardener/blob/v1.10.0/docs/README.md",
			wantDestination:   "https://github.com/gardener/gardener/blob/v1.10.0/docs/image.png",
			wantErr:           nil,
			mutate: func(c *nodeContentProcessor) {
				h := resourcehandlersfakes.FakeResourceHandler{}
				h.AcceptReturns(true)
				h.BuildAbsLinkReturns("https://github.com/gardener/gardener/blob/v1.10.0/docs/image.png", nil)
				c.resourceHandlers = resourcehandlers.NewRegistry(&h)
			},
		},
		{
			name: "Relative link to resource is embeddable",
			node: &manifest.Node{
				FileType: manifest.FileType{
					Source: "https://github.com/gardener/gardener/blob/v1.10.0/docs/README.md",
				},
			},
			destination:       "./image.png",
			contentSourcePath: "https://github.com/gardener/gardener/blob/v1.10.0/docs/README.md",
			wantDestination:   "/__resources",
			wantErr:           nil,
			embeddable:        true,
			mutate: func(c *nodeContentProcessor) {
				h := resourcehandlersfakes.FakeResourceHandler{}
				h.AcceptReturns(true)
				h.BuildAbsLinkReturns("https://github.com/gardener/gardener/blob/v1.10.0/docs/image.png", nil)
				c.resourceHandlers = resourcehandlers.NewRegistry(&h)
			},
		},
		{
			name: "Relative link to resource NOT in download scope",
			node: &manifest.Node{
				FileType: manifest.FileType{
					Source: "https://github.com/gardener/gardener/blob/v1.10.0/docs/README.md",
				},
			},
			destination:       "../image.png",
			contentSourcePath: "https://github.com/gardener/gardener/blob/v1.10.0/docs/README.md",
			wantDestination:   "https://github.com/gardener/gardener/blob/v1.10.0/image.png",
			wantErr:           nil,
			mutate: func(c *nodeContentProcessor) {
				h := resourcehandlersfakes.FakeResourceHandler{}
				h.AcceptReturns(true)
				h.BuildAbsLinkReturns("https://github.com/gardener/gardener/blob/v1.10.0/image.png", nil)
				c.resourceHandlers = resourcehandlers.NewRegistry(&h)
			},
		},
		{
			name:              "Absolute link to resource NOT in download scope",
			node:              nodeA,
			destination:       "https://github.com/owner/repo/blob/master/docs/image.png",
			contentSourcePath: nodeA.Source,
			wantDestination:   "https://github.com/owner/repo/blob/master/docs/image.png",
			wantErr:           nil,
		},
		{
			name: "Absolute link to resource in download scope",
			node: &manifest.Node{
				FileType: manifest.FileType{
					Source: "https://github.com/gardener/gardener/blob/v1.10.0/docs/README.md",
				},
			},
			destination:       "https://github.com/gardener/gardener/blob/v1.10.0/docs/image.png",
			contentSourcePath: "https://github.com/gardener/gardener/blob/v1.10.0/docs/README.md",
			wantDestination:   "/__resources",
			wantErr:           nil,
			embeddable:        true,
		},
		// links to documents
		{
			name:              "Absolute link to document NOT in download scope and NOT from structure",
			node:              nodeA,
			destination:       "https://github.com/owner/repo/blob/master/docs/doc.md",
			contentSourcePath: nodeA.Source,
			wantDestination:   "https://github.com/owner/repo/blob/master/docs/doc.md",
			wantErr:           nil,
		},
		{
			name:              "Absolute link to document in download scope and from structure",
			node:              nodeA,
			destination:       nodeB.Source,
			contentSourcePath: nodeA.Source,
			wantDestination:   "node_B.md",
			wantErr:           nil,
			sourceLocations:   map[string][]*manifest.Node{nodeA.Source: {nodeA}, nodeB.Source: {nodeB}},
		},
		{
			name:              "Relative link to document NOT in download scope and NOT from structure",
			node:              nodeA,
			destination:       "https://github.com/owner/repo/blob/master/docs/doc.md",
			contentSourcePath: nodeA.Source,
			wantDestination:   "https://github.com/owner/repo/blob/master/docs/doc.md",
			wantErr:           nil,
		},
		{
			name:              "Relative link to document in download scope and NOT from structure",
			node:              nodeA,
			destination:       "./another.md",
			contentSourcePath: nodeA.Source,
			wantDestination:   "https://github.com/gardener/gardener/blob/v1.10.0/docs/another.md",
			wantErr:           nil,
			mutate: func(c *nodeContentProcessor) {
				h := resourcehandlersfakes.FakeResourceHandler{}
				h.AcceptReturns(true)
				h.BuildAbsLinkReturns("https://github.com/gardener/gardener/blob/v1.10.0/docs/another.md", nil)
				c.resourceHandlers = resourcehandlers.NewRegistry(&h)
			},
		},
		{
			name: "Relative link to resource in download scope with rewrites",
			node: &manifest.Node{
				FileType: manifest.FileType{
					Source: "https://github.com/gardener/gardener/blob/v1.10.0/docs/README.md",
				},
			},
			destination:       "./image.png",
			contentSourcePath: "https://github.com/gardener/gardener/blob/v1.10.0/docs/README.md",
			wantDestination:   "/__resources",
			embeddable:        true,
			wantErr:           nil,
			mutate: func(c *nodeContentProcessor) {
				h := resourcehandlersfakes.FakeResourceHandler{}
				h.AcceptReturns(true)
				h.BuildAbsLinkReturns("https://github.com/gardener/gardener/blob/v1.10.0/docs/image.png", nil)
				c.resourceHandlers = resourcehandlers.NewRegistry(&h)
			},
		},
	}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			frh := &resourcehandlersfakes.FakeResourceHandler{}
			frh.AcceptReturns(true)
			c := &nodeContentProcessor{
				resourcesRoot:    "/__resources",
				resourceHandlers: resourcehandlers.NewRegistry(frh),
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
			u, _ := url.Parse(tc.destination)
			link := &linkInfo{
				URL:                 u,
				originalDestination: tc.destination,
				destination:         tc.destination,
				isEmbeddable:        tc.embeddable,
			}

			gotErr := lr.resolveBaseLink(link)

			assert.Equal(t, tc.wantErr, gotErr)
			var destination string
			if link.destination != "" {
				destination = link.destination
			}
			if tc.wantDestination != "" {
				if !strings.HasPrefix(destination, tc.wantDestination) {
					t.Errorf("expected destination starting with %s, was %s", tc.wantDestination, destination)
					return
				}
			} else {
				assert.Equal(t, tc.wantDestination, link.destination)
			}
		})
	}
}

type fakeValidator struct{}

func (f *fakeValidator) ValidateLink(_ *url.URL, _, _ string) bool {
	return true
}

type fakeDownload struct{}

func (f fakeDownload) Schedule(_ *DownloadTask) error {
	return nil
}
