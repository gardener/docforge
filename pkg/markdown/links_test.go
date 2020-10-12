package markdown

import (
	"bytes"
	"testing"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	"github.com/gomarkdown/markdown/parser"
	"github.com/stretchr/testify/assert"
)

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
				if entering {
					if l, ok := node.(*ast.Link); ok {
						removeDestination(l)
					}
					if l, ok := node.(*ast.Image); ok {
						removeDestination(l)
					}
				}
				return ast.GoToNext
			})
			var (
				links, images int
				texts         = make([]string, 0)
			)
			ast.WalkFunc(document, func(node ast.Node, entering bool) ast.WalkStatus {
				if entering {
					if _, ok := node.(*ast.Link); ok {
						links++
					}
					if _, ok := node.(*ast.Image); ok {
						images++
					}
					if t, ok := node.(*ast.Text); ok {
						texts = append(texts, string(t.Literal))
					}
				}
				return ast.GoToNext
			})
			assert.Equal(t, tc.wantLinksCount, links)
			assert.Equal(t, tc.wantLinksCount, images)
			assert.Equal(t, tc.wantTexts, texts)
		})
	}
}

func TestUpdateText(t *testing.T) {
	testCases := []struct {
		in                    string
		text                  string
		wantTextsUpdatesCount int
	}{
		{
			`A [a0](b.md) [a1](b.md "c") ![](a.png) B`,
			"b",
			3,
		},
	}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			mdParser := parser.NewWithExtensions(extensions)
			document := markdown.Parse([]byte(tc.in), mdParser)
			ast.WalkFunc(document, func(node ast.Node, entering bool) ast.WalkStatus {
				if entering {
					if l, ok := node.(*ast.Link); ok {
						updateText(l, []byte(tc.text))
					}
					if i, ok := node.(*ast.Image); ok {
						updateText(i, []byte(tc.text))
					}
				}
				return ast.GoToNext
			})
			var (
				textsUpdatesCount int
			)
			ast.WalkFunc(document, func(node ast.Node, entering bool) ast.WalkStatus {
				if entering {
					if l, ok := node.(*ast.Link); ok {
						if bytes.Equal(l.Children[0].AsLeaf().Literal, []byte(tc.text)) {
							textsUpdatesCount++
						}
					}
					if i, ok := node.(*ast.Image); ok {
						if bytes.Equal(i.Children[0].AsLeaf().Literal, []byte(tc.text)) {
							textsUpdatesCount++
						}
					}
				}
				return ast.GoToNext
			})
			assert.Equal(t, tc.wantTextsUpdatesCount, textsUpdatesCount)
		})
	}
}
