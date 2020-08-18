package reactor

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/Kunde21/markdownfmt/v2/markdown"
	"github.com/google/uuid"

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

// ContentProcessor ...
type ContentProcessor struct {
	ResourceAbsLink map[string]string
	rwlock          sync.RWMutex
}

func (c *ContentProcessor) GenerateResourceName(path string) string {
	var (
		ok           bool
		resourceName string
	)

	c.rwlock.Lock()
	defer c.rwlock.Unlock()
	if resourceName, ok = c.ResourceAbsLink[path]; !ok {
		separatedSource := strings.Split(path, "/")
		resource := separatedSource[len(separatedSource)-1]
		resourceFileExtension := filepath.Ext(resource)
		resourceName = uuid.New().String() + resourceFileExtension
		c.ResourceAbsLink[path] = resourceName
	}
	return resourceName
}

// HarvestLinks TODO:
func HarvestLinks(docNode *api.Node, contentSourcePath string, nodeTargetPath string, contentBytes []byte, rdCh chan *ResourceData, c *ContentProcessor) ([]byte, error) {
	// TODO: harvest links from this contentBytes
	// and resolve them to downloadable addresses and serialization targets
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
				var destination string
				handler := resourcehandlers.Get(contentSourcePath)
				absLink, err := handler.BuildAbsLink(contentSourcePath, string(n.Destination))
				if err != nil {
					return ast.WalkStop, err
				}

				existingNode := tryFindNode(absLink, docNode)
				if existingNode != nil {
					relPathBetweenNodes := relativePath(docNode, existingNode)
					n.Destination = []byte(relPathBetweenNodes)
					return ast.WalkContinue, nil
				}

				destination = absLink
				if absLink != "" && downloadLinkedResource(contentSourcePath, absLink) {
					resourceName := c.GenerateResourceName(absLink)
					destination = buildDestination(docNode, resourceName)
					rdCh <- &ResourceData{
						OriginalPath:   string(n.Destination),
						Source:         absLink,
						NodeTargetPath: resourceName,
					}
				}
				n.Destination = []byte(destination)
				return ast.WalkContinue, nil
			}
			if node.Kind() == ast.KindImage {
				n := node.(*ast.Image)
				existingNode := tryFindNode(string(n.Destination), docNode)
				if existingNode != nil {
					relPathBetweenNodes := relativePath(docNode, existingNode)
					n.Destination = []byte(relPathBetweenNodes)
					return ast.WalkContinue, nil
				}
				var destination string
				handler := resourcehandlers.Get(contentSourcePath)
				absLink, err := handler.BuildAbsLink(contentSourcePath, string(n.Destination))
				if err != nil {
					return ast.WalkStop, err
				}

				destination = absLink
				if absLink != "" && downloadLinkedResource(contentSourcePath, absLink) {
					resourceName := c.GenerateResourceName(absLink)
					destination = buildDestination(docNode, resourceName)
					rdCh <- &ResourceData{
						OriginalPath:   string(n.Destination),
						Source:         absLink,
						NodeTargetPath: resourceName,
					}
				}
				n.Destination = []byte(destination)
				return ast.WalkContinue, nil
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

	var b bytes.Buffer
	renderer := markdown.NewRenderer()
	if err := renderer.Render(&b, contentBytes, doc); err != nil {
		return nil, err
	}

	content, err := ioutil.ReadAll(&b)
	if err != nil {
		return nil, err
	}

	return content, nil
}

func downloadLinkedResource(contentSourceLink, absLink string) bool {
	separatedContentSourceLink := strings.Split(contentSourceLink, "/")
	var docsPostion int
	for i, path := range separatedContentSourceLink {
		if path == "docs" {
			docsPostion = i
			break
		}
	}

	separatedAbsLink := strings.Split(absLink, "/")

	return len(separatedAbsLink) >= docsPostion+1 && separatedAbsLink[docsPostion] == "docs"
}

func buildDestination(docNode *api.Node, resourceName string) string {
	resourceRelPath := "__resources/" + resourceName
	parentsSize := len(docNode.Parents())
	for ; parentsSize > 0; parentsSize-- {
		resourceRelPath = "../" + resourceRelPath
	}
	return resourceRelPath
}

func tryFindNode(nodeContentSource string, node *api.Node) *api.Node {
	if node == nil {
		return nil
	}

	for _, contentSelector := range node.ContentSelectors {
		if contentSelector.Source == nodeContentSource {
			return node
		}
	}

	return withMatchinContentSelectorSource(nodeContentSource, getRootNode(node))
}

func withMatchinContentSelectorSource(nodeContentSource string, node *api.Node) *api.Node {
	for _, contentSelector := range node.ContentSelectors {
		if contentSelector.Source == nodeContentSource {
			return node
		}
	}

	for i := range node.Nodes {
		foundNode := withMatchinContentSelectorSource(nodeContentSource, node.Nodes[i])
		if foundNode != nil {
			return foundNode
		}
	}

	return nil
}

func getRootNode(node *api.Node) *api.Node {
	if node == nil {
		return nil
	}

	parentNodes := node.Parents()
	return parentNodes[0]
}
