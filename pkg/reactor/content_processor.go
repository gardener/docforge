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
	"github.com/gardener/docforge/pkg/util/urls"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"k8s.io/klog/v2"
	"net/url"
	"strings"
	"sync"
	"text/template"
)

var (
	// pool with reusable buffers
	bufPool = sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
		},
	}
)

// NodeContentProcessor operates on documents content to reconcile links and schedule linked resources downloads
type NodeContentProcessor interface {
	// Process node content and write the result in a buffer
	Process(ctx context.Context, buffer *bytes.Buffer, reader Reader, node *api.Node) error
}

type nodeContentProcessor struct {
	resourcesRoot    string
	downloader       DownloadScheduler
	validator        Validator
	rewriteEmbedded  bool
	resourceHandlers resourcehandlers.Registry
	sourceLocations  map[string][]*api.Node
	hugo             bool
	PrettyUrls       bool
	IndexFileNames   []string
	BaseURL          string
	//mux              sync.Mutex
	templates map[string]*template.Template
	rwLock    sync.RWMutex
}

// NewNodeContentProcessor creates NodeContentProcessor objects
func NewNodeContentProcessor(resourcesRoot string, downloadJob DownloadScheduler, validator Validator, rewriteEmbedded bool, rh resourcehandlers.Registry,
	hugo bool, PrettyUrls bool, IndexFileNames []string, BaseURL string) NodeContentProcessor {
	c := &nodeContentProcessor{
		// resourcesRoot specifies the root location for downloaded resource.
		// It is used to rewrite resource links in documents to relative paths.
		resourcesRoot:    resourcesRoot,
		downloader:       downloadJob,
		validator:        validator,
		rewriteEmbedded:  rewriteEmbedded,
		resourceHandlers: rh,
		hugo:             hugo,
		PrettyUrls:       PrettyUrls,
		IndexFileNames:   IndexFileNames,
		BaseURL:          BaseURL,
		sourceLocations:  make(map[string][]*api.Node),
		templates:        map[string]*template.Template{},
	}
	return c
}

func (c *nodeContentProcessor) Process(ctx context.Context, b *bytes.Buffer, r Reader, n *api.Node) error {
	// api.Node content by priority
	var nc []*docContent
	path := n.Path("/")
	// 1. Process Source
	if len(n.Source) > 0 {
		if dc := getCachedContent(n); dc != nil {
			nc = append(nc, dc)
		} else {
			source, err := r.Read(ctx, n.Source)
			if err != nil {
				if resourceNotFound, ok := err.(resourcehandlers.ErrResourceNotFound); ok {
					klog.Warningf("reading source %s from node %s/%s failed: %s\n", n.Source, path, n.Name, resourceNotFound)
				} else {
					return fmt.Errorf("reading source %s from node %s/%s failed: %w", n.Source, path, n.Name, err)
				}
			}
			if len(source) > 0 {
				dc = &docContent{docCnt: source, docURI: n.Source}
				dc.docAst, err = markdown.Parse(source)
				if err != nil {
					return fmt.Errorf("fail to parse source %s from node %s/%s: %w", n.Source, path, n.Name, err)
				}
				nc = append(nc, dc)
			} else if err == nil {
				klog.Warningf("no content read from node %s/%s source %s\n", path, n.Name, n.Source)
			}
		}
	}
	// 2. Process MultiSource
	if len(n.MultiSource) > 0 {
		for i, src := range n.MultiSource {
			source, err := r.Read(ctx, src)
			if err != nil {
				if resourceNotFound, ok := err.(resourcehandlers.ErrResourceNotFound); ok {
					klog.Warningf("reading multiSource[%d] %s from node %s/%s failed: %s\n", i, src, path, n.Name, resourceNotFound)
				} else {
					return fmt.Errorf("reading multiSource[%d] %s from node %s/%s failed: %w", i, src, path, n.Name, err)
				}
			}
			if len(source) > 0 {
				dc := &docContent{docCnt: source, docURI: src}
				dc.docAst, err = markdown.Parse(source)
				if err != nil {
					return fmt.Errorf("fail to parse multiSource[%d] %s from node %s/%s: %w", i, src, path, n.Name, err)
				}
				nc = append(nc, dc)
			} else if err == nil {
				klog.Warningf("no content read from node %s/%s multiSource[%d] %s\n", path, n.Name, i, src)
			}
		}
	}
	// if no content -> return
	if len(nc) == 0 {
		return nil //TODO: ?
	}
	// render node content
	// 1 - frontmatter preprocessing
	var fmp *frontmatterProcessor
	if c.hugo {
		fmp = &frontmatterProcessor{
			node:           n,
			IndexFileNames: c.IndexFileNames,
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

type docContent struct {
	docAst ast.Node
	docCnt []byte
	docURI string
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
			// TODO: doc kind must be 'ast.KindDocument', where to validate this?
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

func (c *nodeContentProcessor) getRenderer(n *api.Node, sourceURI string) renderer.Renderer {
	lr := c.newLinkResolver(n, sourceURI)
	return markdown.NewLinkModifierRenderer(markdown.WithLinkResolver(lr.resolveLink))
}

// extends nodeContentProcessor with current source URI & node
type linkResolver struct {
	*nodeContentProcessor
	node   *api.Node
	source string
}

func (c *nodeContentProcessor) newLinkResolver(node *api.Node, sourceURI string) *linkResolver {
	return &linkResolver{
		nodeContentProcessor: c,
		node:                 node,
		source:               sourceURI,
	}
}

// implements markdown.ResolveLink
func (l *linkResolver) resolveLink(dest string, isEmbeddable bool) (string, error) {
	baseLink, err := l.resolveBaseLink(dest, isEmbeddable)
	if err != nil {
		return "", err
	}
	if isEmbeddable {
		if l.rewriteEmbedded {
			if err = l.rawImage(baseLink.Destination); err != nil {
				return *baseLink.Destination, err
			}
		}
	}
	if l.hugo {
		err = l.rewriteDestination(baseLink)
	}
	return *baseLink.Destination, err
}

// resolve base link
func (l *linkResolver) resolveBaseLink(dest string, isEmbeddable bool) (*Link, error) {
	link := &Link{
		OriginalDestination: dest,
		Destination:         &dest,
	}
	if strings.HasPrefix(dest, "#") || strings.HasPrefix(dest, "mailto:") {
		return link, nil
	}
	// validate destination
	u, err := urls.Parse(dest)
	if err != nil {
		return link, err
	}
	// build absolute link
	var absLink string
	if u.IsAbs() {
		// can we handle changes to this destination?
		if l.resourceHandlers.Get(dest) == nil {
			// we don't have a handler for it. Leave it be.
			l.validator.ValidateLink(u, dest, l.source)
			return link, nil
		}
		absLink = dest
	} else {
		handler := l.resourceHandlers.Get(l.source)
		if handler == nil {
			// TODO: here handler must exist because source was read
			return link, fmt.Errorf("no suitable handler registered for URL %s", l.source)
		}
		// build absolute path for the destination using ContentSourcePath as base
		if absLink, err = handler.BuildAbsLink(l.source, dest); err != nil {
			if _, ok := err.(resourcehandlers.ErrResourceNotFound); ok {
				klog.Warningf("failed to validate absolute link for %s from source %s: %v\n", dest, l.source, err)
				err = nil
				if dest != absLink {
					link.Destination = &absLink
					klog.V(6).Infof("[%s] %s -> %s\n", l.source, dest, absLink)
				}
			}
			return link, err
		}
	}
	if l.node != nil {
		// Links to other documents are enforced relative when
		// linking documents from the node structure.
		// Check if md extension to reduce the walkthroughs
		if u.Extension == "md" || u.Extension == "" {
			// try to find the link in source locations
			if nl, ok := l.sourceLocations[strings.TrimSuffix(absLink, "/")]; ok {
				path := ""
				for _, n := range nl {
					n = findVisibleNode(n)
					if n != nil {
						relPathBetweenNodes := l.node.RelativePath(n)
						if swapPaths(path, relPathBetweenNodes) {
							path = relPathBetweenNodes
							link.DestinationNode = n
							link.Destination = &relPathBetweenNodes
						}
					}
				}
				if dest != path {
					klog.V(6).Infof("[%s] %s -> %s\n", l.source, dest, path)
				}
				return link, nil
			}
			// a node with target source location does not exist -> rewrite destination with absolute link
			if dest != absLink {
				link.Destination = &absLink
				klog.V(6).Infof("[%s] %s -> %s\n", l.source, dest, absLink)
			}
			u, err = urls.Parse(absLink)
			l.validator.ValidateLink(u, absLink, l.source)
			return link, nil
		}
		// Links to resources that are not structure document nodes are
		// assessed for download eligibility and if applicable their
		// destination is updated to relative path to predefined location
		// for resources.
		if isEmbeddable && (!u.IsAbs() || strings.HasPrefix(u.Path, "/gardener")) { // TODO: cfg for own organizations
			hash := md5.Sum([]byte(absLink)) // hash based on absolute link
			downloadResourceName := "$name_$hash$ext"
			downloadResourceName = strings.ReplaceAll(downloadResourceName, "$name", u.ResourceName)
			downloadResourceName = strings.ReplaceAll(downloadResourceName, "$hash", hex.EncodeToString(hash[:])[:6])
			downloadResourceName = strings.ReplaceAll(downloadResourceName, "$ext", fmt.Sprintf(".%s", u.Extension))

			resLocation := buildDownloadDestination(l.node, downloadResourceName, l.resourcesRoot)
			if resLocation != dest {
				link.Destination = &resLocation
				klog.V(6).Infof("[%s] %s -> %s\n", l.source, resLocation, dest)
			}
			link.IsResource = true

			if err = l.downloader.Schedule(&DownloadTask{
				absLink,
				downloadResourceName,
				l.source,
				dest,
			}); err != nil {
				return link, err
			}

			return link, nil
		}
	}
	if dest != absLink {
		link.Destination = &absLink
		klog.V(6).Infof("[%s] %s -> %s\n", l.source, dest, absLink)
	}
	u, err = urls.Parse(absLink)
	l.validator.ValidateLink(u, dest, l.source)
	return link, nil
}

// findVisibleNode returns
// - the node if it is a document api.Node
// - first container node that contains index file if the api.Node is container
// - nil if no container node with index file found
// otherwise link will display empty page
// TODO: check if this works if HUGO Pretty URLs are disabled!
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

// used in Hugo mode
type frontmatterProcessor struct {
	node           *api.Node
	IndexFileNames []string
}

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

// rewrite abs links to embedded objects to their raw link format if necessary, to
// ensure they are embeddable
func (l *linkResolver) rawImage(link *string) (err error) {
	var (
		u *url.URL
	)
	if u, err = url.Parse(*link); err != nil {
		return
	}
	if !u.IsAbs() {
		return nil
	}
	handler := l.resourceHandlers.Get(*link)
	if handler == nil {
		return nil
	}
	if *link, err = handler.GetRawFormatLink(*link); err != nil {
		return
	}
	return nil
}

func (l *linkResolver) rewriteDestination(lnk *Link) error {
	link := *lnk.Destination
	link = strings.TrimSpace(link)
	if len(link) == 0 {
		return nil
	}
	// trim leading and trailing quotes if any
	link = strings.TrimSuffix(strings.TrimPrefix(link, "\""), "\"")
	u, err := url.Parse(link)
	if err != nil {
		klog.Warning("Invalid link:", link)
		return nil
	}
	if !u.IsAbs() && !strings.HasPrefix(link, "/") && !strings.HasPrefix(link, "#") {
		_l := link
		link = u.Path
		if lnk.DestinationNode != nil {
			absPath := lnk.DestinationNode.Path("/")
			link = fmt.Sprintf("%s/%s/%s", l.BaseURL, absPath, strings.ToLower(lnk.DestinationNode.Name))
		}
		if lnk.IsResource {
			for strings.HasPrefix(link, "../") {
				link = strings.TrimPrefix(link, "../")
			}
			link = fmt.Sprintf("%s/%s", l.BaseURL, link)
		}

		link = strings.TrimPrefix(link, "./")
		if l.PrettyUrls { // TODO: This is HUGO cfg? what if the URLs are relative, do we need this modification?
			link = strings.TrimSuffix(link, ".md") // TODO all index files names are set to _index.md
			// Remove the last path segment if it is readme, index or _index
			// The Hugo writer will rename those files to _index.md and runtime
			// references will be to the sections in which they reside.
			for _, s := range l.IndexFileNames {
				if strings.HasSuffix(strings.ToLower(link), s) {
					pathSegments := strings.Split(link, "/")
					if len(pathSegments) > 0 {
						pathSegments = pathSegments[:len(pathSegments)-1]
						link = strings.Join(pathSegments, "/")
					}
					break
				}
			}
		} else {
			if strings.HasSuffix(link, ".md") {
				link = strings.TrimSuffix(link, ".md")
				// TODO: propagate fragment and query if any
				link = fmt.Sprintf("%s.html", link)
			}
		}
		if lnk.DestinationNode != nil {
			// check for hugo url property & rewrite the link
			// see https://gohugo.io/content-management/urls/ for details
			if val, ok := lnk.DestinationNode.Properties["frontmatter"]; ok { // TODO: Remove such HUGO properties ???
				if fmProps, ok := val.(map[string]interface{}); ok {
					if urlVal, ok := fmProps["url"]; ok {
						if urlStr, ok := urlVal.(string); ok {
							if _, err = url.Parse(urlStr); err != nil {
								klog.Warningf("Invalid frontmatter url: %s for %s\n", urlStr, lnk.DestinationNode.Source)
							} else {
								link = urlStr
							}
						}
					}
				}
			}
		}
		if _l != link {
			klog.V(6).Infof("[%s] Rewriting node link for Hugo: %s -> %s \n", l.source, _l, link)
		}
		lnk.Destination = &link
		return nil
	}
	return nil
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

// Link defines a markdown link
type Link struct {
	DestinationNode     *api.Node
	IsResource          bool
	OriginalDestination string
	AbsLink             *string
	Destination         *string
}
