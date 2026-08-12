// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package sourceorigin

// SourceOrigin holds configuration for the source-origin frontmatter plugin.
type SourceOrigin struct {
	// SourceOriginEnabled activates the plugin when true.
	SourceOriginEnabled bool `mapstructure:"source-origin-enabled"`
	// GardenerMapping emits managed/local frontmatter fields (Gardener-style)
	// instead of the default origin: remote/local fields when true.
	GardenerMapping bool `mapstructure:"source-origin-gardener-mapping"`
}
