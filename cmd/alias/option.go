// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package alias

// Alias is the configuration for additional names for files
type Alias struct {
	AliasesEnabled bool `mapstructure:"aliases-enabled"`
}
