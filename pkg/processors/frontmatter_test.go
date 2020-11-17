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
	}{
		{
			document: "",
			node: &api.Node{
				Name: "test",
			},
			wantErr: nil,
			wantDocument: `---
title: Test
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
			},
			wantErr: nil,
			wantDocument: `---
title: Test2
---
`,
		},
	}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			fm := &FrontMatter{}
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
