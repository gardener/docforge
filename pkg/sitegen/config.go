// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package sitegen

import "github.com/gardener/docforge/pkg/manifest"

// Config abstracts site-generator-specific behaviour so that pkg/ packages
// have zero knowledge of any concrete site generator (e.g. Hugo, VitePress).
type Config interface {
	Enabled() bool
	IsIndexFile(name string) bool
	// IndexFileName returns the target file name to use when IsIndexFile returns
	// true (e.g. "_index.md" for Hugo, "index.md" for VitePress). An empty
	// string means no rename should happen.
	IndexFileName() string
	// UsesManifestNameInEditPath reports whether the "edit this page" path
	// (.from in Docsy frontmatter) should use the original manifest filename
	// rather than the renamed output filename (true = Hugo: README.md stays
	// README.md even though the output file is _index.md; false = VitePress,
	// which uses IndexFileName() so .from reflects the served path).
	UsesManifestNameInEditPath() bool
	PrettyPath(node *manifest.Node) string
	BaseURL() string
	StructuralDirs() []string
}
