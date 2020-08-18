package processors

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/Kunde21/markdownfmt/v2/markdown"
	"github.com/gardener/docode/pkg/api"

	// "github.com/gardener/docode/pkg/resourcehandlers"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

var (
	hrefAttrMatchRegex = regexp.MustCompile(`href=["\']?([^"\'>]+)["\']?`)
)

// HugoProcessor is a processor implementation responsible to rewrite links
// on document that use source format (<path>/<name>.md) to destination format
// (<path>/<name> for sites configured for pretty URLs and <path>/<name>.html
// for sites configured for ugly URLs)
type HugoProcessor struct {
	PrettyUrls bool
}

// Process implements Processor#Process
func (f *HugoProcessor) Process(documentBlob []byte, node *api.Node) ([]byte, error) {
	var (
		err error
		b   bytes.Buffer
	)
	p := parser.NewParser(parser.WithBlockParsers(parser.DefaultBlockParsers()...),
		parser.WithInlineParsers(parser.DefaultInlineParsers()...),
		parser.WithParagraphTransformers(parser.DefaultParagraphTransformers()...),
	)
	reader := text.NewReader(documentBlob)
	doc := p.Parse(reader)
	if err = ast.Walk(doc, func(_node ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if _node.Kind() == ast.KindLink {
				n := _node.(*ast.Link)
				n.Destination = rewriteDestination(n.Destination, node)
				return ast.WalkContinue, nil
			}

			if _node.Kind() == ast.KindImage {
				n := _node.(*ast.Image)
				n.Destination = rewriteDestination(n.Destination, node)
				return ast.WalkContinue, nil
			}
			if _node.Kind() == ast.KindRawHTML {
				// ?
				n := _node.(*ast.RawHTML)
				l := n.Segments.Len()
				for i := 0; i < l; i++ {
					segment := n.Segments.At(i)
					segmentStr := segment.Value(documentBlob)
					match := hrefAttrMatchRegex.Find([]byte(segmentStr))
					if len(match) > 0 {
						link := strings.Split(string(match), "=")[1]
						// TODO: handle anchors to md files - <a href="./a/b.md">cross link</a>
						fmt.Printf("%v\n", link)
						continue
					}
				}
			}
		}
		return ast.WalkContinue, nil
	}); err != nil {
		return nil, err
	}

	renderer := markdown.NewRenderer()
	if err := renderer.Render(&b, documentBlob, doc); err != nil {
		return nil, err
	}
	if documentBlob, err = ioutil.ReadAll(&b); err != nil {
		return nil, err
	}
	return documentBlob, nil
}

func rewriteDestination(destination []byte, node *api.Node) []byte {
	if len(destination) == 0 {
		return destination
	}
	link := string(destination)
	link = strings.TrimSpace(link)
	// trim leading and trailing quotes
	link = strings.TrimRight(strings.TrimLeft(link, "\""), "\"")
	if !strings.HasPrefix(link, "https") {
		link = strings.TrimRight(link, ".md")
		fmt.Printf("%s rewriting link: %s  ->  %s\n", node.Name, string(destination), link)
		return []byte(fmt.Sprintf("../%s", link))
	}
	return destination
}
