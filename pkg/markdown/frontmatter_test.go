// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package markdown

import (
	"testing"
)

func TestStripFrontMatter(t *testing.T) {
	testCases := []struct {
		in          string
		wantFM      string
		wantContent string
		wantErr     error
	}{
		{
			in: `---
title: Test
prop: A
---

# Head 1
`,
			wantFM: `title: Test
prop: A
`,
			wantContent: `
# Head 1
`,
		},
		{
			in:          `# Head 1`,
			wantFM:      "",
			wantContent: `# Head 1`,
		},
		{
			in: `---
Title: A
`,
			wantFM:      "",
			wantContent: "",
			wantErr:     ErrFrontMatterNotClosed,
		},
		{
			in: `Some text

---
`,
			wantFM: "",
			wantContent: `Some text

---
`,
		},
		{
			in: `---
title: Core Components
---`,
			wantFM:      "title: Core Components\n",
			wantContent: "",
		},
	}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			fm, c, err := StripFrontMatter([]byte(tc.in))
			if err != tc.wantErr {
				t.Errorf("expected err %v != %v", tc.wantErr, err)
			}
			if string(fm) != tc.wantFM {
				t.Errorf("\nwant frontmatter:\n%s\ngot:\n%s\n", tc.wantFM, fm)
			}
			if string(c) != tc.wantContent {
				t.Errorf("\nwant content:\n%s\ngot:\n%s\n", tc.wantContent, c)
			}
		})
	}
}

func TestMatchFrontMatterRule(t *testing.T) {
	testCases := []struct {
		path      string
		value     interface{}
		data      map[string]interface{}
		wantMatch bool
	}{
		{
			path:  ".A.B",
			value: 5,
			data: map[string]interface{}{
				"A": map[string]interface{}{
					"B": 5,
				},
			},
			wantMatch: true,
		},
		{
			path:  ".A.B",
			value: true,
			data: map[string]interface{}{
				"A": map[string]interface{}{
					"B": true,
				},
			},
			wantMatch: true,
		},
		{
			path:  ".A.B",
			value: "a",
			data: map[string]interface{}{
				"A": map[string]interface{}{
					"B": "a",
				},
			},
			wantMatch: true,
		},
		{
			path:  ".A.**.C",
			value: 5,
			data: map[string]interface{}{
				"A": map[string]interface{}{
					"B": map[string]interface{}{
						"C": 5,
					},
				},
			},
			wantMatch: true,
		},
		{
			path:  ".**",
			value: 5,
			data: map[string]interface{}{
				"A": map[string]interface{}{
					"B": map[string]interface{}{
						"C": 5,
					},
				},
			},
			wantMatch: true,
		},
		{
			path:  ".A.B[1]",
			value: "b",
			data: map[string]interface{}{
				"A": map[string]interface{}{
					"B": []interface{}{"a", "b", "c"},
				},
			},
			wantMatch: true,
		},
		{
			path:  ".A.B[1].C2",
			value: 2,
			data: map[string]interface{}{
				"A": map[string]interface{}{
					"B": []interface{}{
						map[string]interface{}{
							"C1": 1,
						},
						map[string]interface{}{
							"C2": 2,
						},
						map[string]interface{}{
							"C3": 3,
						},
					},
				},
			},
			wantMatch: true,
		},
		{
			path:  ".A.**.C2",
			value: 2,
			data: map[string]interface{}{
				"A": map[string]interface{}{
					"B": []interface{}{
						map[string]interface{}{
							"C1": 1,
						},
						map[string]interface{}{
							"C2": 2,
						},
						map[string]interface{}{
							"C3": 3,
						},
					},
				},
			},
			wantMatch: true,
		},
		{
			path:  ".A.B",
			value: 2,
			data: map[string]interface{}{
				"A": map[string]interface{}{
					"B": 5,
				},
			},
			wantMatch: false,
		},
		{
			path: ".A",
			value: map[string]interface{}{
				"B": []interface{}{"a", "b", "c"},
			},
			data: map[string]interface{}{
				"A": map[string]interface{}{
					"B": []interface{}{"a", "b", "c"},
				},
			},
			wantMatch: true,
		},
	}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			matched := MatchFrontMatterRule(tc.path, tc.value, tc.data)
			if tc.wantMatch && !matched {
				t.Errorf("expected a match for path %s, got no match", tc.path)
			}
		})
	}
}
