// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package vitepress

import (
	"strings"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/sitegen"
)

// Adapter wraps VitePress to implement sitegen.Config.
type Adapter struct {
	v VitePress
}

// NewAdapter returns a sitegen.Config backed by the given VitePress config.
func NewAdapter(v VitePress) *Adapter { return &Adapter{v: v} }

var _ sitegen.Config = (*Adapter)(nil)

// Enabled implements sitegen.Config.
func (a *Adapter) Enabled() bool { return a.v.Enabled }

// IsIndexFile implements sitegen.Config.
// Recognises both "index.md" (VitePress convention) and "_index.md" (Hugo
// convention) so that manifests pulling Hugo-style index files from external
// repos are normalised to the VitePress target name automatically.
func (a *Adapter) IsIndexFile(name string) bool {
	if strings.EqualFold(name, "index.md") || strings.EqualFold(name, "_index.md") {
		return true
	}
	for _, f := range a.v.IndexFileNames {
		if strings.EqualFold(name, f) {
			return true
		}
	}
	return false
}

// IndexFileName implements sitegen.Config.
// VitePress uses "index.md" (not "_index.md") as the directory index file.
func (a *Adapter) IndexFileName() string { return "index.md" }

// PrettyPath implements sitegen.Config.
// VitePress handles URL routing natively from the file structure, so no path
// transformation is needed at document-generation time.
func (a *Adapter) PrettyPath(node *manifest.Node) string { return node.NodePath() }

// BaseURL implements sitegen.Config.
func (a *Adapter) BaseURL() string { return a.v.BaseURL }

// StructuralDirs implements sitegen.Config.
func (a *Adapter) StructuralDirs() []string { return a.v.VitePressStructuralDirs }
