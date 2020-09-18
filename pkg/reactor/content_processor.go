package reactor

import (
	"bytes"
	"context"
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
	"github.com/hashicorp/go-multierror"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

var (
	htmlLinksRegexList = []*regexp.Regexp{
		regexp.MustCompile(`href=["\']?([^"\'>]+)["\']?`),
		regexp.MustCompile(`src=["\']?([^"\'>]+)["\']?`),
	}
	mdLinksRegex = regexp.MustCompile(`\[(?P<text>.+)\]\((?P<url>[^ ]+)(?: "(?P<title>.+)")?\)`)
	mdParser     = parser.NewParser(parser.WithBlockParsers(parser.DefaultBlockParsers()...),
		parser.WithInlineParsers(parser.DefaultInlineParsers()...),
		parser.WithParagraphTransformers(parser.DefaultParagraphTransformers()...),
	)
)

// NodeContentProcessor operates on documents content to reconcile links and
// schedule linked resources downloads
type NodeContentProcessor struct {
	resourceAbsLinks map[string]string
	rwlock           sync.RWMutex
	LocalityDomain   LocalityDomain
	// ResourcesRoot specifies the root location for downloaded resource.
	// It is used to rewrite resource links in documents to relative paths.
	resourcesRoot string
	DownloadJob   DownloadJob
	failFast      bool
	markdownFmt   bool
}

// NewNodeContentProcessor creates NodeContentProcessor objects
func NewNodeContentProcessor(resourcesRoot string, localityDomain LocalityDomain, downloadJob DownloadJob, failFast bool, markdownFmt bool) *NodeContentProcessor {
	if localityDomain == nil {
		localityDomain = LocalityDomain{}
	}
	c := &NodeContentProcessor{
		resourceAbsLinks: make(map[string]string),
		LocalityDomain:   localityDomain,
		resourcesRoot:    resourcesRoot,
		DownloadJob:      downloadJob,
		failFast:         failFast,
		markdownFmt:      markdownFmt,
	}
	return c
}

//convenience wrapper adding logging
func (c *NodeContentProcessor) schedule(ctx context.Context, link, resourceName, from string) {
	fmt.Printf("[%s] Linked resource scheduled for download: %s\n", from, link)
	c.DownloadJob.Schedule(ctx, link, resourceName)
}

// ReconcileLinks analyzes a document referenced by a node's contentSourcePath
// and processes its links to other resources to resolve their inconsistencies.
// The processing might involve rewriting links to relative and having new
// destinations, or rewriting them to absolute, as well as downloading some of
// the linked resources.
// The function returns the processed document or error.
func (c *NodeContentProcessor) ReconcileLinks(ctx context.Context, node *api.Node, contentSourcePath string, contentBytes []byte) ([]byte, error) {
	fmt.Printf("[%s] Reconciling links for %s\n", node.Name, contentSourcePath)
	documentBytes, err := c.reconcileMDLinks(ctx, node, contentBytes, contentSourcePath)
	if err != nil {
		return nil, err
	}
	if _, err := c.reconcileHTMLLinks(ctx, node, documentBytes, contentSourcePath); err != nil {
		return nil, err
	}
	return documentBytes, err
}

func (c *NodeContentProcessor) reconcileMDLinks(ctx context.Context, docNode *api.Node, contentBytes []byte, contentSourcePath string) ([]byte, error) {
	if !c.markdownFmt {
		var errors *multierror.Error
		contentBytes = mdLinksRegex.ReplaceAllFunc(contentBytes, func(match []byte) []byte {
			var title string
			link := strings.Split(string(match), "](")
			text := link[0] + "]"          // [text]
			d := link[1]                   // url title)
			d = strings.TrimSuffix(d, ")") // url title
			_d := strings.Split(d, "\"")
			_u := _d[0] // url
			if len(_d) > 1 {
				title = "\"%s" + _d[1] //title
			}

			destination, downloadURL, resourceName, err := c.processLink(ctx, docNode, _u, contentSourcePath)
			fmt.Printf("[%s] %s -> %s\n", contentSourcePath, _u, destination)
			if len(downloadURL) > 0 {
				c.schedule(ctx, downloadURL, resourceName, contentSourcePath)
			}
			if err != nil {
				errors = multierror.Append(err)
				return match
			}
			if len(title) > 0 {
				return []byte(fmt.Sprintf("%s(%s %s)", text, destination, title))
			}
			return []byte(fmt.Sprintf("%s(%s)", text, destination))
		})
		return contentBytes, errors.ErrorOrNil()
	}

	reader := text.NewReader(contentBytes)
	doc := mdParser.Parse(reader)
	var errors *multierror.Error
	if err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			var (
				destination  string
				downloadLink string
				resourceName string
				err          error
			)
			switch node.Kind() {
			case ast.KindLink:
				{
					n := node.(*ast.Link)
					if destination, downloadLink, resourceName, err = c.processLink(ctx, docNode, string(n.Destination), contentSourcePath); err != nil {
						return ast.WalkContinue, err
					}
					n.Destination = []byte(destination)
				}
			case ast.KindImage:
				{
					n := node.(*ast.Image)
					if destination, downloadLink, resourceName, err = c.processLink(ctx, docNode, string(n.Destination), contentSourcePath); err != nil {
						return ast.WalkContinue, err
					}
					n.Destination = []byte(destination)
				}
			}
			if len(downloadLink) > 0 {
				c.schedule(ctx, downloadLink, resourceName, contentSourcePath)
			}
		}
		return ast.WalkContinue, nil
	}); err != nil {
		if c.failFast {
			return nil, err
		}
		errors = multierror.Append(err)
	}

	var b bytes.Buffer
	renderer := markdown.NewRenderer()
	if err := renderer.Render(&b, contentBytes, doc); err != nil {
		return nil, multierror.Append(err)
	}

	documentBytes, err := ioutil.ReadAll(&b)
	if err != nil {
		return nil, multierror.Append(err)
	}

	return documentBytes, errors.ErrorOrNil()
}

// replace html raw links of any sorts.
func (c *NodeContentProcessor) reconcileHTMLLinks(ctx context.Context, docNode *api.Node, documentBytes []byte, contentSourcePath string) ([]byte, error) {
	var errors *multierror.Error
	for _, regex := range htmlLinksRegexList {
		documentBytes = regex.ReplaceAllFunc(documentBytes, func(match []byte) []byte {
			attr := strings.Split(string(match), "=")
			name := attr[0]
			url := attr[1]
			if len(url) > 0 {
				url = strings.TrimPrefix(url, "\"")
				url = strings.TrimSuffix(url, "\"")
			}
			destination, downloadURL, resourceName, err := c.processLink(ctx, docNode, url, contentSourcePath)
			fmt.Printf("[%s] %s -> %s\n", contentSourcePath, url, destination)
			if len(downloadURL) > 0 {
				c.schedule(ctx, downloadURL, resourceName, contentSourcePath)
			}
			if err != nil {
				errors = multierror.Append(err)
				return match
			}
			return []byte(fmt.Sprintf("%s=%s", name, destination))
		})
	}
	return documentBytes, errors.ErrorOrNil()
}

func (c *NodeContentProcessor) processLink(ctx context.Context, node *api.Node, destination string, contentSourcePath string) (string, string, string, error) {
	if strings.HasPrefix(destination, "#") {
		return destination, "", "", nil
	}

	handler := resourcehandlers.Get(contentSourcePath)
	if handler == nil {
		return destination, "", "", nil
	}
	absLink, err := handler.BuildAbsLink(contentSourcePath, destination)
	if err != nil {
		return "", "", "", err
	}

	u, err := url.Parse(absLink)
	if err != nil {
		return "", "", "", err
	}
	if strings.HasSuffix(u.Path, ".md") {
		//TODO: this is URI-specific - fixme
		l := strings.TrimRight(absLink, "?")
		l = strings.TrimRight(l, "#")
		existingNode := api.FindNodeByContentSource(l, node)
		if existingNode != nil {
			relPathBetweenNodes := node.RelativePath(existingNode)
			if destination != relPathBetweenNodes {
				fmt.Printf("[%s] %s -> %s\n", contentSourcePath, destination, relPathBetweenNodes)
			}
			destination = relPathBetweenNodes
			return destination, "", "", nil
		}
		if destination != absLink {
			fmt.Printf("[%s] %s -> %s\n", contentSourcePath, destination, absLink)
		}
		return absLink, "", "", nil
	}

	rh := resourcehandlers.Get(absLink)
	if rh == nil {
		if absLink != destination {
			fmt.Printf("[%s] No resource hanlder for %s found. No changes to %s\n", contentSourcePath, absLink, destination)
		}
		return destination, "", "", nil
	}

	key, path, err := rh.GetLocalityDomainCandidate(absLink)
	if err != nil {
		return "", "", "", err
	}
	if absLink != "" && c.LocalityDomain.PathInLocality(key, path) {
		resourceName := c.generateResourceName(absLink)
		_d := destination
		destination = buildDestination(node, resourceName, c.resourcesRoot)
		if _d != destination {
			fmt.Printf("[%s] %s -> %s\n", contentSourcePath, _d, destination)
		}
		return destination, absLink, resourceName, nil
	}
	if destination != absLink {
		fmt.Printf("[%s] %s -> %s\n", contentSourcePath, destination, absLink)
	}
	return absLink, "", "", nil
}

// Builds destination path for links from node to resource in root path
// If root is not specified as document root (with leading "/"), the
// returned destinations are relative paths from the node to the resource
// in root, e.g. "../../__resources/image.png", where root is "__resources".
// If root is document root path, destinations are paths from the root,
// e.g. "/__resources/image.png", where root is "/__resources".
func buildDestination(node *api.Node, resourceName, root string) string {
	if strings.HasPrefix(root, "/") {
		return root + "/" + resourceName
	}
	resourceRelPath := fmt.Sprintf("%s/%s", root, resourceName)
	parentsSize := len(node.Parents())
	for ; parentsSize > 0; parentsSize-- {
		resourceRelPath = "../" + resourceRelPath
	}
	return resourceRelPath
}

func (c *NodeContentProcessor) generateResourceName(path string) string {
	var (
		ok           bool
		resourceName string
	)

	c.rwlock.Lock()
	defer c.rwlock.Unlock()
	if resourceName, ok = c.resourceAbsLinks[path]; !ok {
		separatedSource := strings.Split(path, "/")
		resource := separatedSource[len(separatedSource)-1]
		resourceFileExtension := filepath.Ext(resource)
		resourceName = uuid.New().String() + resourceFileExtension
		c.resourceAbsLinks[path] = resourceName
	}
	return resourceName
}
