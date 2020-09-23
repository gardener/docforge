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
// by mardown#TransformLinks
type OnLink func(destination []byte) ([]byte, error)

const (
	extensions = parser.CommonExtensions | parser.AutoHeadingIDs
)

//TransformLinks transforms document links destinations, delegating
// the transformation to a callback invoked on each link
// TODO: failfast vs fault tolerance support
func TransformLinks(documentBlob []byte, callback OnLink) ([]byte, error) {
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
				l.Destination = destination
				return ast.GoToNext
			}
			if l, ok := _node.(*ast.Image); ok {
				if destination, err = callback(l.Destination); err != nil {
					return ast.Terminate
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

		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, nil, err
		}

		if strings.TrimSpace(line) != "---" {
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
		return nil, nil, errors.New("Missing closing front-matter `---` mark found")
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
	buf.WriteString("---\n\n")
	buf.Write(content)
	if data, err = ioutil.ReadAll(buf); err != nil {
		return nil, err
	}
	return data, nil
}
