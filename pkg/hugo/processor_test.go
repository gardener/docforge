// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package hugo

import (
	"bytes"
	"testing"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/processors"
	"github.com/gardener/docforge/pkg/util/tests"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/pointer"
)

func init() {
	tests.SetKlogV(6)
}

func TestHugoProcess(t *testing.T) {
	testCases := []struct {
		name    string
		in      []byte
		node    *api.Node
		want    []byte
		wantErr error
		mutate  func(p *Processor, node *api.Node)
		links   []*processors.Link
	}{
		{
			name: "",
			in:   []byte(`[GitHub](./a/b.md)`),
			node: &api.Node{Name: "Test"},
			want: []byte("[GitHub](/a/b)"),
			mutate: func(p *Processor, node *api.Node) {
				p.PrettyUrls = true
			},
			links: []*processors.Link{
				{
					DestinationNode: createNodeWithParents("b.md", "a"),
					Destination:     pointer.StringPtr("./a/b.md"),
				},
			},
		},
		{
			name:   "",
			in:     []byte(`<a href="https://a.com/b.md">B</a>`),
			node:   &api.Node{Name: "Test"},
			want:   []byte(`<a href="https://a.com/b.md">B</a>`),
			mutate: nil,
		},
		{
			name: "",
			in:   []byte(`<a href="a/b.md">B</a>`),
			node: &api.Node{Name: "Test"},
			want: []byte(`<a href="/a/b">B</a>`),
			mutate: func(p *Processor, node *api.Node) {
				p.PrettyUrls = true
			},
			links: []*processors.Link{
				{
					DestinationNode: createNodeWithParents("b.md", "a"),
					Destination:     pointer.StringPtr("a/b.md"),
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			p := &Processor{}
			if tc.mutate != nil {
				tc.mutate(p, tc.node)
			}
			document := &processors.Document{
				Node:          tc.node,
				DocumentBytes: tc.in,
				Links:         tc.links,
			}
			err := p.Process(document)

			if tc.wantErr != err {
				t.Errorf("want err %v != %v", tc.wantErr, err)
			}
			assert.Equal(t, string(tc.want), string(document.DocumentBytes))
		})
	}
}

func TestRewriteDestination(t *testing.T) {
	testCases := []struct {
		name            string
		destination     string
		text            string
		title           string
		nodeName        string
		isNodeIndexFile bool
		wantDestination string
		wantText        string
		wantTitle       string
		wantError       error
		mutate          func(h *Processor)
		destinationNode *api.Node
		isResource      bool
	}{
		{
			name:            "doesn't_change_links_that_begin_with_#",
			destination:     "#fragment-id",
			text:            "",
			title:           "",
			nodeName:        "testnode",
			isNodeIndexFile: false,
			wantDestination: "#fragment-id",
			wantTitle:       "",
			wantText:        "",
			wantError:       nil,
			mutate:          nil,
			destinationNode: nil,
		},
		{
			name:            "doesn't_change_absolute_links",
			destination:     "https://github.com/a/b/sample.md",
			text:            "",
			title:           "",
			nodeName:        "testnode",
			isNodeIndexFile: false,
			wantDestination: "https://github.com/a/b/sample.md",
			wantTitle:       "",
			wantText:        "",
			wantError:       nil,
			mutate:          nil,
			destinationNode: nil,
		},
		{
			name:            "to_relative_link_when_node_is_part_of_structure",
			destination:     "./a/b/sample.md",
			text:            "",
			title:           "",
			nodeName:        "testnode",
			isNodeIndexFile: false,
			wantDestination: "/a/b/sample",
			wantTitle:       "",
			wantText:        "",
			wantError:       nil,
			mutate:          func(h *Processor) { h.PrettyUrls = true },
			destinationNode: createNodeWithParents("sample.md", "b", "a"),
		},
		{
			name:            "changes_file_extension_when_pretty_urls_is_disabled",
			destination:     "./a/b/README.md",
			text:            "",
			title:           "",
			nodeName:        "testnode",
			isNodeIndexFile: false,
			wantDestination: "/a/b/readme.html",
			wantTitle:       "",
			wantText:        "",
			wantError:       nil,
			mutate:          nil,
			destinationNode: createNodeWithParents("README.md", "b", "a"),
		},
		{
			name:            "to_link_known_as_hugo_section_when_link_destination_matches_index_file_names",
			destination:     "./a/b/README.md",
			text:            "",
			title:           "",
			nodeName:        "testnode",
			isNodeIndexFile: false,
			wantDestination: "/base/url/a/b",
			wantTitle:       "",
			wantText:        "",
			wantError:       nil,
			mutate: func(h *Processor) {
				h.PrettyUrls = true
				h.IndexFileNames = []string{"readme", "read.me", "index", "_index"}
				h.BaseURL = "/base/url"
			},
			destinationNode: createNodeWithParents("README.md", "b", "a"),
		},
		{
			name:            "to_the_root_relative_link_of_a_resource",
			destination:     "images/1.png",
			isNodeIndexFile: true,
			nodeName:        "_index.md",
			wantDestination: "/images/1.png",
			mutate: func(h *Processor) {
				h.PrettyUrls = true
			},
			isResource: true,
		},
		{
			name:            "to_the_root_relative_link_of_a_resource",
			destination:     "../../images/1.png",
			isNodeIndexFile: true,
			nodeName:        "_index.md",
			wantDestination: "/images/1.png",
			mutate: func(h *Processor) {
				h.PrettyUrls = true
			},
			isResource: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p := &Processor{}
			if tc.mutate != nil {
				tc.mutate(p)
			}
			l := &processors.Link{
				DestinationNode: tc.destinationNode,
				Destination:     &tc.destination,
				IsResource:      tc.isResource,
			}
			gotDestination, gotText, gotTitle, gotErr := p.rewriteDestination([]byte(tc.destination), []byte(tc.text), []byte(tc.title), tc.nodeName, tc.isNodeIndexFile, l)

			if gotErr != tc.wantError {
				t.Errorf("want error %v != %v", tc.wantError, gotErr)
			}
			if !bytes.Equal(gotDestination, []byte(tc.wantDestination)) {
				t.Errorf("want destination %v != %v", tc.wantDestination, string(gotDestination))
			}
			if !bytes.Equal(gotText, []byte(tc.wantText)) {
				t.Errorf("want text %v != %v", tc.wantText, string(gotText))
			}
			if !bytes.Equal(gotTitle, []byte(tc.wantTitle)) {
				t.Errorf("want title %v != %v", tc.wantTitle, string(gotTitle))
			}
		})
	}
}

func createNodeWithParents(name string, parentNames ...string) *api.Node {
	destinationNode := &api.Node{
		Name: name,
	}
	childNode := destinationNode
	for _, n := range parentNames {
		parent := &api.Node{
			Name: n,
		}
		childNode.SetParent(parent)
		childNode = parent
	}

	return destinationNode
}

func TestProcessor_nodeIsIndexFile(t *testing.T) {
	tests := []struct {
		name           string
		IndexFileNames []string
		nodeName       string
		want           bool
	}{
		{
			name:           "returns_true_when_compared_to_default_index_name",
			IndexFileNames: []string{},
			nodeName:       "_index.md",
			want:           true,
		},
		{
			name:           "returns_false_if_not_equal_to_default_index_name",
			IndexFileNames: []string{},
			nodeName:       "someNodeName.md",
			want:           false,
		},
		{
			name: "returns_false_if_not_equal_to_default_index_name",
			IndexFileNames: []string{
				"someNodeName.md",
			},
			nodeName: "someNodeName.md",
			want:     true,
		},
		{
			name: "returns_true_ignoring_text_case_type",
			IndexFileNames: []string{
				"someNodeName.md",
			},
			nodeName: "somenodename.md",
			want:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &Processor{
				IndexFileNames: tt.IndexFileNames,
			}
			if got := f.nodeIsIndexFile(tt.nodeName); got != tt.want {
				t.Errorf("Processor.nodeIsIndexFile() = %v, want %v", got, tt.want)
			}
		})
	}
}
