package markdown

import (
	"testing"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	"github.com/gomarkdown/markdown/parser"
	"github.com/stretchr/testify/assert"
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

func TestRemoveLink(t *testing.T) {
	testCases := []struct {
		in             string
		wantLinksCount int
		wantImgsCount  int
		wantTexts      []string
	}{
		{
			`A [a0](b.md) [a1](b.md "c") ![](a.png) B`,
			0, 0,
			[]string{"A a0", " a1", " ", " B"},
		},
	}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			mdParser := parser.NewWithExtensions(extensions)
			document := markdown.Parse([]byte(tc.in), mdParser)
			ast.WalkFunc(document, func(node ast.Node, entering bool) ast.WalkStatus {
				if l, ok := node.(*ast.Link); ok {
					removeDestination(l)
				}
				if l, ok := node.(*ast.Image); ok {
					removeDestination(l)
				}
				return ast.GoToNext
			})
			var (
				links, images int
				texts         = make([]string, 0)
			)
			ast.WalkFunc(document, func(node ast.Node, entering bool) ast.WalkStatus {
				if _, ok := node.(*ast.Link); ok {
					links++
				}
				if _, ok := node.(*ast.Image); ok {
					images++
				}
				if t, ok := node.(*ast.Text); ok {
					texts = append(texts, string(t.Literal))
				}
				return ast.GoToNext
			})
			assert.Equal(t, tc.wantLinksCount, links)
			assert.Equal(t, tc.wantLinksCount, images)
			assert.Equal(t, tc.wantTexts, texts)
		})
	}
}
