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
