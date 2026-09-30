// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package sitegen

import "github.com/gardener/docforge/pkg/manifest"

// NoopConfig is a Config that disables all site-generator features.
type NoopConfig struct{}

// Enabled implements Config.
func (NoopConfig) Enabled() bool { return false }

// IsIndexFile implements Config.
func (NoopConfig) IsIndexFile(_ string) bool { return false }

// PrettyPath implements Config.
func (NoopConfig) PrettyPath(node *manifest.Node) string { return node.NodePath() }

// BaseURL implements Config.
func (NoopConfig) BaseURL() string { return "" }

// StructuralDirs implements Config.
func (NoopConfig) StructuralDirs() []string { return nil }
