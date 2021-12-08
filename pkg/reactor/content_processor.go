// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reactor

import (
	"bytes"
	"context"
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
	resourcesRoot     string
	globalLinksConfig *api.Links
	downloader        DownloadScheduler
	validator         Validator
	rewriteEmbedded   bool
	resourceHandlers  resourcehandlers.Registry
	sourceLocations   map[string][]*api.Node
	hugo              bool
	PrettyUrls        bool
	IndexFileNames    []string
	BaseURL           string
	mux               sync.Mutex
	resourceAbsLinks  map[string]string
	templates         map[string]*template.Template
	rwLock            sync.RWMutex
}

// NewNodeContentProcessor creates NodeContentProcessor objects
func NewNodeContentProcessor(resourcesRoot string, globalLinksConfig *api.Links, downloadJob DownloadScheduler, validator Validator, rewriteEmbedded bool, rh resourcehandlers.Registry,
	hugo bool, PrettyUrls bool, IndexFileNames []string, BaseURL string) NodeContentProcessor {
	c := &nodeContentProcessor{
		// resourcesRoot specifies the root location for downloaded resource.
		// It is used to rewrite resource links in documents to relative paths.
		resourcesRoot:     resourcesRoot,
		globalLinksConfig: globalLinksConfig,
		downloader:        downloadJob,
		validator:         validator,
		rewriteEmbedded:   rewriteEmbedded,
		resourceHandlers:  rh,
		hugo:              hugo,
		PrettyUrls:        PrettyUrls,
		IndexFileNames:    IndexFileNames,
		BaseURL:           BaseURL,
		sourceLocations:   make(map[string][]*api.Node),
		resourceAbsLinks:  make(map[string]string),
		templates:         map[string]*template.Template{},
	}
	return c
}

func (c *nodeContentProcessor) Process(ctx context.Context, b *bytes.Buffer, r Reader, n *api.Node) error {
	// api.Node content by priority
	var nc []*docContent
	path := api.Path(n, "/")
	// 1. Process Source
	if len(n.Source) > 0 {
		source, err := r.Read(ctx, n.Source)
		if err != nil {
			if resourceNotFound, ok := err.(resourcehandlers.ErrResourceNotFound); ok {
				klog.Warningf("reading source %s from node %s/%s failed: %s\n", n.Source, path, n.Name, resourceNotFound)
			} else {
				return fmt.Errorf("reading source %s from node %s/%s failed: %w", n.Source, path, n.Name, err)
			}
		}
		if len(source) == 0 {
			klog.Warningf("no content read from node %s/%s source %s\n", path, n.Name, n.Source)
		} else {
			dc := &docContent{docCnt: source, docURI: n.Source}
			dc.docAst, err = markdown.Parse(source)
			if err != nil {
				return fmt.Errorf("fail to parse source %s from node %s/%s: %w", n.Source, path, n.Name, err)
			}
			nc = append(nc, dc)
		}
	}
	// 2. Process ContentSelectors
	if len(n.ContentSelectors) > 0 {
		for _, content := range n.ContentSelectors {
			source, err := r.Read(ctx, content.Source)
			if err != nil {
				if resourceNotFound, ok := err.(resourcehandlers.ErrResourceNotFound); ok {
					klog.Warningf("reading content selector source %s from node %s/%s failed: %s\n", content.Source, path, n.Name, resourceNotFound)
				} else {
					return fmt.Errorf("reading content selector source %s from node %s/%s failed: %w", content.Source, path, n.Name, err)
				}
			}
			if len(source) == 0 {
				klog.Warningf("no content read from node %s/%s content selector source %s\n", path, n.Name, content.Source)
			} else {
				dc := &docContent{docCnt: source, docURI: content.Source}
				dc.docAst, err = markdown.Parse(source)
				if err != nil {
					return fmt.Errorf("fail to parse content selector source %s from node %s/%s: %w", content.Source, path, n.Name, err)
				}
				nc = append(nc, dc)
			}
		}
	}
	// 3. Process Template
	var tmplBlob []byte
	if n.Template != nil {
		// init template
		tmpl, err := c.initTemplate(ctx, path, r, n)
		if err != nil {
			return err
		}
		// init placeholders content
		vars := map[string]string{}
		for varName, content := range n.Template.Sources {
			var source []byte
			var doc ast.Node
			source, err = r.Read(ctx, content.Source)
			if err != nil {
				if resourceNotFound, ok := err.(resourcehandlers.ErrResourceNotFound); ok {
					klog.Warningf("reading template source %s:%s from node %s/%s failed: %s\n", varName, content.Source, path, n.Name, resourceNotFound)
				} else {
					return fmt.Errorf("reading template source %s:%s from node %s/%s failed: %w", varName, content.Source, path, n.Name, err)
				}
			}
			if len(source) == 0 {
				klog.Warningf("no content read from node %s/%s template source %s:%s\n", path, n.Name, varName, content.Source)
			} else {
				doc, err = markdown.Parse(source)
				if err != nil {
					return fmt.Errorf("fail to parse template source %s:%s from node %s/%s: %w", varName, content.Source, path, n.Name, err)
				}
			}
			if doc != nil {
				// remove frontmatter for template placeholder values
				if doc.Kind() == ast.KindDocument {
					doc.(*ast.Document).SetMeta(nil)
				}
				// render content
				var value string
				value, err = c.renderTemplatePlaceholderValue(source, doc, n, content.Source)
				if err != nil {
					return fmt.Errorf("fail to render template source %s:%s from node %s/%s: %w", varName, content.Source, path, n.Name, err)
				}
				vars[varName] = value
			} else {
				// add empty string
				vars[varName] = ""
			}
		}
		if tmplBlob, err = applyTemplate(tmpl, vars, path, n); err != nil {
			return err
		}
		// add empty doc content to prepend node frontmatter if only a template is defined
		dc := &docContent{docAst: ast.NewDocument(), docCnt: []byte{}, docURI: ""}
		nc = append(nc, dc)
	}
	// if no content -> return
	if len(nc) == 0 {
		return nil
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
	if len(tmplBlob) > 0 {
		_, _ = b.Write(tmplBlob)
	}
	return nil
}

type docContent struct {
	docAst ast.Node
	docCnt []byte
	docURI string
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

// TODO: how to ensure that template will produce valid Markdown ?
func applyTemplate(tmpl *template.Template, vars map[string]string, nodePath string, n *api.Node) ([]byte, error) {
	tmplBytes := &bytes.Buffer{}
	if err := tmpl.Execute(tmplBytes, vars); err != nil {
		return nil, fmt.Errorf("executing template %s from node %s/%s failed: %w", n.Template.Path, nodePath, n.Name, err)
	}
	return tmplBytes.Bytes(), nil
}

func (c *nodeContentProcessor) renderTemplatePlaceholderValue(source []byte, doc ast.Node, n *api.Node, sourceURI string) (string, error) {
	rnd := c.getRenderer(n, sourceURI)
	bytesBuff := bufPool.Get().(*bytes.Buffer)
	defer bufPool.Put(bytesBuff)
	bytesBuff.Reset()
	if err := rnd.Render(bytesBuff, source, doc); err != nil {
		return "", err
	}
	return string(bytesBuff.Bytes()), nil
}

func (c *nodeContentProcessor) getTemplate(templatePath string) (*template.Template, bool) {
	c.rwLock.RLock()
	defer c.rwLock.RUnlock()
	tmpl, ok := c.templates[templatePath]
	return tmpl, ok
}

func (c *nodeContentProcessor) initTemplate(ctx context.Context, nodePath string, r Reader, n *api.Node) (*template.Template, error) {
	if tmpl, ok := c.getTemplate(n.Template.Path); ok {
		if tmpl == nil {
			return nil, fmt.Errorf("template %s from node %s/%s initialization failed", n.Template.Path, nodePath, n.Name)
		}
		return tmpl, nil
	}
	c.rwLock.Lock()
	defer c.rwLock.Unlock()
	if tmpl, ok := c.templates[n.Template.Path]; ok {
		return tmpl, nil
	}
	blob, err := r.Read(ctx, n.Template.Path)
	if err != nil {
		if resourceNotFound, ok := err.(resourcehandlers.ErrResourceNotFound); ok {
			klog.Warningf("reading template blob %s from node %s/%s failed: %s\n", n.Template.Path, nodePath, n.Name, resourceNotFound)
		} else {
			return nil, fmt.Errorf("reading template blob %s from node %s/%s failed: %w", n.Template.Path, nodePath, n.Name, err)
		}
	}
	if len(blob) == 0 {
		c.templates[n.Template.Path] = nil
		return nil, fmt.Errorf("no content read from node %s/%s template blob %s", nodePath, n.Name, n.Template.Path)
	}
	var tmpl *template.Template
	tmpl, err = template.New(n.Template.Path).Parse(string(blob))
	if err != nil {
		c.templates[n.Template.Path] = nil
		return nil, err
	}
	c.templates[n.Template.Path] = tmpl
	return tmpl, nil
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
	baseLink, err := l.resolveBaseLink(dest)
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
	// TODO: include stats?
	if l.hugo {
		err = l.rewriteDestination(baseLink)
	}
	return *baseLink.Destination, err
}

// resolve base link
func (l *linkResolver) resolveBaseLink(dest string) (*Link, error) {
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
	// rewrite link if required - TODO: remove link rewrites using rewrite rules?
	var globalRewrites map[string]*api.LinkRewriteRule
	if gLinks := l.globalLinksConfig; gLinks != nil {
		globalRewrites = gLinks.Rewrites
	}
	_a := absLink
	if l.node != nil {
		if version, substituteDestination, _, _, ok := MatchForLinkRewrite(absLink, l.node, globalRewrites); ok {
			if substituteDestination != nil {
				if len(*substituteDestination) == 0 {
					// quit early. substitution is a request to remove this link
					s := ""
					link.Destination = &s
					return link, nil
				}
				absLink = *substituteDestination
			}
			if version != nil {
				handler := l.resourceHandlers.Get(absLink)
				if handler == nil {
					link.Destination = &absLink
					return link, nil
				}
				absLink, err = handler.SetVersion(absLink, *version) // TODO: SetVersion?
				if err != nil {
					klog.Warningf("Failed to set version %s to %s: %s\n", *version, absLink, err.Error())
					link.Destination = &absLink
					return link, nil
				}
			}
		}
	}

	// validate potentially rewritten links
	u, err = urls.Parse(absLink)
	if err != nil {
		return link, err
	}
	if _a != absLink {
		klog.V(6).Infof("[%s] Link rewritten %s -> %s\n", l.source, _a, absLink)
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
					relPathBetweenNodes := l.node.RelativePath(n)
					if swapPaths(path, relPathBetweenNodes) {
						path = relPathBetweenNodes
						link.DestinationNode = n
						link.Destination = &relPathBetweenNodes
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
		var globalDownloadsConfig *api.Downloads
		if l.globalLinksConfig != nil {
			globalDownloadsConfig = l.globalLinksConfig.Downloads
		}
		if downloadResourceName, ok := MatchForDownload(u, l.node, globalDownloadsConfig); ok {
			resourceName := l.getDownloadResourceName(u, downloadResourceName)
			resLocation := buildDownloadDestination(l.node, resourceName, l.resourcesRoot)
			if resLocation != dest {
				link.Destination = &resLocation
				klog.V(6).Infof("[%s] %s -> %s\n", l.source, resLocation, dest)
			}
			link.IsResource = true
			if err = l.downloader.Schedule(&DownloadTask{
				absLink,
				resourceName,
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
			return nil, fmt.Errorf("invalid frontmatter properties for node:  %s/%s", api.Path(f.node, "/"), f.node.Name)
		}
	}
	// 2 front matter from doc node parent (only if the current one is section file)
	if f.node.Name == "_index.md" && f.node.Parent() != nil {
		if val, ok := f.node.Parent().Properties["frontmatter"]; ok {
			parentMeta, ok = val.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid frontmatter properties for node:  %s", api.Path(f.node, "/"))
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
// and a default name '_index.md' to determin if this node
// is an index document node.
func (f *frontmatterProcessor) nodeIsIndexFile(name string) bool {
	for _, s := range f.IndexFileNames {
		if strings.EqualFold(name, s) {
			return true
		}
	}
	return name == "_index.md"
}

// Check for cached resource name first and return that if found. Otherwise,
// return the downloadName
// TODO:
func (l *linkResolver) getDownloadResourceName(u *urls.URL, downloadName string) string {
	l.mux.Lock()
	defer l.mux.Unlock()
	if cachedDownloadName, ok := l.resourceAbsLinks[u.Path]; ok {
		return cachedDownloadName
	}
	return downloadName
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
			absPath := api.Path(lnk.DestinationNode, "/")
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
