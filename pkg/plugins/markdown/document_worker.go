// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package markdown

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/gardener/docforge/cmd/hugo"
	"github.com/gardener/docforge/pkg/core/manifest"
	"github.com/gardener/docforge/pkg/core/registry"
	"github.com/gardener/docforge/pkg/core/registry/repositoryhost"
	"github.com/gardener/docforge/pkg/osshim/filesystem"
	"github.com/gardener/docforge/pkg/plugins/markdown/frontmatter"
	"github.com/gardener/docforge/pkg/plugins/markdown/linkresolver"
	"github.com/gardener/docforge/pkg/plugins/markdown/parser"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"k8s.io/klog/v2"
)

// Worker represents document worker
type Worker struct {
	markdown     goldmark.Markdown
	linkresolver linkresolver.Interface

	fs       filesystem.Interface
	rootPath string

	repositoryhosts    registry.Interface
	hugo               hugo.Hugo
	skipLinkValidation bool
}

// NewDocumentWorker creates Worker objects
func NewDocumentWorker(linkResolver linkresolver.Interface, rh registry.Interface, hugo hugo.Hugo, fs filesystem.Interface, rootPath string, skipLinkValidation bool) *Worker {
	return &Worker{
		parser.New(),
		linkResolver,
		fs,
		rootPath,
		rh,
		hugo,
		skipLinkValidation,
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

// ProcessNode processes a node and writes its content, returning collected external links
func (d *Worker) ProcessNode(ctx context.Context, node *manifest.Node) ([]manifest.ExternalLink, error) {
	var cnt []byte
	var allLinks []manifest.ExternalLink

	if node.HasContent() {
		// Process the node
		bytesBuff := bufPool.Get().(*bytes.Buffer)
		defer bufPool.Put(bytesBuff)
		bytesBuff.Reset()
		links, err := d.process(ctx, bytesBuff, node)
		if err != nil {
			return nil, err
		}
		allLinks = append(allLinks, links...)
		if bytesBuff.Len() == 0 {
			klog.Warningf("document node processing halted: no content assigned to document node %s/%s", node.Path, node.Name())
			return allLinks, nil
		}
		cnt = bytesBuff.Bytes()
	}
	if err := writeDocument(d.fs, d.rootPath, d.hugo.Enabled, node.Name(), node.Path, cnt, node, d.hugo.IndexFileNames); err != nil {
		return nil, err
	}
	return allLinks, nil
}

func (d *Worker) process(ctx context.Context, b *bytes.Buffer, n *manifest.Node) ([]manifest.ExternalLink, error) {
	nodePath := n.NodePath()
	if len(n.Source) == 0 {
		klog.Warningf("empty content for node %s\n", nodePath)
		return nil, nil
	}

	// Read content from the single source
	content, err := d.repositoryhosts.Read(ctx, n.Source)
	if err != nil {
		return nil, fmt.Errorf("reading %s from node %s failed: %w", n.Source, nodePath, err)
	}

	var docAst ast.Node
	if strings.HasSuffix(n.Source, ".md") {
		docAst, err = parser.Parse(d.markdown, content)
		if err != nil {
			return nil, fmt.Errorf("fail to parses %s from node %s: %w", n.Source, nodePath, err)
		}
	}

	// Handle frontmatter processing for markdown documents
	if docAst != nil && docAst.Kind() == ast.KindDocument {
		doc := docAst.(*ast.Document)
		frontmatter.ComputeNodeTitle(doc, n, d.hugo.IndexFileNames, d.hugo.Enabled)
		frontmatter.MergeDocumentAndNodeFrontmatter(doc, n)
	}

	// Process the content and resolve links
	lrt := linkResolverTask{
		Worker:         *d,
		node:           n,
		source:         n.Source,
		collectedLinks: []manifest.ExternalLink{},
	}

	rnd := parser.NewLinkModifierRenderer(parser.WithLinkResolver(lrt.resolveLink))
	if err := rnd.Render(b, content, docAst); err != nil {
		return nil, err
	}

	return lrt.collectedLinks, nil
}

type linkResolverTask struct {
	Worker
	node           *manifest.Node
	source         string
	collectedLinks []manifest.ExternalLink
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
			// absolute link that is not referencing any documentation page
			if !d.node.SkipValidation && !d.skipLinkValidation {
				// Collect the external link for deferred validation
				d.collectedLinks = append(d.collectedLinks, manifest.ExternalLink{
					URL:        dest,
					SourceFile: d.source,
				})
			}
			return dest, nil
		}
	}
	return d.linkresolver.ResolveResourceLink(dest, d.node, d.source)
}

func (d *linkResolverTask) resolveEmbededLink(embeddedLink string, source string) (string, error) {
	var err error
	if repositoryhost.IsRelative(embeddedLink) {
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
