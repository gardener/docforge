package markdown

import (
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
