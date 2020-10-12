package markdown

import (
	"github.com/gardener/docforge/pkg/markdown/renderer"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	"github.com/gomarkdown/markdown/parser"
)

const (
	extensions = parser.CommonExtensions | parser.AutoHeadingIDs
)

// OnLink is a callback function invoked on each link
// by mardown#UpdateLinkRefs
// It is supplied a link and is expected to return destination,
// text, title or error.
// A nil destination will yield removing of this link/image markup,
// leaving only the text component if it's a link
// Nil text or title returned yield no change. Any other value replaces
// the original. If a returned title is empty string an originally
// existing title element will be completely removed
type OnLink func(destination, text, title []byte) ([]byte, []byte, []byte, error)

func removeDestination(node ast.Node) {
	children := node.GetParent().GetChildren()
	idx := nodeIndex(node)
	if idx > -1 {
		if link, ok := node.(*ast.Link); ok {
			textNode := link.Children[0]
			if textNode != nil && len(textNode.AsLeaf().Literal) > 0 {
				// if prev sibling is text node, add this link text to it
				if idx > 0 {
					_n := children[idx-1]
					if t, ok := _n.(*ast.Text); ok {
						t.Literal = append(t.Literal, textNode.AsLeaf().Literal...)
						children = removeNode(children, idx)
						node.GetParent().SetChildren(children)
						return
					}
				}
				// if next sibling is text node, add this link text to it
				if idx < len(children)-1 {
					_n := children[idx+1]
					if t, ok := _n.(*ast.Text); ok {
						t.Literal = append(t.Literal, textNode.AsLeaf().Literal...)
						children = removeNode(children, idx)
						node.GetParent().SetChildren(children)
						return
					}
				}
				node.GetParent().AsContainer().Children[idx] = textNode
				return
			}
		}
		if _, ok := node.(*ast.Image); ok {
			children = removeNode(children, idx)
			node.GetParent().SetChildren(children)
			return
		}
	}
}
func removeNode(n []ast.Node, i int) []ast.Node {
	return append(n[:i], n[i+1:]...)
}
func nodeIndex(node ast.Node) int {
	children := node.GetParent().GetChildren()
	idx := -1
	for i, p := range children {
		if p == node {
			idx = i
			break
		}
	}
	return idx
}

func updateText(node ast.Node, text []byte) {
	idx := nodeIndex(node)
	if idx > -1 {
		if link, ok := node.(*ast.Link); ok {
			textNode := link.AsContainer().Children[0]
			textNode.AsLeaf().Literal = text
			return
		}
		if image, ok := node.(*ast.Image); ok {
			textNode := image.AsContainer().Children[0]
			textNode.AsLeaf().Literal = text
			return
		}
	}
}

// UpdateLinkRefs changes document links destinations, consulting
// with callback on the destination to use on each link or image in document.
// If a callback returns "" for a destination, this is interpreted as
// request to remove the link destination and leave only the link text or in
// case it's an image - to remvoe it completely.
// TODO: failfast vs fault tolerance support
func UpdateLinkRefs(documentBlob []byte, callback OnLink) ([]byte, error) {
	mdParser := parser.NewWithExtensions(extensions)
	document := markdown.Parse(documentBlob, mdParser)
	ast.WalkFunc(document, func(_node ast.Node, entering bool) ast.WalkStatus {
		if entering {
			var (
				destination, text, title []byte
				err                      error
			)
			if l, ok := _node.(*ast.Link); ok {
				text = l.GetChildren()[0].AsLeaf().Literal
				if destination, text, title, err = callback(l.Destination, text, l.Title); err != nil {
					return ast.Terminate
				}
				if destination != nil {
					updateText(_node, text)
				}
				if destination == nil {
					removeDestination(l)
					return ast.GoToNext
				}
				l.Destination = destination
				if title != nil {
					l.Title = title
				}
				return ast.GoToNext
			}
			if l, ok := _node.(*ast.Image); ok {
				text = l.GetChildren()[0].AsLeaf().Literal
				if destination, text, title, err = callback(l.Destination, text, l.Title); err != nil {
					return ast.Terminate
				}
				if destination != nil {
					updateText(_node, text)
				}
				if destination == nil {
					removeDestination(l)
					return ast.GoToNext
				}
				l.Destination = destination
				if title != nil {
					l.Title = title
				}
				return ast.GoToNext
			}
		}
		return ast.GoToNext
	})
	r := renderer.NewRenderer(renderer.RendererOptions{
		TextWidth: -1,
	})
	documentBlob = markdown.Render(document, r)
	return documentBlob, nil
}
