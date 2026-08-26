// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package gentoc

import (
	"context"
	"path/filepath"
	"strings"

	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/titleresolver"
	"github.com/yuin/goldmark"
	"gopkg.in/yaml.v3"
	"k8s.io/klog/v2"
)

// NavEntry represents one entry in the generated navigation structure.
type NavEntry struct {
	// Title is the human-readable display name for this entry.
	// Resolved via: manifest frontmatter.title > document frontmatter.title > filename derivation.
	Title    string      `yaml:"title"`
	Filename string      `yaml:"filename"`
	Subnav   []*NavEntry `yaml:"subnav,omitempty"`
}

// Nav is the top-level navigation document.
type Nav struct {
	Nav []*NavEntry `yaml:"nav"`
}

// contentExts is the set of file extensions considered navigation-worthy.
var contentExts = map[string]bool{ //nolint:gochecknoglobals
	".md":   true,
	".html": true,
}

// Builder holds configuration needed to build the navigation structure.
type Builder struct {
	Registry       registry.Interface
	IndexFileNames []string
	// goldmarkMD is a package-level goldmark instance reused for all parses.
	goldmarkMD goldmark.Markdown
}

// NewBuilder creates a Builder. indexFileNames lists filenames treated as section
// index files (e.g. ["readme.md", "README.md"]); _index.md is always treated as one.
func NewBuilder(reg registry.Interface, indexFileNames []string) *Builder {
	gm := goldmark.New(
		goldmark.WithExtensions(extension.GFM, meta.Meta),
	)
	return &Builder{
		Registry:       reg,
		IndexFileNames: indexFileNames,
		goldmarkMD:     gm,
	}
}

// FromNodes builds a Nav from the resolved manifest node tree.
// nodes[0] is expected to be the root node produced by manifest.ResolveManifest.
// stripRoot removes the top-level directory prefix from all filenames.
func (b *Builder) FromNodes(ctx context.Context, nodes []*manifest.Node, stripRoot bool) *Nav {
	root := nodes[0]
	children := root.Structure

	var stripPrefix string
	if stripRoot && len(children) == 1 && children[0].Type == "dir" {
		stripPrefix = children[0].NodePath() + "/"
		children = children[0].Structure
	}

	return &Nav{Nav: b.dirEntries(ctx, children, stripPrefix)}
}

func (b *Builder) dirEntries(ctx context.Context, children []*manifest.Node, stripPrefix string) []*NavEntry {
	var entries []*NavEntry
	for _, n := range children {
		entry := b.nodeToEntry(ctx, n, stripPrefix)
		if entry != nil {
			entries = append(entries, entry)
		}
	}
	return entries
}

func (b *Builder) nodeToEntry(ctx context.Context, n *manifest.Node, stripPrefix string) *NavEntry {
	switch n.Type {
	case "dir":
		return b.dirNode(ctx, n, stripPrefix)
	case "file":
		return b.fileNode(ctx, n, stripPrefix)
	default:
		return nil
	}
}

func filename(nodePath, stripPrefix string) string {
	return strings.TrimPrefix(nodePath, stripPrefix)
}

// fileNode returns an entry for a file node, or nil for non-content files.
func (b *Builder) fileNode(ctx context.Context, n *manifest.Node, stripPrefix string) *NavEntry {
	if !isContentFile(n.NodePath()) {
		return nil
	}
	docFM := b.readDocFrontmatter(ctx, n)
	title := titleresolver.ResolveTitle(
		n.Frontmatter,
		docFM,
		n.Name(),
		b.IndexFileNames,
		parentName(n),
		isRootIndex(n, b.IndexFileNames),
	)
	return &NavEntry{
		Title:    title,
		Filename: filename(n.NodePath(), stripPrefix),
	}
}

// dirNode converts a dir node to a NavEntry.
//
// Title resolution for dir nodes:
// When a dir is promoted via its README/index file (the README's path becomes the
// entry filename), the title is sourced from the DIR node, not the README file.
// Rationale: the dir node carries the manifest-level frontmatter (e.g. weight, title
// overrides for the section), which is the author's intent for the section title.
// The README's own document frontmatter title would describe the document, not the
// section. So: dir.Frontmatter["title"] > dir document FM (typically nil for dirs) >
// dir name derivation.
//
// Returns nil if the dir has no navigable content.
func (b *Builder) dirNode(ctx context.Context, n *manifest.Node, stripPrefix string) *NavEntry {
	readme, rest := splitReadme(n.Structure, b.IndexFileNames)
	sub := b.dirEntries(ctx, rest, stripPrefix)

	// Dir title: resolved from the dir node itself (not the README's document FM).
	// Dir nodes almost never have a Source, so docFM will be nil in practice.
	dirDocFM := b.readDocFrontmatter(ctx, n)
	dirTitle := titleresolver.ResolveTitle(
		n.Frontmatter,
		dirDocFM,
		n.Dir,
		b.IndexFileNames,
		parentName(n),
		false,
	)

	if readme != nil {
		entry := &NavEntry{
			Title:    dirTitle,
			Filename: filename(readme.NodePath(), stripPrefix),
		}
		if len(sub) > 0 {
			entry.Subnav = sub
		}
		return entry
	}

	if len(sub) == 0 {
		return nil
	}
	return &NavEntry{
		Title:    dirTitle,
		Filename: filename(n.NodePath(), stripPrefix),
		Subnav:   sub,
	}
}

// readDocFrontmatter reads and parses the YAML frontmatter from node.Source.
// Returns nil on any error (missing source, read failure, parse failure) and
// logs a warning so the caller can fall back to filename derivation.
func (b *Builder) readDocFrontmatter(ctx context.Context, n *manifest.Node) map[string]interface{} {
	if n.Source == "" || !strings.HasSuffix(strings.ToLower(n.Source), ".md") {
		return nil
	}
	content, err := b.Registry.Read(ctx, n.Source)
	if err != nil {
		klog.Warningf("gen-toc: failed to read %s for title resolution, falling back to filename: %v", n.Source, err)
		return nil
	}
	reader := text.NewReader(content)
	pCtx := parser.NewContext()
	b.goldmarkMD.Parser().Parse(reader, parser.WithContext(pCtx))
	fm, err := meta.TryGet(pCtx)
	if err != nil {
		klog.Warningf("gen-toc: failed to parse frontmatter from %s, falling back to filename: %v", n.Source, err)
		return nil
	}
	return fm
}

// splitReadme separates the README/index file from the rest of the children.
func splitReadme(children []*manifest.Node, indexFileNames []string) (readme *manifest.Node, rest []*manifest.Node) {
	for _, n := range children {
		if n.Type == "file" && isIndexFileName(n.Name(), indexFileNames) {
			readme = n
		} else {
			rest = append(rest, n)
		}
	}
	return
}

func isIndexFileName(name string, indexFileNames []string) bool {
	lower := strings.ToLower(name)
	for _, s := range indexFileNames {
		if strings.ToLower(s) == lower {
			return true
		}
	}
	return lower == "_index.md"
}

func isContentFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return contentExts[ext]
}

// parentName returns the dir name of n's parent, or "" if there is none.
func parentName(n *manifest.Node) string {
	p := n.Parent()
	if p == nil {
		return ""
	}
	return p.Name()
}

// isRootIndex returns true when n is an index/README file whose parent is the root.
func isRootIndex(n *manifest.Node, indexFileNames []string) bool {
	if !isIndexFileName(n.Name(), indexFileNames) {
		return false
	}
	p := n.Parent()
	return p == nil || p.Path == ""
}

// Marshal serialises nav to YAML bytes.
func Marshal(nav *Nav) ([]byte, error) {
	return yaml.Marshal(nav)
}
