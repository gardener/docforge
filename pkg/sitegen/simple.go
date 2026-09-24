// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package sitegen

import (
	"strings"

	hugoutil "github.com/gardener/docforge/pkg/hugo"
	"github.com/gardener/docforge/pkg/manifest"
)

// SimpleConfig is a value-based Config for tests and cmd-layer wiring.
// It delegates PrettyPath to pkg/hugo to share the canonical implementation.
type SimpleConfig struct {
	IsEnabled    bool
	IndexFiles   []string
	BaseURLValue string
	StructDirs   []string
}

func (s SimpleConfig) Enabled() bool { return s.IsEnabled }

func (s SimpleConfig) IsIndexFile(name string) bool {
	if name == "_index.md" {
		return true
	}
	for _, f := range s.IndexFiles {
		if strings.EqualFold(name, f) {
			return true
		}
	}
	return false
}

func (s SimpleConfig) PrettyPath(node *manifest.Node) string {
	return hugoutil.PrettyPath(node, s.IndexFiles)
}

func (s SimpleConfig) BaseURL() string          { return s.BaseURLValue }
func (s SimpleConfig) StructuralDirs() []string { return s.StructDirs }
