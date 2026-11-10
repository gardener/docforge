// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"bytes"
	"context"
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

// Worker represents document worker
type Worker struct {
	markdown     goldmark.Markdown
	linkresolver linkresolver.Interface

	writer writers.Writer

	repositoryhosts     registry.Interface
	config              sitegen.Config
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

// ProcessNode processes a node and writes its content
func (d *Worker) ProcessNode(ctx context.Context, node *manifest.Node) error {
	var cnt []byte
	if node.HasContent() {
		// Process the node
		bytesBuff := bufPool.Get().(*bytes.Buffer)
		defer bufPool.Put(bytesBuff)
		bytesBuff.Reset()
		if err := d.process(ctx, bytesBuff, node); err != nil {
			return err
		}
		if bytesBuff.Len() == 0 {
			klog.Warningf("document node processing halted: no content assigned to document node %s/%s", node.Path, node.Name())
			return nil
		}
		cnt = bytesBuff.Bytes()
	} else {
		// For content-less nodes, sanitize the node's own frontmatter to strip the "head" key.
		// Copy → sanitize → write → restore so that the manifest tree is not permanently mutated.
		if node.Frontmatter != nil && !node.AllowsScriptBlocks() {
			original := node.Frontmatter
			sanitized := make(map[string]interface{}, len(original))
			for k, v := range original {
				sanitized[k] = v
			}
			if findings := markdown.SanitizeFrontmatter(sanitized, false); len(findings) > 0 {
				for _, f := range findings {
					klog.Warningf("content policy: %s in node %s/%s", f.Kind, node.Path, node.Name())
				}
				node.Frontmatter = sanitized
				defer func() { node.Frontmatter = original }()
			}
		}
	}
	name := node.Name()
	if d.config != nil && d.config.Enabled() && d.config.IsIndexFile(name) {
		if target := d.config.IndexFileName(); target != "" {
			name = target
		}
	}
	if err := d.writer.Write(name, node.Path, cnt, node); err != nil {
		return err
	}
	return nil
}

func (d *Worker) process(ctx context.Context, b *bytes.Buffer, n *manifest.Node) error {
	sources := []string{}
	nodePath := n.NodePath()
	if len(n.Source) > 0 {
		sources = append(sources, n.Source)
	}
	sources = append(sources, n.MultiSource...)
	if len(sources) == 0 {
		klog.Warningf("empty content for node %s\n", nodePath)
		return nil
	}

	trusted := n.AllowsScriptBlocks()

	type docContent struct {
		docAst ast.Node
		docCnt []byte
		docURI string
	}
	var fullContent []*docContent

	for _, source := range sources {
		if !strings.HasSuffix(source, ".md") {
			var pol markdown.DefaultPolicy
			switch pol.NonMarkdownDecision(source) {
			case markdown.Refuse:
				klog.Warningf("content policy: refusing non-markdown source %s in node %s (blocked file type)", source, nodePath)
				continue
			case markdown.WarnNonMarkdown:
				klog.Warningf("content policy: non-markdown source %s in node %s (include with warning)", source, nodePath)
			}
		}

		content, err := d.repositoryhosts.Read(ctx, source)
		if err != nil {
			return fmt.Errorf("reading %s from node %s failed: %w", source, nodePath, err)
		}
		dc := &docContent{docCnt: content, docURI: source}
		if strings.HasSuffix(source, ".md") {
			dc.docAst, err = markdown.Parse(d.markdown, content)
			if err != nil {
				return fmt.Errorf("fail to parses %s from node %s: %w", source, nodePath, err)
			}
		}
		fullContent = append(fullContent, dc)
	}

	if len(fullContent) == 0 {
		klog.Warningf("no accepted content sources for node %s\n", nodePath)
		return nil
	}

	if fullContent[0].docAst != nil && fullContent[0].docAst.Kind() == ast.KindDocument {
		firstDoc := fullContent[0].docAst.(*ast.Document)
		docs := []frontmatter.NodeMeta{}
		for _, astNode := range fullContent {
			if astNode.docAst != nil && astNode.docAst.Kind() == ast.KindDocument {
				docs = append(docs, astNode.docAst.(*ast.Document))
			}
		}
		frontmatter.MoveMultiSourceFrontmatterToTopDocument(docs)
		frontmatter.ComputeNodeTitle(firstDoc, n, d.config)
		// Sanitize before merging so the "head" key is never copied to n.Frontmatter.
		if meta := firstDoc.Meta(); meta != nil {
			if findings := markdown.SanitizeFrontmatter(meta, trusted); len(findings) > 0 {
				for _, f := range findings {
					klog.Warningf("content policy: %s in node %s", f.Kind, nodePath)
				}
				if d.failOnStrippedContent {
					return fmt.Errorf("content policy: stripped frontmatter field in node %s", nodePath)
				}
			}
		}
		frontmatter.MergeDocumentAndNodeFrontmatter(firstDoc, n)
	}

	type renderedSource struct {
		buf      bytes.Buffer
		findings []markdown.Finding
		uri      string
	}
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
				return err
			}
			rs.findings = rnd.Result()
		} else {
			rs.buf.Write(cnt.docCnt)
		}
		rendered = append(rendered, rs)
	}

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
		title := frontmatter.PlaceholderTitle(n, d.config)
		placeholderMeta, _ := yaml.Marshal(map[string]interface{}{"title": title})
		b.Write([]byte("---\n"))
		b.Write(placeholderMeta)
		b.Write([]byte("---\n"))
		klog.Warningf("content policy: document suppressed for node %s (inline script/style found)", nodePath)
	} else {
		for _, rs := range rendered {
			b.Write(rs.buf.Bytes())
		}
	}

	if d.failOnStrippedContent {
		for _, f := range allFindings {
			if f.Decision == markdown.StripBlock || f.Decision == markdown.SuppressDocument {
				return fmt.Errorf("content policy violation in node %s: %s", nodePath, f.Kind)
			}
		}
	}

	return nil
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
