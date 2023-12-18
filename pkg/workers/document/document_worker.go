// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"path"
	"strings"
	"sync"

	"github.com/gardener/docforge/cmd/hugo"
	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/readers/link"
	"github.com/gardener/docforge/pkg/readers/repositoryhosts"
	"github.com/gardener/docforge/pkg/workers/document/frontmatter"
	"github.com/gardener/docforge/pkg/workers/document/markdown"
	"github.com/gardener/docforge/pkg/workers/downloader"
	"github.com/gardener/docforge/pkg/workers/linkresolver"
	"github.com/gardener/docforge/pkg/workers/linkvalidator"
	"github.com/gardener/docforge/pkg/writers"
	"github.com/yuin/goldmark/ast"
	"k8s.io/klog/v2"
)

// Worker represents document worker
type Worker struct {
	linkresolver linkresolver.Interface
	downloader   downloader.Interface
	validator    linkvalidator.Interface

	writer writers.Writer

	resourcesRoot string

	Repositoryhosts repositoryhosts.Registry
	Hugo            hugo.Hugo
}

// docContent defines a document content
type docContent struct {
	docAst ast.Node
	docCnt []byte
	docURI string
}

// NewDocumentWorker creates Worker objects
func NewDocumentWorker(resourcesRoot string, downloader downloader.Interface, validator linkvalidator.Interface, linkResolver linkresolver.Interface, rh repositoryhosts.Registry, hugo hugo.Hugo, writer writers.Writer) *Worker {
	return &Worker{
		linkResolver,
		downloader,
		validator,
		writer,
		resourcesRoot,
		rh,
		hugo,
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
	if err := d.writer.Write(node.Name(), node.Path, cnt, node); err != nil {
		return err
	}
	return nil
}

func (d *Worker) process(ctx context.Context, b *bytes.Buffer, n *manifest.Node) error {
	// manifest.Node content by priority
	var fullContent []*docContent
	nodePath := n.NodePath()
	if len(n.Source) > 0 {
		nc, err := d.processSource(ctx, "source", n.Source, nodePath)
		if err != nil {
			return err
		}
		fullContent = append(fullContent, nc)
	}
	for _, src := range n.MultiSource {
		nc, err := d.processSource(ctx, "multiSource", src, nodePath)
		if err != nil {
			return err
		}
		fullContent = append(fullContent, nc)
	}
	if len(fullContent) == 0 {
		klog.Warningf("empty content for node %s\n", nodePath)
		return nil
	}

	if fullContent[0].docAst.Kind() == ast.KindDocument {
		firstDoc := fullContent[0].docAst.(*ast.Document)
		docs := []frontmatter.NodeMeta{}
		for _, astNode := range fullContent {
			if astNode.docAst.Kind() == ast.KindDocument {
				docs = append(docs, astNode.docAst.(*ast.Document))
			}
		}
		frontmatter.MoveMultiSourceFrontmatterToTopDocument(docs)
		frontmatter.MergeDocumentAndNodeFrontmatter(firstDoc, n)
		frontmatter.ComputeNodeTitle(firstDoc, n, d.Hugo.IndexFileNames, d.Hugo.Enabled)
	}
	// 2. - write node content
	for _, cnt := range fullContent {
		lrt := linkResolverTask{
			*d,
			n,
			cnt.docURI,
		}
		rnd := markdown.NewLinkModifierRenderer(markdown.WithLinkResolver(lrt.resolveLink))
		if err := rnd.Render(b, cnt.docCnt, cnt.docAst); err != nil {
			return err
		}
	}
	return nil
}

func (d *Worker) processSource(ctx context.Context, sourceType string, source string, nodePath string) (*docContent, error) {
	var dc *docContent
	repoHost, err := d.Repositoryhosts.Get(source)
	if err != nil {
		return nil, err
	}
	content, err := repoHost.Read(ctx, source)
	if err != nil {
		return nil, fmt.Errorf("reading %s %s from node %s failed: %w", sourceType, source, nodePath, err)
	}
	dc = &docContent{docCnt: content, docURI: source}
	dc.docAst, err = markdown.Parse(content)
	if err != nil {
		return nil, fmt.Errorf("fail to parse %s %s from node %s: %w", sourceType, source, nodePath, err)
	}
	return dc, nil
}

type linkResolverTask struct {
	Worker
	Node   *manifest.Node
	Source string
}

func (d *linkResolverTask) resolveLink(dest string, isEmbeddable bool) (string, error) {
	u, err := url.Parse(dest)
	if err != nil {
		return dest, err
	}
	if u.Scheme == "mailto" {
		return dest, nil
	}
	resource, err := link.NewResourceFromURL(u)
	if err != nil {
		return dest, err
	}
	newLink, shouldValidate, err := d.linkresolver.ResolveLink(dest, d.Node, d.Source)
	if err != nil {
		return dest, err
	}
	if shouldValidate && !downloadEmbeddable(resource) {
		d.validator.ValidateLink(dest, d.Source)
	}
	if !isEmbeddable {
		return newLink, nil
	}
	// Links to resources that are not structure document nodes are scheduled for download and their destination is updated to relative path to predefined location for resources.
	if downloadEmbeddable(resource) {
		downloadResourceName := downloader.DownloadResourceName(resource, d.Source)
		if err = d.downloader.Schedule(newLink, downloadResourceName, d.Source); err != nil {
			return dest, err
		}
		return "/" + path.Join(d.Hugo.BaseURL, d.resourcesRoot, downloadResourceName), nil
	}
	// convert them to raw format
	handler, err := d.Repositoryhosts.Get(dest)
	if err != nil {
		return dest, nil
	}
	rawLink, err := handler.GetRawFormatLink(dest)
	if err != nil {
		return dest, err
	}
	return rawLink, nil
}

func downloadEmbeddable(resource link.Resource) bool {
	if !resource.IsAbs() {
		return true
	}
	// if embeddable link is absolute, download only if belongs to internal GitHub or own organization
	// TODO: make it configurable
	if resource.Host == "github.tools.sap" || resource.Host == "raw.github.tools.sap" || resource.Host == "github.wdf.sap.corp" {
		return true
	}
	return strings.HasPrefix(resource.Path, "/gardener/") && (resource.Host == "github.com" || resource.Host == "raw.githubusercontent.com")
}
