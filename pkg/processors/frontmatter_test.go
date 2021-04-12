// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package processors

import (
	"testing"

	"github.com/gardener/docforge/pkg/api"
)

func TestFrontmatterProcess(t *testing.T) {
	testCases := []struct {
		name                string
		strippedFrontMatter string
		node                *api.Node
		wantErr             error
		wantDocument        string
		mutate              func(node *api.Node)
	}{
		{
			name:                "adds_missing_title_to_document_front_matter",
			strippedFrontMatter: "",
			node: &api.Node{
				Source: "whatever",
				Name:   "test-1_2.md",
			},
			wantErr:      nil,
			wantDocument: `title: Test 1 2`,
		},
		{
			name:                "reuses_defined_title_in_document",
			strippedFrontMatter: "# Heading",
			node:                &api.Node{Name: "test", Properties: map[string]interface{}{"frontmatter": map[string]interface{}{"title": "Test1"}}, Source: "whatever"},
			wantErr:             nil,
			wantDocument:        `title: Test1`,
			mutate: func(node *api.Node) {
			},
		},
		{
			name:                "merge_front_matter_from_doc_and_node",
			strippedFrontMatter: `prop1: A`,
			node:                &api.Node{Name: "test", Properties: map[string]interface{}{"frontmatter": map[string]interface{}{"title": "Test"}}, Source: "whatever"},
			wantErr:             nil,
			wantDocument: `prop1: A
title: Test`,
			mutate: func(node *api.Node) {
			},
		},
		{
			name:                "rewrite_title_defined_in_the_stripped_fm",
			strippedFrontMatter: `title: Test1`,
			node: &api.Node{
				Name: "test3",
				Properties: map[string]interface{}{
					"frontmatter": map[string]interface{}{
						"title": "Test2",
					},
				},
				Source: "whatever",
			},
			wantErr:      nil,
			wantDocument: `title: Test2`,
		},
	}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			fm := &FrontMatter{
				IndexFileNames: []string{"README.md"},
			}
			if tc.mutate != nil {
				tc.mutate(tc.node)
			}
			document := &Document{
				Node:        tc.node,
				FrontMatter: []byte(tc.strippedFrontMatter),
			}
			err := fm.Process(document)
			if err != tc.wantErr {
				t.Errorf("expected err %v!=%v", tc.wantErr, err)
			}
			if string(document.FrontMatter) == tc.wantDocument {
				t.Errorf("expected bytes \n%s\n!=\n%s\n", tc.wantDocument, string(document.FrontMatter))
			}
		})
	}
}
