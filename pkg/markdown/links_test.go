// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package markdown

// func TestRemoveLink(t *testing.T) {
// 	testCases := []struct {
// 		in              string
// 		wantLinksCount  int
// 		wantImagesCount int
// 		wantTexts       []string
// 	}{
// 		{
// 			`A [a0](b.md) [a1](b.md "c") ![](a.png) B`,
// 			0, 0,
// 			[]string{"A a0", " a1", " ", " B"},
// 		},
// 	}
// 	for _, tc := range testCases {
// 		t.Run("", func(t *testing.T) {
// 			mdParser := parser.NewWithExtensions(extensions)
// 			document := markdown.Parse([]byte(tc.in), mdParser)
// 			ast.WalkFunc(document, func(node ast.Node, entering bool) ast.WalkStatus {
// 				if entering {
// 					if l, ok := node.(*ast.Link); ok {
// 						removeDestination(l)
// 					}
// 					if l, ok := node.(*ast.Image); ok {
// 						removeDestination(l)
// 					}
// 				}
// 				return ast.GoToNext
// 			})
// 			var (
// 				links, images int
// 				texts         = make([]string, 0)
// 			)
// 			ast.WalkFunc(document, func(node ast.Node, entering bool) ast.WalkStatus {
// 				if entering {
// 					if _, ok := node.(*ast.Link); ok {
// 						links++
// 					}
// 					if _, ok := node.(*ast.Image); ok {
// 						images++
// 					}
// 					if t, ok := node.(*ast.text); ok {
// 						texts = append(texts, string(t.Literal))
// 					}
// 				}
// 				return ast.GoToNext
// 			})
// 			assert.Equal(t, tc.wantLinksCount, links)
// 			assert.Equal(t, tc.wantLinksCount, images)
// 			assert.Equal(t, tc.wantTexts, texts)
// 		})
// 	}
// }

// func TestSetText(t *testing.T) {
// 	testCases := []struct {
// 		in                    string
// 		text                  string
// 		wantTextsUpdatesCount int
// 	}{
// 		{
// 			`A [a0](b.md) [a1](b.md "c") ![](a.png) B`,
// 			"b",
// 			3,
// 		},
// 		{
// 			`A [a0](b.md) [a1](b.md "c") ![](a.png) B`,
// 			"",
// 			3,
// 		},
// 	}
// 	for _, tc := range testCases {
// 		t.Run("", func(t *testing.T) {
// 			mdParser := parser.NewWithExtensions(extensions)
// 			document := markdown.Parse([]byte(tc.in), mdParser)
// 			ast.WalkFunc(document, func(node ast.Node, entering bool) ast.WalkStatus {
// 				if entering {
// 					if l, ok := node.(*ast.Link); ok {
// 						setText(l, []byte(tc.text))
// 					}
// 					if i, ok := node.(*ast.Image); ok {
// 						setText(i, []byte(tc.text))
// 					}
// 				}
// 				return ast.GoToNext
// 			})
// 			var (
// 				textsUpdatesCount int
// 			)
// 			ast.WalkFunc(document, func(node ast.Node, entering bool) ast.WalkStatus {
// 				if entering {
// 					if l, ok := node.(*ast.Link); ok {
// 						if bytes.Equal(l.Children[0].AsLeaf().Literal, []byte(tc.text)) {
// 							textsUpdatesCount++
// 						}
// 					}
// 					if i, ok := node.(*ast.Image); ok {
// 						if bytes.Equal(i.Children[0].AsLeaf().Literal, []byte(tc.text)) {
// 							textsUpdatesCount++
// 						}
// 					}
// 				}
// 				return ast.GoToNext
// 			})
// 			assert.Equal(t, tc.wantTextsUpdatesCount, textsUpdatesCount)
// 		})
// 	}
// }

// //TODO: improve the test results checking
// func TestUpdateLink(t *testing.T) {
// 	testCases := []struct {
// 		in                         string
// 		destination                []byte
// 		text                       []byte
// 		title                      []byte
// 		wantLinkTextUpdates        []string
// 		wantLinkDestinationUpdates []string
// 		wantLinkTitleUpdates       []string
// 		wantLinkUpdatesCount       int
// 		wantImageUpdatesCount      int
// 	}{
// 		{
// 			`A [a0](b.md) [a1](b.md "c") ![](a.png) B`,
// 			[]byte("b"),
// 			[]byte("new"),
// 			nil,
// 			nil,
// 			nil,
// 			nil,
// 			2,
// 			1,
// 		},
// 		{
// 			`A [a0](b.md) [a1](b.md "c") ![](a.png) B`,
// 			nil,
// 			[]byte(""),
// 			nil,
// 			nil,
// 			nil,
// 			nil,
// 			0,
// 			0,
// 		},
// 		{
// 			`A [a0](b.md) [a1](b.md "c") ![](a.png) B`,
// 			nil,
// 			[]byte("A"),
// 			nil,
// 			nil,
// 			nil,
// 			nil,
// 			0,
// 			0,
// 		},
// 	}
// 	for _, tc := range testCases {
// 		t.Run("", func(t *testing.T) {
// 			mdParser := parser.NewWithExtensions(extensions)
// 			document := markdown.Parse([]byte(tc.in), mdParser)
// 			ast.WalkFunc(document, func(node ast.Node, entering bool) ast.WalkStatus {
// 				if entering {
// 					if l, ok := node.(*ast.Link); ok {
// 						updateLink(l, tc.destination, tc.text, tc.title)
// 					}
// 					if i, ok := node.(*ast.Image); ok {
// 						updateLink(i, tc.destination, tc.text, tc.title)
// 					}
// 				}
// 				return ast.GoToNext
// 			})
// 			var (
// 				linkUpdatesCount  int
// 				imageUpdatesCount int
// 			)
// 			ast.WalkFunc(document, func(node ast.Node, entering bool) ast.WalkStatus {
// 				if entering {
// 					if l, ok := node.(*ast.Link); ok {
// 						text := l.Children[0].AsLeaf().Literal
// 						destination := l.destination
// 						title := l.Title
// 						if bytes.Equal(text, tc.text) || bytes.Equal(destination, tc.destination) || bytes.Equal(title, tc.title) {
// 							linkUpdatesCount++
// 						}
// 					}
// 					if i, ok := node.(*ast.Image); ok {
// 						text := i.Children[0].AsLeaf().Literal
// 						destination := i.destination
// 						title := i.Title
// 						if bytes.Equal(text, tc.text) || bytes.Equal(destination, tc.destination) || bytes.Equal(title, tc.title) {
// 							imageUpdatesCount++
// 						}
// 					}
// 				}
// 				return ast.GoToNext
// 			})
// 			assert.Equal(t, tc.wantLinkUpdatesCount, linkUpdatesCount, "link updates")
// 			assert.Equal(t, tc.wantImageUpdatesCount, imageUpdatesCount, "image updates")
// 		})
// 	}
// }

// func TestParseLinks(t *testing.T) {
// 	p := NewParser()
// 	p.parse(&ast.Container{}, []byte("  [a](b.com)"))
// }
