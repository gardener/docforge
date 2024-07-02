// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/url"
	"path"
	"strings"
	"sync"

	"github.com/gardener/docforge/cmd/hugo"
	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
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

	repositoryhosts    registry.Interface
	hugo               hugo.Hugo
	skipLinkValidation bool
}

// docContent defines a document content
type docContent struct {
	docAst ast.Node
	docCnt []byte
	docURI string
}

// NewDocumentWorker creates Worker objects
func NewDocumentWorker(resourcesRoot string, downloader downloader.Interface, validator linkvalidator.Interface, linkResolver linkresolver.Interface, rh registry.Interface, hugo hugo.Hugo, writer writers.Writer, skipLinkValidation bool) *Worker {
	return &Worker{
		linkResolver,
		downloader,
		validator,
		writer,
		resourcesRoot,
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
		frontmatter.ComputeNodeTitle(firstDoc, n, d.hugo.IndexFileNames, d.hugo.Enabled)
	}
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
	content, err := d.repositoryhosts.Read(ctx, source)
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
	node   *manifest.Node
	source string
}

// DownloadURLName create resource name that will be dowloaded from a resource link
func DownloadURLName(url repositoryhost.URL) string {
	resourcePath := url.ResourceURL()
	mdsum := md5.Sum([]byte(resourcePath))
	ext := path.Ext(resourcePath)
	name := strings.TrimSuffix(path.Base(resourcePath), ext)
	hash := hex.EncodeToString(mdsum[:])[:6]
	return fmt.Sprintf("%s_%s%s", name, hash, ext)

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

func (d *linkResolverTask) resolveEmbededLink(link string, source string) (string, error) {
	var err error
	if repositoryhost.IsRelative(link) {
		link, err = d.repositoryhosts.ResolveRelativeLink(source, link)
		if err != nil {
			return link, err
		}
	} else if !repositoryhost.IsResourceURL(link) {
		return link, nil
	}
	// link has format of a resource url
	resourceURL, err := d.repositoryhosts.ResourceURL(link)
	if err != nil {
		// convert urls from not referenced repository  to raw
		return repositoryhost.RawURL(link)
	}
	// download urls from referenced repositories
	downloadResourceName := DownloadURLName(*resourceURL)
	if err = d.downloader.Schedule(link, downloadResourceName, source); err != nil {
		return link, err
	}
	return "/" + path.Join(d.hugo.BaseURL, d.resourcesRoot, downloadResourceName), nil
}
