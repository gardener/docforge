// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/nodeplugins/markdown/document/frontmatter"
	"github.com/gardener/docforge/pkg/nodeplugins/markdown/document/markdown"
	"github.com/gardener/docforge/pkg/nodeplugins/markdown/linkresolver"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	"github.com/gardener/docforge/pkg/sitegen"
	"github.com/gardener/docforge/pkg/writers"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"gopkg.in/yaml.v3"
	"k8s.io/klog/v2"
)

// policyViolation is a soft error returned by process() when content was stripped and
// failOnStrippedContent is set. Unlike a hard error, the output buffer is fully populated
// when this is returned, so ProcessNode still writes the output before propagating it.
type policyViolation struct{ msg string }

func (e *policyViolation) Error() string { return e.msg }

// Worker represents document worker
type Worker struct {
	markdown     goldmark.Markdown
	linkresolver linkresolver.Interface

	writer writers.Writer

	repositoryhosts       registry.Interface
	config                sitegen.Config
	failOnStrippedContent bool
}

// NewDocumentWorker creates Worker objects
func NewDocumentWorker(linkResolver linkresolver.Interface, rh registry.Interface, config sitegen.Config, writer writers.Writer) *Worker {
	return &Worker{
		markdown:        markdown.New(),
		linkresolver:    linkResolver,
		writer:          writer,
		repositoryhosts: rh,
		config:          config,
	}
}

var (
	// pool with reusable buffers
	bufPool = sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
		},
	}
)

// ProcessNode processes a node and writes its content.
// When failOnStrippedContent is set, any policy violation causes a non-nil error return, but the
// (placeholder or sanitized) output is always written to the writer before the error is returned.
func (d *Worker) ProcessNode(ctx context.Context, node *manifest.Node) error {
	var cnt []byte
	var policyErr error
	writeNode := node // default: write the original node unchanged
	if node.HasContent() {
		bytesBuff := bufPool.Get().(*bytes.Buffer)
		defer bufPool.Put(bytesBuff)
		bytesBuff.Reset()
		err := d.process(ctx, bytesBuff, node)
		var pv *policyViolation
		if err != nil && !errors.As(err, &pv) {
			return err // hard error: do not write
		}
		policyErr = err
		if bytesBuff.Len() == 0 {
			klog.Warningf("document node processing halted: no content assigned to document node %s/%s", node.Path, node.Name())
			return policyErr
		}
		cnt = bytesBuff.Bytes()
	} else {
		writeNode, policyErr = d.sanitizeContentlessNode(node)
	}
	name := node.Name()
	if d.config != nil && d.config.Enabled() && d.config.IsIndexFile(name) {
		if target := d.config.IndexFileName(); target != "" {
			name = target
		}
	}
	if err := d.writer.Write(name, node.Path, cnt, writeNode); err != nil {
		return err
	}
	return policyErr
}

// sanitizeContentlessNode returns the node to pass to Write for a content-less node, with
// "head" removed, and a StripHead violation when --fail-on-stripped-content is set.
// For content-less nodes, pass a shallow node copy to Write with "head" removed.
// The top-level frontmatter map is copied and "head" is deleted from the copy, so
// node.Frontmatter keeps its original keys. Nested values are shared with the
// original (shallow copy); this is safe because the writer only reads them
// (hugoIndexContent marshals node.Frontmatter without mutating it).
func (d *Worker) sanitizeContentlessNode(node *manifest.Node) (*manifest.Node, error) {
	if node.Frontmatter == nil || node.AllowsScriptBlocks() {
		return node, nil
	}
	sanitized := make(map[string]interface{}, len(node.Frontmatter))
	for k, v := range node.Frontmatter {
		sanitized[k] = v
	}
	findings := markdown.SanitizeFrontmatter(sanitized, false)
	if len(findings) == 0 {
		return node, nil
	}
	for _, f := range findings {
		klog.Warningf("content policy: %s in node %s/%s", f.Kind, node.Path, node.Name())
	}
	nodeCopy := *node
	nodeCopy.Frontmatter = sanitized
	if d.failOnStrippedContent {
		return &nodeCopy, &policyViolation{fmt.Sprintf("content policy: stripped frontmatter field in node %s/%s", node.Path, node.Name())}
	}
	return &nodeCopy, nil
}

// docContent holds a single resolved source: its parsed AST (nil for non-markdown),
// raw bytes and origin URI.
type docContent struct {
	docAst ast.Node
	docCnt []byte
	docURI string
}

// renderedSource holds the rendered output buffer and policy findings for one source.
type renderedSource struct {
	buf      bytes.Buffer
	findings []markdown.Finding
	uri      string
}

func (d *Worker) process(ctx context.Context, b *bytes.Buffer, n *manifest.Node) error {
	nodePath := n.NodePath()
	sources := collectSources(n)
	if len(sources) == 0 {
		klog.Warningf("empty content for node %s\n", nodePath)
		return nil
	}

	trusted := n.AllowsScriptBlocks()

	fullContent, refuseFindings, err := d.loadSources(ctx, sources, nodePath)
	if err != nil {
		return err
	}

	// Any refused source makes the whole document a placeholder (fail closed).
	// This takes precedence over other content, even if other sources were accepted.
	if len(refuseFindings) > 0 {
		writePlaceholder(b, n, d.config)
		klog.Warningf("content policy: document replaced with placeholder for node %s (non-markdown source refused)", nodePath)
		if d.failOnStrippedContent {
			return &policyViolation{fmt.Sprintf("content policy violation in node %s: %s", nodePath, refuseFindings[0].Kind)}
		}
		return nil
	}

	if len(fullContent) == 0 {
		klog.Warningf("no accepted content sources for node %s\n", nodePath)
		return nil
	}

	// Track a deferred StripHead policy violation: we want the rendered output in the buffer
	// before returning the error so that ProcessNode can still write the sanitized content.
	stripHeadViolation := d.sanitizeFirstSourceFrontmatter(n, fullContent, trusted, nodePath)
	if v := d.stripHeadFromMarkdownSources(n, fullContent, trusted, nodePath); v != nil {
		stripHeadViolation = v
	}

	rendered, err := d.renderSources(n, fullContent, trusted)
	if err != nil {
		return err
	}

	if v := d.writeRenderedResult(b, n, rendered, nodePath); v != nil {
		return v
	}
	if stripHeadViolation != nil {
		return stripHeadViolation
	}
	return nil
}

// collectSources returns the node's single Source (if any) followed by its MultiSource list.
func collectSources(n *manifest.Node) []string {
	sources := []string{}
	if len(n.Source) > 0 {
		sources = append(sources, n.Source)
	}
	return append(sources, n.MultiSource...)
}

// loadSources applies the non-markdown decision, reads and (for markdown) parses every source.
// It returns the accepted content, any Refuse findings, and a hard error on read/parse failure.
func (d *Worker) loadSources(ctx context.Context, sources []string, nodePath string) ([]*docContent, []markdown.Finding, error) {
	var fullContent []*docContent
	var refuseFindings []markdown.Finding

	for _, source := range sources {
		if !strings.HasSuffix(source, ".md") {
			var pol markdown.DefaultPolicy
			switch pol.NonMarkdownDecision(source) {
			case markdown.Refuse:
				klog.Warningf("content policy: refusing non-markdown source %s in node %s (blocked file type)", source, nodePath)
				refuseFindings = append(refuseFindings, markdown.Finding{Decision: markdown.Refuse, Kind: "non-markdown source refused", Source: source})
				continue
			case markdown.WarnNonMarkdown:
				klog.Warningf("content policy: non-markdown source %s in node %s (include with warning)", source, nodePath)
			}
		}

		content, err := d.repositoryhosts.Read(ctx, source)
		if err != nil {
			return nil, nil, fmt.Errorf("reading %s from node %s failed: %w", source, nodePath, err)
		}
		dc := &docContent{docCnt: content, docURI: source}
		if strings.HasSuffix(source, ".md") {
			dc.docAst, err = markdown.Parse(d.markdown, content)
			if err != nil {
				return nil, nil, fmt.Errorf("fail to parses %s from node %s: %w", source, nodePath, err)
			}
		}
		fullContent = append(fullContent, dc)
	}
	return fullContent, refuseFindings, nil
}

// sanitizeFirstSourceFrontmatter moves multi-source frontmatter to the first document, computes
// the node title, merges node frontmatter, and sanitizes the merged frontmatter. It runs only
// when the first source is Markdown. Returns a StripHead violation when the flag is set and a
// field was stripped, else nil.
func (d *Worker) sanitizeFirstSourceFrontmatter(n *manifest.Node, fullContent []*docContent, trusted bool, nodePath string) *policyViolation {
	if fullContent[0].docAst == nil || fullContent[0].docAst.Kind() != ast.KindDocument {
		return nil
	}
	firstDoc := fullContent[0].docAst.(*ast.Document)
	docs := []frontmatter.NodeMeta{}
	for _, astNode := range fullContent {
		if astNode.docAst != nil && astNode.docAst.Kind() == ast.KindDocument {
			docs = append(docs, astNode.docAst.(*ast.Document))
		}
	}
	frontmatter.MoveMultiSourceFrontmatterToTopDocument(docs)
	frontmatter.ComputeNodeTitle(firstDoc, n, d.config)
	frontmatter.MergeDocumentAndNodeFrontmatter(firstDoc, n)
	// Sanitize AFTER merging so "head" from manifest node frontmatter is also removed.
	meta := firstDoc.Meta()
	if meta == nil {
		return nil
	}
	findings := markdown.SanitizeFrontmatter(meta, trusted)
	if len(findings) == 0 {
		return nil
	}
	for _, f := range findings {
		klog.Warningf("content policy: %s in node %s", f.Kind, nodePath)
		if f.Decision == markdown.StripHead {
			// MergeDocumentAndNodeFrontmatter writes merged frontmatter back to
			// n.Frontmatter; delete "head" here too so both stay in sync.
			// The content-less path uses a shallow copy instead of mutating n
			// directly, because hugoIndexContent reads node.Frontmatter after Write.
			delete(n.Frontmatter, "head")
		}
	}
	if d.failOnStrippedContent {
		return &policyViolation{fmt.Sprintf("content policy: stripped frontmatter field in node %s", nodePath)}
	}
	return nil
}

// stripHeadFromMarkdownSources strips "head" from every Markdown source for untrusted nodes.
// sanitizeFirstSourceFrontmatter sanitizes only the first source, and only when the first
// source is Markdown. A node whose first source is non-Markdown (e.g. Source=page.html,
// MultiSource=[doc.md]) skips it entirely, so "head" from a later Markdown source would
// otherwise reach the output with no finding and no error. SanitizeFrontmatter is a no-op when
// "head" is absent, so a source already sanitized above yields no duplicate finding.
func (d *Worker) stripHeadFromMarkdownSources(n *manifest.Node, fullContent []*docContent, trusted bool, nodePath string) *policyViolation {
	if trusted {
		return nil
	}
	headStripped := false
	for _, dc := range fullContent {
		if dc.docAst == nil || dc.docAst.Kind() != ast.KindDocument {
			continue
		}
		meta := dc.docAst.(*ast.Document).Meta()
		if meta == nil {
			continue
		}
		for _, f := range markdown.SanitizeFrontmatter(meta, false) {
			klog.Warningf("content policy: %s in source %s (node %s)", f.Kind, dc.docURI, nodePath)
			if f.Decision == markdown.StripHead {
				// keep n.Frontmatter in sync; no-op when "head" is not present there
				delete(n.Frontmatter, "head")
			}
			headStripped = true
		}
	}
	if headStripped && d.failOnStrippedContent {
		return &policyViolation{fmt.Sprintf("content policy: stripped frontmatter field in node %s", nodePath)}
	}
	return nil
}

// renderSources renders each source into its own buffer and collects policy findings.
func (d *Worker) renderSources(n *manifest.Node, fullContent []*docContent, trusted bool) ([]*renderedSource, error) {
	rendered := make([]*renderedSource, 0, len(fullContent))
	for _, cnt := range fullContent {
		lrt := linkResolverTask{*d, n, cnt.docURI}
		rs := &renderedSource{uri: cnt.docURI}
		if strings.HasSuffix(cnt.docURI, ".md") {
			rnd := markdown.NewPolicyRenderer(
				markdown.WithTrusted(trusted),
				markdown.WithLinkResolver(lrt.resolveLink),
			)
			if err := rnd.Render(&rs.buf, cnt.docCnt, cnt.docAst); err != nil {
				return nil, err
			}
			rs.findings = rnd.Result()
		} else {
			rs.buf.Write(cnt.docCnt)
		}
		rendered = append(rendered, rs)
	}
	return rendered, nil
}

// writeRenderedResult writes the placeholder (when a source suppresses the document) or the
// concatenated buffers to b, and returns a StripBlock/SuppressDocument violation when the flag
// is set, else nil.
func (d *Worker) writeRenderedResult(b *bytes.Buffer, n *manifest.Node, rendered []*renderedSource, nodePath string) *policyViolation {
	// Accumulate findings, log them, and decide whether to suppress the whole document.
	var allFindings []markdown.Finding
	suppressDoc := false
	for _, rs := range rendered {
		for _, f := range rs.findings {
			klog.Warningf("content policy: %s in source %s (node %s)", f.Kind, rs.uri, nodePath)
			if f.Decision == markdown.SuppressDocument {
				suppressDoc = true
			}
		}
		allFindings = append(allFindings, rs.findings...)
	}

	if suppressDoc {
		writePlaceholder(b, n, d.config)
		klog.Warningf("content policy: document suppressed for node %s (inline script/style found)", nodePath)
	} else {
		for _, rs := range rendered {
			b.Write(rs.buf.Bytes())
		}
	}

	// Return policy violations after the output is in the buffer, so ProcessNode can still write.
	if d.failOnStrippedContent {
		for _, f := range allFindings {
			if f.Decision == markdown.StripBlock || f.Decision == markdown.SuppressDocument {
				return &policyViolation{fmt.Sprintf("content policy violation in node %s: %s", nodePath, f.Kind)}
			}
		}
	}
	return nil
}

// writePlaceholder writes a placeholder document (frontmatter + suppression body) to b.
func writePlaceholder(b *bytes.Buffer, n *manifest.Node, config sitegen.Config) {
	title := frontmatter.PlaceholderTitle(n, config)
	placeholderMeta, _ := yaml.Marshal(map[string]interface{}{"title": title})
	b.Write([]byte("---\n"))
	b.Write(placeholderMeta)
	b.Write([]byte("---\n\n<!-- docforge: document suppressed due to active content -->\n"))
}

type linkResolverTask struct {
	Worker
	node   *manifest.Node
	source string
}

func (d *linkResolverTask) resolveLink(dest string, isEmbeddable bool) (string, error) {
	escapedEmoji := strings.ReplaceAll(dest, "/:v:/", "/%3Av%3A/")
	if escapedEmoji != dest {
		klog.Warningf("escaping : for /:v:/ in link %s for source %s ", dest, d.source)
		dest = escapedEmoji
	}
	url, err := url.Parse(dest)
	if err != nil {
		return dest, err
	}
	if url.Scheme == "mailto" {
		return dest, nil
	}
	if isEmbeddable {
		return d.resolveEmbededLink(dest, d.source)
	}
	// handle non-embeded links
	if url.IsAbs() {
		if _, err = d.repositoryhosts.ResourceURL(dest); err != nil {
			return dest, nil
		}
	}
	resolved, err := d.linkresolver.ResolveResourceLink(dest, d.node, d.source)
	if err != nil {
		return resolved, err
	}
	return resolved, nil
}

func (d *linkResolverTask) resolveEmbededLink(embeddedLink string, source string) (string, error) {
	var err error
	if repositoryhost.IsRelative(embeddedLink) {
		if srcURL, e := d.repositoryhosts.ResourceURL(source); e == nil {
			var structuralDirs []string
			if d.config != nil {
				structuralDirs = d.config.StructuralDirs()
			}
			embeddedLink = linkresolver.ReAnchorRootAbsolute(embeddedLink, srcURL.GetResourcePath(), structuralDirs)
		}
		embeddedLink, err = d.repositoryhosts.ResolveRelativeLink(source, embeddedLink)
		if err != nil {
			return embeddedLink, err
		}
	} else if !repositoryhost.IsResourceURL(embeddedLink) {
		return embeddedLink, nil
	}
	// link has format of a resource url
	resourceURL, err := d.repositoryhosts.ResourceURL(embeddedLink)
	if err != nil {
		// convert urls from not referenced repository  to raw
		return repositoryhost.RawURL(embeddedLink)
	}
	// resolve urls from referenced repositories
	return d.linkresolver.ResolveResourceLink(resourceURL.String(), d.node, source)
}
