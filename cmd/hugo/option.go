// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package hugo

// Hugo is the configuration options for creating HUGO implementations
type Hugo struct {
	Enabled            bool     `mapstructure:"hugo"`
	PrettyURLs         bool     `mapstructure:"hugo-pretty-urls"`
	BaseURL            string   `mapstructure:"hugo-base-url"`
	IndexFileNames     []string `mapstructure:"hugo-section-files"`
	HugoStructuralDirs []string `mapstructure:"hugo-structural-dirs"`
}
