// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gentoc

import (
	"path/filepath"
	"strings"

	"github.com/gardener/docforge/pkg/manifest"
	"gopkg.in/yaml.v3"
)

// NavEntry represents one entry in the generated navigation structure.
type NavEntry struct {
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

// FromNodes builds a Nav from the resolved manifest node tree.
// nodes[0] is expected to be the root node produced by manifest.ResolveManifest.
// stripRoot removes the top-level directory prefix from all filenames.
func FromNodes(nodes []*manifest.Node, stripRoot bool) *Nav {
	root := nodes[0]
	children := root.Structure

	var stripPrefix string
	// When the root has exactly one dir child and stripRoot is requested,
	// skip that dir and strip its name from all descendant paths.
	if stripRoot && len(children) == 1 && children[0].Type == "dir" {
		stripPrefix = children[0].NodePath() + "/"
		children = children[0].Structure
	}

	return &Nav{Nav: dirEntries(children, stripPrefix)}
}

// dirEntries converts a slice of sibling nodes into NavEntry slice.
// Directories that contain a README.md are promoted: the README becomes the
// entry filename and the remaining siblings become subnav.
func dirEntries(children []*manifest.Node, stripPrefix string) []*NavEntry {
	var entries []*NavEntry
	for _, n := range children {
		entry := nodeToEntry(n, stripPrefix)
		if entry != nil {
			entries = append(entries, entry)
		}
	}
	return entries
}

func nodeToEntry(n *manifest.Node, stripPrefix string) *NavEntry {
	switch n.Type {
	case "dir":
		return dirNode(n, stripPrefix)
	case "file":
		return fileNode(n, stripPrefix)
	default:
		return nil
	}
}

func filename(nodePath, stripPrefix string) string {
	return strings.TrimPrefix(nodePath, stripPrefix)
}

// fileNode returns an entry for a file node, or nil for non-content files.
func fileNode(n *manifest.Node, stripPrefix string) *NavEntry {
	if !isContentFile(n.NodePath()) {
		return nil
	}
	return &NavEntry{Filename: filename(n.NodePath(), stripPrefix)}
}

// dirNode converts a dir node to a NavEntry.
// If the dir contains a README.md (or _index.md), that file becomes the
// entry filename and its siblings become subnav children.
// Returns nil if the dir has no navigable content.
func dirNode(n *manifest.Node, stripPrefix string) *NavEntry {
	readme, rest := splitReadme(n.Structure)
	sub := dirEntries(rest, stripPrefix)

	if readme != nil {
		entry := &NavEntry{Filename: filename(readme.NodePath(), stripPrefix)}
		if len(sub) > 0 {
			entry.Subnav = sub
		}
		return entry
	}

	// No README — skip dirs with no navigable content (e.g. assets/).
	if len(sub) == 0 {
		return nil
	}
	return &NavEntry{Filename: filename(n.NodePath(), stripPrefix), Subnav: sub}
}

// splitReadme separates the README/index file from the rest of the children.
func splitReadme(children []*manifest.Node) (readme *manifest.Node, rest []*manifest.Node) {
	for _, n := range children {
		if n.Type == "file" && isIndexFile(n.Name()) {
			readme = n
		} else {
			rest = append(rest, n)
		}
	}
	return
}

func isIndexFile(name string) bool {
	lower := strings.ToLower(name)
	return lower == "readme.md" || lower == "_index.md" || lower == "index.md"
}

func isContentFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return contentExts[ext]
}

// Marshal serialises nav to YAML bytes.
func Marshal(nav *Nav) ([]byte, error) {
	return yaml.Marshal(nav)
}
