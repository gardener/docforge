// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package alias

// Alias is the configuration for additional names for files
type Alias struct {
	AliasesEnabled bool `mapstructure:"aliases-enabled"`
}
