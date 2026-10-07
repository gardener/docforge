// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package vitepress

// VitePress is the configuration options for the VitePress site generator.
type VitePress struct {
	Enabled                 bool     `mapstructure:"vitepress"`
	BaseURL                 string   `mapstructure:"vitepress-base-url"`
	IndexFileNames          []string `mapstructure:"vitepress-section-files"`
	VitePressStructuralDirs []string `mapstructure:"vitepress-structural-dirs"`
}
