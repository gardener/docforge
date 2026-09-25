// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package hugo

import (
	"strings"

	hugoutil "github.com/gardener/docforge/pkg/hugo"
	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/sitegen"
)

// Adapter wraps Hugo to implement sitegen.Config.
// Composition (not embedding) avoids the name clash between the Hugo.Enabled
// bool field and the Enabled() method required by the interface.
type Adapter struct {
	h Hugo
}

// NewAdapter returns a sitegen.Config backed by the given Hugo config.
func NewAdapter(h Hugo) *Adapter { return &Adapter{h: h} }

var _ sitegen.Config = (*Adapter)(nil)

// Enabled implements sitegen.Config.
func (a *Adapter) Enabled() bool { return a.h.Enabled }

// IsIndexFile implements sitegen.Config.
func (a *Adapter) IsIndexFile(name string) bool {
	if name == "_index.md" {
		return true
	}
	for _, f := range a.h.IndexFileNames {
		if strings.EqualFold(name, f) {
			return true
		}
	}
	return false
}

// PrettyPath implements sitegen.Config.
func (a *Adapter) PrettyPath(node *manifest.Node) string {
	return hugoutil.PrettyPath(node, a.h.IndexFileNames)
}

// BaseURL implements sitegen.Config.
func (a *Adapter) BaseURL() string { return a.h.BaseURL }

// StructuralDirs implements sitegen.Config.
func (a *Adapter) StructuralDirs() []string { return a.h.HugoStructuralDirs }
