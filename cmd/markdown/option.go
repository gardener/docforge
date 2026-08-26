// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package markdown

// Markdown is the configuration options when you want to process markdown files
type Markdown struct {
	MarkdownEnabled bool `mapstructure:"markdown-enabled"`
}
