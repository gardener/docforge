// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package sitegen

import "github.com/gardener/docforge/pkg/manifest"

// NoopConfig is a Config that disables all site-generator features.
type NoopConfig struct{}

func (NoopConfig) Enabled() bool                         { return false }
func (NoopConfig) IsIndexFile(_ string) bool             { return false }
func (NoopConfig) PrettyPath(node *manifest.Node) string { return node.NodePath() }
func (NoopConfig) BaseURL() string                       { return "" }
func (NoopConfig) StructuralDirs() []string              { return nil }
