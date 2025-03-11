// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
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

	"github.com/gardener/docforge/cmd/hugo"
	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/nodeplugins/markdown/document/frontmatter"
	"github.com/gardener/docforge/pkg/nodeplugins/markdown/document/markdown"
	"github.com/gardener/docforge/pkg/nodeplugins/markdown/linkresolver"
	"github.com/gardener/docforge/pkg/nodeplugins/markdown/linkvalidator"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	"github.com/gardener/docforge/pkg/writers"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"k8s.io/klog/v2"
)

// Worker represents document worker
type Worker struct {
	markdown     goldmark.Markdown
	linkresolver linkresolver.Interface
	validator    linkvalidator.Interface

	writer writers.Writer

	repositoryhosts    registry.Interface
	hugo               hugo.Hugo
	skipLinkValidation bool
}

// NewDocumentWorker creates Worker objects
func NewDocumentWorker(validator linkvalidator.Interface, linkResolver linkresolver.Interface, rh registry.Interface, hugo hugo.Hugo, writer writers.Writer, skipLinkValidation bool) *Worker {
	return &Worker{
		markdown.New(),
		linkResolver,
		validator,
		writer,
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
	}
	if err := d.writer.Write(node.Name(), node.Path, cnt, node, d.hugo.IndexFileNames); err != nil {
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

	type docContent struct {
		docAst ast.Node
		docCnt []byte
		docURI string
	}
	var fullContent []*docContent

	for _, source := range sources {
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

	if fullContent[0].docAst != nil && fullContent[0].docAst.Kind() == ast.KindDocument {
		firstDoc := fullContent[0].docAst.(*ast.Document)
		docs := []frontmatter.NodeMeta{}
		for _, astNode := range fullContent {
			if astNode.docAst != nil && astNode.docAst.Kind() == ast.KindDocument {
				docs = append(docs, astNode.docAst.(*ast.Document))
			}
		}
		frontmatter.MoveMultiSourceFrontmatterToTopDocument(docs)
		frontmatter.ComputeNodeTitle(firstDoc, n, d.hugo.IndexFileNames, d.hugo.Enabled)
		frontmatter.MergeDocumentAndNodeFrontmatter(firstDoc, n)
	}
	for _, cnt := range fullContent {
		lrt := linkResolverTask{*d, n, cnt.docURI}
		if strings.HasSuffix(cnt.docURI, ".md") {
			rnd := markdown.NewLinkModifierRenderer(markdown.WithLinkResolver(lrt.resolveLink))
			if err := rnd.Render(b, cnt.docCnt, cnt.docAst); err != nil {
				return err
			}
		} else {
			b.Write(cnt.docCnt)
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
			// absolute link that is not referencing any documentation page
			if !d.node.SkipValidation && !d.skipLinkValidation {
				d.validator.ValidateLink(dest, d.source)
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
