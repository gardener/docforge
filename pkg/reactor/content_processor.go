// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reactor

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/markdown"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"k8s.io/klog/v2"
	"net/url"
	"path"
	"strings"
	"sync"
)

var (
	// pool with reusable buffers
	bufPool = sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
		},
	}
)

type nodeContentProcessor struct {
	resourcesRoot    string
	downloader       DownloadScheduler
	validator        Validator
	resourceHandlers resourcehandlers.Registry
	sourceLocations  map[string][]*api.Node
	hugo             *Hugo
	rwLock           sync.RWMutex
}

// extends nodeContentProcessor with current source URI & node
type linkResolver struct {
	*nodeContentProcessor
	node   *api.Node
	source string
}

// linkInfo defines a markdown link
type linkInfo struct {
	URL                 *url.URL
	originalDestination string
	destination         string
	destinationNode     *api.Node
	isEmbeddable        bool
}

// docContent defines a document content
type docContent struct {
	docAst ast.Node
	docCnt []byte
	docURI string
}

// used in Hugo mode
type frontmatterProcessor struct {
	node           *api.Node
	IndexFileNames []string
}

// NodeContentProcessor operates on documents content to reconcile links and schedule linked resources downloads
//counterfeiter:generate . NodeContentProcessor
type NodeContentProcessor interface {
	// Prepare performs pre-processing on resolved documentation structure (e.g. collect api.Node sources)
	Prepare(docStructure []*api.Node)
	// Process node content and write the result in a buffer
	Process(ctx context.Context, buffer *bytes.Buffer, reader Reader, node *api.Node) error
}

// NewNodeContentProcessor creates NodeContentProcessor objects
func NewNodeContentProcessor(resourcesRoot string, downloadJob DownloadScheduler, validator Validator, rh resourcehandlers.Registry, hugo *Hugo) NodeContentProcessor {
	c := &nodeContentProcessor{
		// resourcesRoot specifies the root location for downloaded resource.
		// It is used to rewrite resource links in documents to relative paths.
		resourcesRoot:    resourcesRoot,
		downloader:       downloadJob,
		validator:        validator,
		resourceHandlers: rh,
		hugo:             hugo,
		sourceLocations:  make(map[string][]*api.Node),
	}
	return c
}

///////////// node content processor ///////

func (c *nodeContentProcessor) Prepare(structure []*api.Node) {
	c.rwLock.Lock()
	defer c.rwLock.Unlock()
	for _, node := range structure {
		c.addSourceLocation(node)
	}
}

func (c *nodeContentProcessor) Process(ctx context.Context, b *bytes.Buffer, r Reader, n *api.Node) error {
	// api.Node content by priority
	var nc []*docContent
	nFullName := n.FullName("/")
	// 1. Process Source
	if len(n.Source) > 0 {
		if dc := getCachedContent(n); dc != nil {
			nc = append(nc, dc)
		} else {
			source, err := r.Read(ctx, n.Source)
			if err != nil {
				if resourceNotFound, ok := err.(resourcehandlers.ErrResourceNotFound); ok {
					klog.Warningf("reading source %s from node %s failed: %s\n", n.Source, nFullName, resourceNotFound)
				} else {
					return fmt.Errorf("reading source %s from node %s failed: %w", n.Source, nFullName, err)
				}
			}
			if len(source) > 0 {
				dc = &docContent{docCnt: source, docURI: n.Source}
				dc.docAst, err = markdown.Parse(source)
				if err != nil {
					return fmt.Errorf("fail to parse source %s from node %s: %w", n.Source, nFullName, err)
				}
				nc = append(nc, dc)
			} else if err == nil {
				klog.Warningf("no content read from node %s source %s\n", nFullName, n.Source)
			}
		}
	}
	// 2. Process MultiSource
	if len(n.MultiSource) > 0 {
		for i, src := range n.MultiSource {
			source, err := r.Read(ctx, src)
			if err != nil {
				if resourceNotFound, ok := err.(resourcehandlers.ErrResourceNotFound); ok {
					klog.Warningf("reading multiSource[%d] %s from node %s failed: %s\n", i, src, nFullName, resourceNotFound)
				} else {
					return fmt.Errorf("reading multiSource[%d] %s from node %s failed: %w", i, src, nFullName, err)
				}
			}
			if len(source) > 0 {
				dc := &docContent{docCnt: source, docURI: src}
				dc.docAst, err = markdown.Parse(source)
				if err != nil {
					return fmt.Errorf("fail to parse multiSource[%d] %s from node %s: %w", i, src, nFullName, err)
				}
				nc = append(nc, dc)
			} else if err == nil {
				klog.Warningf("no content read from node %s multiSource[%d] %s\n", nFullName, i, src)
			}
		}
	}
	// if no content -> return
	if len(nc) == 0 {
		klog.Warningf("empty content for node %s\n", nFullName)
		return nil
	}
	// render node content
	// 1 - frontmatter preprocessing
	var fmp *frontmatterProcessor
	if c.hugo.Enabled {
		fmp = &frontmatterProcessor{
			node:           n,
			IndexFileNames: c.hugo.IndexFileNames,
		}
	}
	if err := preprocessFrontmatter(nc, fmp); err != nil {
		return err
	}
	// 2. - write node content
	for _, cnt := range nc {
		rnd := c.getRenderer(n, cnt.docURI)
		if err := rnd.Render(b, cnt.docCnt, cnt.docAst); err != nil {
			return err
		}
	}
	return nil
}

func (c *nodeContentProcessor) addSourceLocation(node *api.Node) {
	if node.Source != "" {
		c.sourceLocations[node.Source] = append(c.sourceLocations[node.Source], node)
	} else if len(node.MultiSource) > 0 {
		for _, s := range node.MultiSource {
			c.sourceLocations[s] = append(c.sourceLocations[s], node)
		}
	} else if len(node.Properties) > 0 {
		if val, found := node.Properties[api.ContainerNodeSourceLocation]; found {
			if sl, ok := val.(string); ok {
				c.sourceLocations[sl] = append(c.sourceLocations[sl], node)
				delete(node.Properties, api.ContainerNodeSourceLocation)
			}
		}
	}
	for _, childNode := range node.Nodes {
		c.addSourceLocation(childNode)
	}
}

func (c *nodeContentProcessor) getRenderer(n *api.Node, sourceURI string) renderer.Renderer {
	lr := c.newLinkResolver(n, sourceURI)
	return markdown.NewLinkModifierRenderer(markdown.WithLinkResolver(lr.resolveLink))
}

func (c *nodeContentProcessor) newLinkResolver(node *api.Node, sourceURI string) *linkResolver {
	return &linkResolver{
		nodeContentProcessor: c,
		node:                 node,
		source:               sourceURI,
	}
}

func getCachedContent(n *api.Node) *docContent {
	if len(n.Properties) > 0 {
		if val, found := n.Properties[api.CachedNodeContent]; found {
			if dc, ok := val.(*docContent); ok {
				delete(n.Properties, api.CachedNodeContent)
				return dc
			}
		}
	}
	return nil
}

func preprocessFrontmatter(nc []*docContent, fmp *frontmatterProcessor) error {
	if len(nc) > 1 {
		aggregated := make(map[string]interface{})
		for i := len(nc) - 1; i >= 0; i-- {
			if nc[i].docAst.Kind() == ast.KindDocument {
				d := nc[i].docAst.(*ast.Document)
				mergeFrontmatter(aggregated, d.Meta())
				d.SetMeta(nil)
			}
		}
		if nc[0].docAst.Kind() == ast.KindDocument {
			nc[0].docAst.(*ast.Document).SetMeta(aggregated)
		} else {
			return fmt.Errorf("expect ast kind %s, but get %s", ast.KindDocument, nc[0].docAst.Kind())
		}
	}
	if fmp != nil && len(nc) > 0 && nc[0].docAst.Kind() == ast.KindDocument {
		d := nc[0].docAst.(*ast.Document)
		pfm, err := fmp.processFrontmatter(d.Meta())
		if err != nil {
			return err
		}
		d.SetMeta(pfm)
	}
	return nil
}

// write add key:values over base
func mergeFrontmatter(base, add map[string]interface{}) {
	for k, v := range add {
		base[k] = v
	}
}

///////////// link resolver ////////////////

// implements markdown.ResolveLink
func (l *linkResolver) resolveLink(dest string, isEmbeddable bool) (string, error) {
	// validate destination
	u, err := url.Parse(strings.TrimSuffix(dest, "/"))
	if err != nil {
		return "", err
	}
	link := &linkInfo{
		URL:                 u,
		originalDestination: dest,
		destination:         dest,
		isEmbeddable:        isEmbeddable,
	}
	if err = l.resolveBaseLink(link); err != nil {
		return "", err
	}
	if isEmbeddable {
		if err = l.rawLink(link); err != nil {
			return link.destination, err
		}
	}
	if l.hugo.Enabled {
		err = l.rewriteDestination(link)
	}
	return link.destination, err
}

// resolve base link
func (l *linkResolver) resolveBaseLink(link *linkInfo) error {
	var err error
	if strings.HasPrefix(link.destination, "#") || strings.HasPrefix(link.destination, "mailto:") {
		return nil
	}
	// build absolute link
	var absLink string
	if link.URL.IsAbs() {
		// can we handle changes to this destination?
		if l.resourceHandlers.Get(link.destination) == nil {
			// we don't have a handler for it. Leave it be.
			l.validator.ValidateLink(link.URL, link.destination, l.source)
			return nil
		}
		absLink = link.destination
	} else {
		handler := l.resourceHandlers.Get(l.source) // handler must exist because source content has been read
		// build absolute path for the destination using content source path as base
		if absLink, err = handler.BuildAbsLink(l.source, link.destination); err != nil {
			if _, ok := err.(resourcehandlers.ErrResourceNotFound); ok {
				klog.Warningf("failed to validate absolute link for %s from source %s: %v\n", link.destination, l.source, err)
				err = nil
				if link.destination != absLink {
					klog.V(6).Infof("[%s] %s -> %s\n", l.source, link.destination, absLink)
					link.destination = absLink
				}
			}
			return err
		}
	}
	absURL, _ := url.Parse(absLink) // absLink should be valid URL
	key := fmt.Sprintf("%s://%s%s", absURL.Scheme, absURL.Host, absURL.Path)
	// Links to other documents are enforced relative when linking documents from the node structure.
	if nl, ok := l.getNodesBySource(strings.TrimSuffix(key, "/")); ok {
		// found nodes with this source -> find the shortest path from l.node to one of nodes
		nPath := ""
		for _, n := range nl {
			n = findVisibleNode(n)
			if n != nil {
				relPathBetweenNodes := l.node.RelativePath(n)
				if swapPaths(nPath, relPathBetweenNodes) {
					nPath = relPathBetweenNodes
					link.destinationNode = n
				}
			}
		}
		if link.destinationNode != nil { // i.e. visible destination node found
			if link.URL.ForceQuery || link.URL.RawQuery != "" {
				nPath = fmt.Sprintf("%s?%s", nPath, link.URL.RawQuery)
			}
			if link.URL.Fragment != "" {
				nPath = fmt.Sprintf("%s#%s", nPath, link.URL.Fragment)
			}
			if link.destination != nPath {
				klog.V(6).Infof("[%s] %s -> %s\n", l.source, link.destination, nPath)
				link.destination = nPath
			}
			return nil
		}
	}
	// Links to resources that are not structure document nodes are scheduled for download and their destination is updated to relative path to predefined location for resources.
	if link.isEmbeddable && downloadEmbeddable(link.URL) {
		hash := md5.Sum([]byte(key)) // hash based on absolute link, but without query & fragment to avoid duplications
		ext := path.Ext(link.URL.Path)
		downloadResourceName := "$name_$hash$ext"
		downloadResourceName = strings.ReplaceAll(downloadResourceName, "$name", strings.TrimSuffix(path.Base(link.URL.Path), ext))
		downloadResourceName = strings.ReplaceAll(downloadResourceName, "$hash", hex.EncodeToString(hash[:])[:6])
		downloadResourceName = strings.ReplaceAll(downloadResourceName, "$ext", ext)
		resLocation := buildDownloadDestination(l.node, downloadResourceName, l.resourcesRoot)
		if link.destination != resLocation {
			klog.V(6).Infof("[%s] %s -> %s\n", l.source, link.destination, resLocation)
			link.destination = resLocation
		}
		if err = l.downloader.Schedule(&DownloadTask{
			absLink,
			downloadResourceName,
			l.source,
			link.destination,
		}); err != nil {
			return err
		}
		return nil
	}
	// Rewrite with absolute link
	if link.destination != absLink {
		klog.V(6).Infof("[%s] %s -> %s\n", l.source, link.destination, absLink)
		link.destination = absLink
	}
	l.validator.ValidateLink(absURL, link.destination, l.source)
	return nil
}

// rewrite abs links to embedded objects to their raw link format if necessary, to ensure they are embeddable
func (l *linkResolver) rawLink(link *linkInfo) error {
	u, err := url.Parse(link.destination)
	if err != nil {
		return err
	}
	if !u.IsAbs() {
		return nil
	}
	handler := l.resourceHandlers.Get(link.destination)
	if handler == nil {
		return nil // not a GitHub resource
	}
	var rawLink string
	if rawLink, err = handler.GetRawFormatLink(link.destination); err != nil {
		return err
	}
	if link.destination != rawLink {
		klog.V(6).Infof("[%s] %s -> %s\n", l.source, link.destination, rawLink)
		link.destination = rawLink
	}
	return nil
}

// rewrite destination in HUGO mode
func (l *linkResolver) rewriteDestination(link *linkInfo) error {
	u, err := url.Parse(link.destination)
	if err != nil {
		return err
	}
	if !u.IsAbs() && !strings.HasPrefix(link.destination, "/") && !strings.HasPrefix(link.destination, "#") {
		base := l.hugo.BaseURL
		if base != "" {
			base = strings.TrimSuffix(base, "/")
			if !strings.HasPrefix(base, "/") {
				base = fmt.Sprintf("/%s", base)
			}
		}
		if link.destinationNode != nil {
			dnPath := strings.ToLower(link.destinationNode.FullName("/"))
			// prepare HUGO Pretty URLs - https://gohugo.io/content-management/urls/#pretty-urls
			if strings.HasSuffix(strings.ToLower(dnPath), ".md") {
				dnPath = dnPath[:len(dnPath)-3]
			}
			dnPath = strings.TrimSuffix(dnPath, "_index")
			if !strings.HasSuffix(dnPath, "/") {
				dnPath = fmt.Sprintf("%s/", dnPath)
			}
			if !l.hugo.PrettyURLs { // https://gohugo.io/content-management/urls/#ugly-urls
				dnPath = fmt.Sprintf("%s.html", strings.TrimSuffix(dnPath, "/"))
			}
			// check for hugo url property & rewrite the link; see https://gohugo.io/content-management/urls/ for details
			if val, ok := link.destinationNode.Properties["frontmatter"]; ok {
				if fmProps, cast := val.(map[string]interface{}); cast {
					if val, ok = fmProps["url"]; ok {
						var urlStr string
						if urlStr, cast = val.(string); cast {
							if _, err = url.Parse(urlStr); err != nil {
								klog.Warningf("Invalid frontmatter url: %s for %s\n", urlStr, link.destinationNode.Source)
							} else {
								dnPath = urlStr
							}
						}
					}
				}
			}
			dnPath = fmt.Sprintf("%s/%s", base, strings.TrimPrefix(dnPath, "/")) // adding base
			if u.ForceQuery || u.RawQuery != "" {
				dnPath = fmt.Sprintf("%s?%s", dnPath, u.RawQuery)
			}
			if u.Fragment != "" {
				dnPath = fmt.Sprintf("%s#%s", dnPath, u.Fragment)
			}
			if link.destination != dnPath {
				klog.V(6).Infof("[%s] %s -> %s\n", l.source, link.destination, dnPath)
				link.destination = dnPath
			}
		} else if link.isEmbeddable {
			ePath := link.destination
			for strings.HasPrefix(ePath, "../") {
				ePath = strings.TrimPrefix(ePath, "../")
			}
			ePath = fmt.Sprintf("%s/%s", l.hugo.BaseURL, ePath)
			if link.destination != ePath {
				klog.V(6).Infof("[%s] %s -> %s\n", l.source, link.destination, ePath)
				link.destination = ePath
			}
		}
	}
	return nil
}

func (l *linkResolver) getNodesBySource(source string) ([]*api.Node, bool) {
	l.rwLock.RLock()
	defer l.rwLock.RUnlock()
	nl, ok := l.sourceLocations[source]
	return nl, ok
}

func downloadEmbeddable(u *url.URL) bool {
	if !u.IsAbs() { // download all relative embeddable links
		return true
	}
	// if embeddable link is absolute, download only if belongs to internal GitHub or own organization
	// TODO: make it configurable
	if u.Host == "github.tools.sap" || u.Host == "raw.github.tools.sap" || u.Host == "github.wdf.sap.corp" {
		return true
	}
	if strings.HasPrefix(u.Path, "/gardener/") {
		return u.Host == "github.com" || u.Host == "raw.githubusercontent.com"
	}
	return false
}

// findVisibleNode returns
// - the node if it is a document api.Node
// - first container node that contains index file if the api.Node is container
// - nil if no container node with index file found
// otherwise link will display empty page
func findVisibleNode(n *api.Node) *api.Node {
	if n == nil || n.IsDocument() {
		return n
	}
	for _, ch := range n.Nodes {
		if ch.Name == "_index.md" {
			return n
		}
	}
	return findVisibleNode(n.Parent())
}

func swapPaths(path string, newPath string) bool {
	if path == "" {
		return true
	}
	// prefer descending vs ascending paths
	if !strings.HasPrefix(path, "./") && strings.HasPrefix(newPath, "./") {
		return true
	}
	// prefer shorter paths
	return strings.Count(path, "/") > strings.Count(newPath, "/")
}

// Builds destination path for links from node to resource in root path
// If root is not specified as document root (with leading "/"), the
// returned destinations are relative paths from the node to the resource
// in root, e.g. "../../__resources/image.png", where root is "__resources".
// If root is document root path, destinations are paths from the root,
// e.g. "/__resources/image.png", where root is "/__resources".
func buildDownloadDestination(node *api.Node, resourceName, root string) string {
	if strings.HasPrefix(root, "/") {
		return root + "/" + resourceName
	}
	resourceRelPath := fmt.Sprintf("%s/%s", root, resourceName)
	parentsSize := len(node.Parents())
	for ; parentsSize > 1; parentsSize-- {
		resourceRelPath = "../" + resourceRelPath
	}
	return resourceRelPath
}

///////////// frontmatter processor ////////

func (f *frontmatterProcessor) processFrontmatter(docFrontmatter map[string]interface{}) (map[string]interface{}, error) {
	var nodeMeta, parentMeta map[string]interface{}
	// 1 front matter from doc node
	if val, ok := f.node.Properties["frontmatter"]; ok {
		nodeMeta, ok = val.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid frontmatter properties for node: %s", f.node.FullName("/"))
		}
	}
	// 2 front matter from doc node parent (only if the current one is section file)
	if f.node.Name == "_index.md" && f.node.Parent() != nil {
		if val, ok := f.node.Parent().Properties["frontmatter"]; ok {
			parentMeta, ok = val.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid frontmatter properties for node: %s", f.node.Path("/"))
			}
		}
	}
	// overwrite docFrontmatter with node
	for k, v := range nodeMeta {
		docFrontmatter[k] = v
	}
	// overwrite docFrontmatter with parent
	for k, v := range parentMeta {
		docFrontmatter[k] = v
	}
	// add Title if missing
	if _, ok := docFrontmatter["title"]; !ok {
		docFrontmatter["title"] = f.getNodeTitle()
	}
	return docFrontmatter, nil
}

// Determines node title from its name or its parent name if
// it is eligible to be index file, and then normalizes either
// as a title - removing `-`, `_`, `.md` and converting to title
// case.
func (f *frontmatterProcessor) getNodeTitle() string {
	title := f.node.Name
	if f.node.Parent() != nil && f.nodeIsIndexFile(f.node.Name) {
		title = f.node.Parent().Name
	}
	title = strings.TrimRight(title, ".md")
	title = strings.ReplaceAll(title, "_", " ")
	title = strings.ReplaceAll(title, "-", " ")
	title = strings.Title(title)
	return title
}

// Compares a node name to the configured list of index file
// and a default name '_index.md' to determine if this node
// is an index document node.
func (f *frontmatterProcessor) nodeIsIndexFile(name string) bool {
	for _, s := range f.IndexFileNames {
		if strings.EqualFold(name, s) {
			return true
		}
	}
	return name == "_index.md"
}
