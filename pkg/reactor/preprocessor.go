package reactor

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/Kunde21/markdownfmt/v2/markdown"

	"github.com/gardener/docode/pkg/api"
	"github.com/gardener/docode/pkg/resourcehandlers"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// PreProcess TODO:
func (r *Reactor) PreProcess(contentBytes []byte, source string, node *api.Node) error {
	contentSelectors := node.ContentSelectors
	if len(contentSelectors) > 0 {
		//TODO: fixme
		for _, cs := range contentSelectors {
			SelectContent(contentBytes, *cs.Selector)
			// HarvestLinks(cs.Source, contentBytes, nil)
		}
	}
	return fmt.Errorf("No ResourceHandler found for URI %s", source)
}

// SelectContent TODO:
func SelectContent(contentBytes []byte, selectorExpression string) ([]byte, error) {
	// TODO: select content sections from contentBytes if source has a content selector and then filter the rest of it.
	// TODO: define selector expression language. Do CSS/SaaS selectors or alike apply/ can be adapted?
	// Example: "h1-first-of-type" -> the first level one heading (#) in the document
	return contentBytes, nil
}

// HarvestLinks TODO:
func HarvestLinks(contentSourcePath string, nodeTargetPath string, contentBytes []byte, rdCh chan *ResourceData) ([]byte, error) {
	// TODO: harvest links from this contentBytes
	// and resolve them to downloadable addresses and serialization targets
	var b bytes.Buffer
	p := parser.NewParser(parser.WithBlockParsers(parser.DefaultBlockParsers()...),
		parser.WithInlineParsers(parser.DefaultInlineParsers()...),
		parser.WithParagraphTransformers(parser.DefaultParagraphTransformers()...),
	)
	hrefAttrMatchRegex := regexp.MustCompile(`href=["\']?([^"\'>]+)["\']?`)
	srcAttrMatchRegex := regexp.MustCompile(`src=["\']?([^"\'>]+)["\']?`)
	reader := text.NewReader(contentBytes)
	doc := p.Parse(reader)
	if err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if node.Kind() == ast.KindLink {
				n := node.(*ast.Link)
				handler := resourcehandlers.Get(contentSourcePath)
				absLink, rewrite := handler.ResolveRelLink(contentSourcePath, string(n.Destination))
				if absLink != "" && !rewrite {
					rdCh <- &ResourceData{
						OriginalPath:   string(n.Destination),
						Source:         absLink,
						NodeTargetPath: nodeTargetPath,
					}
					return ast.WalkContinue, nil
				}
				n.Destination = []byte(absLink)
			}
			if node.Kind() == ast.KindImage {
				n := node.(*ast.Image)
				handler := resourcehandlers.Get(contentSourcePath)
				absLink, rewrite := handler.ResolveRelLink(contentSourcePath, string(n.Destination))
				if absLink != "" && !rewrite {
					rdCh <- &ResourceData{
						NodeTargetPath: nodeTargetPath,
						OriginalPath:   string(n.Destination),
						Source:         absLink,
					}
					return ast.WalkContinue, nil
				}
				n.Destination = []byte(absLink)
			}
			if node.Kind() == ast.KindRawHTML {
				n := node.(*ast.RawHTML)
				l := n.Segments.Len()
				for i := 0; i < l; i++ {
					segment := n.Segments.At(i)
					segmentStr := segment.Value(contentBytes)
					match := hrefAttrMatchRegex.Find([]byte(segmentStr))
					if len(match) > 0 {
						url := strings.Split(string(match), "=")[1]
						rdCh <- &ResourceData{
							Source:         url,
							NodeTargetPath: nodeTargetPath,
							OriginalPath:   url,
						}
						continue
					}
					match = srcAttrMatchRegex.Find([]byte(segmentStr))
					if len(match) > 0 {
						url := strings.Split(string(match), "=")[1]
						rdCh <- &ResourceData{
							Source:         url,
							NodeTargetPath: nodeTargetPath,
							OriginalPath:   url,
						}
						continue
					}
				}
				// return ast.WalkSkipChildren, nil
			}
		}
		return ast.WalkContinue, nil
	}); err != nil {
		fmt.Printf("%v", err)
	}

	renderer := markdown.NewRenderer()
	if err := renderer.Render(&b, contentBytes, doc); err != nil {
		return nil, err
	}

	return ioutil.ReadAll(&b)
}
