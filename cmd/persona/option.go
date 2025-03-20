// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package persona

// Persona is the configuration options when you want to filter content by personas
type Persona struct {
	PersonaFilterEnabled bool `mapstructure:"persona-filter-enabled"`
}
