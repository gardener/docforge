package markdown

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"strings"

	"github.com/gardener/docforge/pkg/markdown/renderer"
	md "github.com/gardener/docforge/pkg/markdown/renderer"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	"github.com/gomarkdown/markdown/parser"
)

// OnLink is a callback function invoked on each link
// by mardown#UpdateLinkRefDestinations
type OnLink func(destination []byte) ([]byte, error)

const (
	extensions = parser.CommonExtensions | parser.AutoHeadingIDs
)

func removeDestination(node ast.Node) {
	children := node.GetParent().GetChildren()
	idx := -1
	for i, p := range children {
		if p == node {
			idx = i
			break
		}
	}
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

// UpdateLinkRefDestinations changes document links destinations, consulting
// with callback on the destination to use on each link or image in document.
// If a callback returns "" for a destination, this is interpreted as
// request to remove the link destination and leave only the link text or in
// case it's an image - to remvoe it completely.
// TODO: failfast vs fault tolerance support
func UpdateLinkRefDestinations(documentBlob []byte, callback OnLink) ([]byte, error) {
	mdParser := parser.NewWithExtensions(extensions)
	document := markdown.Parse(documentBlob, mdParser)
	ast.WalkFunc(document, func(_node ast.Node, entering bool) ast.WalkStatus {
		if entering {
			var (
				destination []byte
				err         error
			)
			if l, ok := _node.(*ast.Link); ok {
				if destination, err = callback(l.Destination); err != nil {
					return ast.Terminate
				}
				if destination == nil {
					removeDestination(l)
					return ast.GoToNext
				}
				l.Destination = destination
				return ast.GoToNext
			}
			if l, ok := _node.(*ast.Image); ok {
				if destination, err = callback(l.Destination); err != nil {
					return ast.Terminate
				}
				if destination == nil {
					removeDestination(l)
					return ast.GoToNext
				}
				l.Destination = destination
				return ast.GoToNext
			}
		}
		return ast.GoToNext
	})
	r := md.NewRenderer(renderer.RendererOptions{
		TextWidth: -1,
	})
	documentBlob = markdown.Render(document, r)
	return documentBlob, nil
}

// ErrFrontMatterNotClosed is raised to signal
// that the rules for defining a frontmatter element
// in a markdown document have been violated
var ErrFrontMatterNotClosed error = errors.New("Missing closing frontmatter `---` found")

// StripFrontMatter splits a provided document into front-matter
// and content.
func StripFrontMatter(b []byte) ([]byte, []byte, error) {
	var (
		started      bool
		yamlBeg      int
		yamlEnd      int
		contentStart int
	)

	buf := bytes.NewBuffer(b)

	for {
		line, err := buf.ReadString('\n')

		if errors.Is(err, io.EOF) {
			// handle documents that contain only forntmatter
			// and no line ending after closing ---
			if started && yamlEnd == 0 {
				if l := strings.TrimSpace(line); l == "---" {
					yamlEnd = len(b) - buf.Len() - len([]byte(line))
					contentStart = len(b)
				}

			}
			break
		}

		if err != nil {
			return nil, nil, err
		}

		if l := strings.TrimSpace(line); l != "---" {
			// Only whitespace is acceptable before front-matter
			// Any other preceding text is interpeted as frontmater-less
			// document
			if !started && len(l) > 0 {
				return nil, b, nil
			}
			continue
		}

		if !started {
			started = true
			yamlBeg = len(b) - buf.Len()
		} else {
			yamlEnd = len(b) - buf.Len() - len([]byte(line))
			contentStart = yamlEnd + len([]byte(line))
			break
		}
	}

	if started && yamlEnd == 0 {
		return nil, nil, ErrFrontMatterNotClosed
	}

	fm := b[yamlBeg:yamlEnd]
	content := b[contentStart:]

	return fm, content, nil
}

// InsertFrontMatter prepends the content bytes with
// front matter enclosed in the standard marks ---
func InsertFrontMatter(fm []byte, content []byte) ([]byte, error) {
	var (
		data []byte
		err  error
	)
	if len(fm) < 1 {
		return content, nil
	}
	buf := bytes.NewBuffer([]byte("---\n"))
	buf.Write(fm)
	// TODO: configurable empty line after frontmatter
	buf.WriteString("---\n")
	buf.Write(content)
	if data, err = ioutil.ReadAll(buf); err != nil {
		return nil, err
	}
	return data, nil
}
