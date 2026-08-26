// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func configureFlags(command *cobra.Command, vip *viper.Viper) {
	command.Flags().StringP("destination", "d", "",
		"Path to the directory where the forged documentation bundle will be written.")
	_ = vip.BindPFlag("destination", command.Flags().Lookup("destination"))

	command.Flags().StringP("manifest", "f", "",
		"Path or URL of the documentation manifest file.")
	_ = vip.BindPFlag("manifest", command.Flags().Lookup("manifest"))

	command.Flags().StringToString("github-oauth-env-map", map[string]string{},
		"Map of GitHub host to environment variable name holding the access token (e.g. github.com=GITHUB_TOKEN). Required for authenticated requests.")
	_ = vip.BindPFlag("github-oauth-env-map", command.Flags().Lookup("github-oauth-env-map"))

	command.Flags().String("github-info-destination", "",
		"If set, write a .json sidecar per source file with GitHub commit metadata (author, contributors, lastmod, publishdate, SHA) into this subdirectory of --destination.")
	_ = vip.BindPFlag("github-info-destination", command.Flags().Lookup("github-info-destination"))

	command.Flags().Bool("fail-fast", false,
		"Stop immediately on the first processing error. Default (false) is fault-tolerant: log the error and continue with remaining files.")
	_ = vip.BindPFlag("fail-fast", command.Flags().Lookup("fail-fast"))

	command.Flags().Bool("dry-run", false,
		"Print the resolved manifest node tree to stdout and skip cleaning the destination. Does NOT prevent files from being written — node processing and downloads still run.")
	_ = vip.BindPFlag("dry-run", command.Flags().Lookup("dry-run"))

	command.Flags().Bool("clean-destination", false,
		"Remove the destination directory before writing. Ignored when --dry-run is set.")
	_ = vip.BindPFlag("clean-destination", command.Flags().Lookup("clean-destination"))

	command.Flags().Int("document-workers", 25,
		"Number of parallel workers for document processing.")
	_ = vip.BindPFlag("document-workers", command.Flags().Lookup("document-workers"))

	command.Flags().Int("download-workers", 10,
		"Number of workers downloading document resources in parallel.")
	_ = vip.BindPFlag("download-workers", command.Flags().Lookup("download-workers"))

	command.Flags().Bool("hugo", true,
		"Enable Hugo-specific processing: rename section index files to _index.md and rewrite links to pretty-URL format. Pass --hugo=false for non-Hugo targets.")
	_ = vip.BindPFlag("hugo", command.Flags().Lookup("hugo"))

	command.Flags().Bool("docsy-edit-this-page-enabled", false,
		"Add Docsy 'Edit this page' frontmatter fields to output files.")
	_ = vip.BindPFlag("docsy-edit-this-page-enabled", command.Flags().Lookup("docsy-edit-this-page-enabled"))

	command.Flags().Bool("hugo-pretty-urls", true,
		"Rewrite .md links to directory-style pretty URLs (./sample.md -> ../sample/). Only active when --hugo=true.")
	_ = vip.BindPFlag("hugo-pretty-urls", command.Flags().Lookup("hugo-pretty-urls"))

	command.Flags().String("hugo-base-url", "",
		"Rewrites the relative links of documentation files to root-relative where possible.")
	_ = vip.BindPFlag("hugo-base-url", command.Flags().Lookup("hugo-base-url"))

	command.Flags().StringSlice("hugo-structural-dirs", []string{},
		"List of directories that are part of the hugo bundle structure and should not be included in the resolved links.")
	_ = vip.BindPFlag("hugo-structural-dirs", command.Flags().Lookup("hugo-structural-dirs"))

	command.Flags().StringSlice("hugo-section-files", []string{"readme.md", "README.md"},
		"Files with a name matching any entry in this list are renamed to _index.md in the output. Only active when --hugo=true.")
	_ = vip.BindPFlag("hugo-section-files", command.Flags().Lookup("hugo-section-files"))

	command.Flags().StringSlice("content-files-formats", []string{},
		"File extensions to include in the output (e.g. .md,.html). When empty (the default), all file types are included.")
	_ = vip.BindPFlag("content-files-formats", command.Flags().Lookup("content-files-formats"))

	command.Flags().Bool("aliases-enabled", false,
		"Propagate Hugo aliases from parent dir frontmatter to child files.")
	_ = vip.BindPFlag("aliases-enabled", command.Flags().Lookup("aliases-enabled"))

	cacheDir := ""
	userHomeDir, err := os.UserHomeDir()
	if err == nil {
		// default value $HOME/.docforge
		cacheDir = filepath.Join(userHomeDir, DocforgeHomeDir)
	}
	command.Flags().String("cache-dir", cacheDir,
		"Directory for the repository HTTP cache.")
	_ = vip.BindPFlag("cache-dir", command.Flags().Lookup("cache-dir"))
}
