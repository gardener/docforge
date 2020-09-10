package reactor

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/url"
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
			// ReconcileLinks(cs.Source, contentBytes, nil)
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
	LocalityDomain  LocalityDomain
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

// ReconcileLinks analyzes a document referenced by a node's contentSourcePath
// and processes its links to other resources to resolve their inconsistencies.
// The processing might involve rewriting links to relative and having new 
// destinations, or rewriting them to absolute, as well as downloading some of
// the linked resources.
// The function returns the processed document or error.
func ReconcileLinks(docNode *api.Node, contentSourcePath string, contentBytes []byte, c *ContentProcessor, rdCh chan *ResourceData, failFast bool) ([]byte, error) {
	p := parser.NewParser(parser.WithBlockParsers(parser.DefaultBlockParsers()...),
		parser.WithInlineParsers(parser.DefaultInlineParsers()...),
		parser.WithParagraphTransformers(parser.DefaultParagraphTransformers()...),
	)
	reader := text.NewReader(contentBytes)
	doc := p.Parse(reader)
	if err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			var (
				destination string
				err error
			)
			switch node.Kind() {
				case ast.KindLink: {
					n := node.(*ast.Link)
					if destination, err = c.processLink(docNode, string(n.Destination), contentSourcePath, rdCh); err!=nil {
						return ast.WalkContinue, err
					}
					n.Destination = []byte(destination)
					return ast.WalkContinue, nil
				}
				case ast.KindImage: {
					n := node.(*ast.Image)
					if destination, err = c.processLink(docNode, string(n.Destination), contentSourcePath, rdCh); err!=nil {
						return ast.WalkContinue, err
					}
					n.Destination = []byte(destination)
					return ast.WalkContinue, nil
				}
			}
			// Note, AutoLinks are always absolute. No rewrite is necessary
			// There's really no good way to rewrite HTML in ast tree model
			// if node.Kind() == ast.KindBlockHTML {
			// }
			// if node.Kind() == ast.KindRawHTML {
			// 	n := node.(*ast.RawHTML)
			// 	l := n.Segments.Len()
			// 	for i := 0; i < l; i++ {
			// 		segment := n.Segments.At(i)
			// 		segmentStr := segment.Value(contentBytes)
			// 		match := hrefAttrMatchRegex.Find([]byte(segmentStr))
			// 		if len(match) > 0 {
			// 			url := strings.Split(string(match), "=")[1]
			// 			if destination, err = processLink(docNode, url, contentSourcePath, rdCh); err!=nil {
			// 				return ast.WalkContinue, err
			// 			}

			// 			continue
			// 		}
			// 		match = srcAttrMatchRegex.Find([]byte(segmentStr))
			// 		if len(match) > 0 {
			// 			url := strings.Split(string(match), "=")[1]
			// 			if destination, err = processLink(docNode, url, contentSourcePath, rdCh); err!=nil {
			// 				return ast.WalkContinue, err
			// 			}
			// 			continue
			// 		}
			// 	}
			// 	// return ast.WalkSkipChildren, nil
			// }
		}
		return ast.WalkContinue, nil
	}); err != nil {
		if failFast {
			return nil, err
		}
		// TODO: use multierror
		fmt.Printf("%v", err)
	}

	var b bytes.Buffer
	renderer := markdown.NewRenderer()
	if err := renderer.Render(&b, contentBytes, doc); err != nil {
		return nil, err
	}

	documentBytes, err := ioutil.ReadAll(&b)
	if err!=nil{
		return nil, err
	}
		
	// replace html raw links of any sorts.
	htmlLinksRegexList := []*regexp.Regexp{
		regexp.MustCompile(`href=["\']?([^"\'>]+)["\']?`), 
		regexp.MustCompile(`src=["\']?([^"\'>]+)["\']?`),
	}
	for _, regex:= range htmlLinksRegexList {
		documentBytes = regex.ReplaceAllFunc(documentBytes, func(match []byte) []byte {
			attr := strings.Split(string(match), "=")
			name:= attr[0]
			url := attr[1]
			destination, err := c.processLink(docNode, url, contentSourcePath, rdCh)
			if err!=nil {
				//TODO: error handling
				fmt.Printf("Link processing failed %v\n", err)
				return match
			}
			fmt.Printf("Link destination %s\n", destination)
			return []byte(fmt.Sprintf("%s=%s", name, destination))
		}) 	
	}

	return documentBytes, err
}

func (c *ContentProcessor) processLink(node *api.Node, destination string, contentSourcePath string, rdCh chan *ResourceData) (string, error) {
	if strings.HasPrefix(destination, "#") {
		return destination, nil
	}

	handler := resourcehandlers.Get(contentSourcePath)
	absLink, err := handler.BuildAbsLink(contentSourcePath, destination)
	if err != nil {
		return "", err
	}

	u, err := url.Parse(absLink)
	if err!=nil {
		return "", err
	}
	if strings.HasSuffix(u.Path, ".md") {
		//TODO: this is URI-specific - fixme
		l := strings.TrimRight(absLink, "?")
		l = strings.TrimRight(l, "#")
		existingNode := tryFindNode(l, node)
		if existingNode != nil {
			relPathBetweenNodes := node.RelativePath(existingNode)
			if destination!=relPathBetweenNodes{
				fmt.Printf("[%s] %s -> %s\n", contentSourcePath, destination, relPathBetweenNodes)
			}
			destination = relPathBetweenNodes
			return destination, nil
		}
		if destination!=absLink{
			fmt.Printf("[%s] %s -> %s\n", contentSourcePath, destination, absLink)
		}
		return absLink, nil
	}

	rh := resourcehandlers.Get(absLink)
	if rh == nil {
		if absLink!=destination {
			fmt.Printf("[%s] No resource hanlder for %s found. No changes to %s\n", contentSourcePath, absLink, destination)
		}
		return destination, nil
	}

	key, path, err := rh.GetLocalityDomainCandidate(absLink)
	if err != nil {
		return "", err
	}
	if absLink != "" && c.LocalityDomain.PathInLocality(key, path) {
		resourceName := c.GenerateResourceName(absLink)
		_d := destination
		destination = buildDestination(node, resourceName)
		fmt.Printf("[%s] Linked resource scheduled for download: %s\n", contentSourcePath, absLink)
		rdCh <- &ResourceData{
			Source:         absLink,
			NodeTargetPath: resourceName,
		}
		if _d!=destination{
			fmt.Printf("[%s] %s -> %s\n",contentSourcePath, _d, destination)
		}
		return destination, nil
	}
	if destination!=absLink {
		fmt.Printf("[%s] %s -> %s\n", contentSourcePath, destination, absLink)
	}
	return absLink, nil
}

func buildDestination(node *api.Node, resourceName string) string {
	resourceRelPath := "__resources/" + resourceName
	parentsSize := len(node.Parents())
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
	if node == nil {
		return nil
	}
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
	if len(parentNodes) <= 0 {
		return nil
	}
	return parentNodes[0]
}
