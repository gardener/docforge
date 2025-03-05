// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package docsy

// Docsy is the configuration options when using docsy as a theme
type Docsy struct {
	EditThisPageEnabled bool `mapstructure:"docsy-edit-this-page-enabled"`
}
