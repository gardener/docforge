// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package manifest

// ParsingOptions Options that are given to the parser in the api package
type ParsingOptions struct {
	ExtractedFilesFormats []string `mapstructure:"extracted-files-formats"`
	Hugo                  bool     `mapstructure:"hugo"`
}
