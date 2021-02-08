// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package processors

import (
	"reflect"
	"testing"

	"github.com/gardener/docforge/pkg/api"
)

func TestFrontmatterProcess(t *testing.T) {
	testCases := []struct {
		document     string
		node         *api.Node
		wantErr      error
		wantDocument string
		mutate       func(node *api.Node)
	}{
		{
			document: "",
			node: &api.Node{
				Source: "whatever",
				Name:   "test-1_2.md",
			},
			wantErr: nil,
			wantDocument: `---
title: Test 1 2
---
`,
		},
		{
			document: "# Heading",
			node: &api.Node{
				Name: "test",
				Properties: map[string]interface{}{
					"frontmatter": map[string]interface{}{
						"title": "Test1",
					},
				},
				Source: "whatever",
			},
			wantErr: nil,
			wantDocument: `---
title: Test1
---
# Heading`,
		},
		{
			document: `---
prop1: A
---

# Heading`,
			node: &api.Node{
				Name: "test",
				Properties: map[string]interface{}{
					"frontmatter": map[string]interface{}{
						"title": "Test",
					},
				},
				Source: "whatever",
			},
			wantErr: nil,
			wantDocument: `---
prop1: A
title: Test
---

# Heading`,
		},
		{
			document: `---
title: Test1
---`,
			node: &api.Node{
				Name: "test3",
				Properties: map[string]interface{}{
					"frontmatter": map[string]interface{}{
						"title": "Test2",
					},
				},
				Source: "whatever",
			},
			wantErr: nil,
			wantDocument: `---
title: Test2
---
`,
		},
		{
			document: `# heading 1`,
			node: &api.Node{
				Name:   "README.md",
				Source: "whatever",
			},
			wantErr: nil,
			wantDocument: `---
title: Content
---
# heading 1`,
			mutate: func(node *api.Node) {
				node.SetParent(&api.Node{
					Name: "content",
				})
			},
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
			gotDocumentBlob, err := fm.Process([]byte(tc.document), tc.node)
			if err != tc.wantErr {
				t.Errorf("expected err %v!=%v", tc.wantErr, err)
			}
			if !reflect.DeepEqual(string(gotDocumentBlob), tc.wantDocument) {
				t.Errorf("expected bytes \n%s\n!=\n%s\n", tc.wantDocument, string(gotDocumentBlob))
			}
		})
	}
}
