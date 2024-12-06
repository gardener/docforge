// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func configureFlags(command *cobra.Command) {
	command.Flags().StringP("destination", "d", "",
		"Destination path.")
	_ = vip.BindPFlag("destination", command.Flags().Lookup("destination"))

	command.Flags().StringP("manifest", "f", "",
		"Manifest path.")
	_ = vip.BindPFlag("manifest", command.Flags().Lookup("manifest"))

	command.Flags().String("resources-download-path", "__resources",
		"Resources download path.")
	_ = vip.BindPFlag("resources-download-path", command.Flags().Lookup("resources-download-path"))

	command.Flags().String("resources-website-path", "__resources",
		"The path in the website where resources will be accessed through.")
	_ = vip.BindPFlag("resources-website-path", command.Flags().Lookup("resources-website-path"))

	command.Flags().StringToString("github-oauth-token-map", map[string]string{},
		"GitHub personal tokens authorizing read access from repositories per GitHub instance. Note that if the GitHub token is already provided by `github-oauth-token` it will be overridden by it.")
	_ = vip.BindPFlag("github-oauth-token-map", command.Flags().Lookup("github-oauth-token-map"))

	command.Flags().String("github-info-destination", "",
		"If specified, docforge will download also additional github info for the files from the documentation structure into this destination.")
	_ = vip.BindPFlag("github-info-destination", command.Flags().Lookup("github-info-destination"))

	command.Flags().Bool("fail-fast", false,
		"Fail-fast vs fault tolerant operation.")
	_ = vip.BindPFlag("fail-fast", command.Flags().Lookup("fail-fast"))

	command.Flags().Bool("dry-run", false,
		"Runs the command end-to-end but instead of writing files, it will output the projected file/folder hierarchy to the standard output and statistics for the processing of each file.")
	_ = vip.BindPFlag("dry-run", command.Flags().Lookup("dry-run"))

	command.Flags().Int("document-workers", 25,
		"Number of parallel workers for document processing.")
	_ = vip.BindPFlag("document-workers", command.Flags().Lookup("document-workers"))

	command.Flags().Int("validation-workers", 10,
		"Number of parallel workers to validate the markdown links")
	_ = vip.BindPFlag("validation-workers", command.Flags().Lookup("validation-workers"))

	command.Flags().Int("download-workers", 10,
		"Number of workers downloading document resources in parallel.")
	_ = vip.BindPFlag("download-workers", command.Flags().Lookup("download-workers"))

	command.Flags().Bool("hugo", false,
		"Build documentation bundle for hugo.")
	_ = vip.BindPFlag("hugo", command.Flags().Lookup("hugo"))

	command.Flags().Bool("hugo-pretty-urls", true,
		"Build documentation bundle for hugo with pretty URLs (./sample.md -> ../sample). Only useful with --hugo=true")
	_ = vip.BindPFlag("hugo-pretty-urls", command.Flags().Lookup("hugo-pretty-urls"))

	command.Flags().String("hugo-base-url", "",
		"Rewrites the relative links of documentation files to root-relative where possible.")
	_ = vip.BindPFlag("hugo-base-url", command.Flags().Lookup("hugo-base-url"))

	command.Flags().StringSlice("hugo-section-files", []string{"readme.md", "README.md"},
		"When building a Hugo-compliant documentation bundle, files with filename matching one form this list (in that order) will be renamed to _index.md. Only useful with --hugo=true")
	_ = vip.BindPFlag("hugo-section-files", command.Flags().Lookup("hugo-section-files"))

	command.Flags().StringSlice("content-files-formats", []string{".md"},
		"Supported content format extensions (example: .md)")
	_ = vip.BindPFlag("content-files-formats", command.Flags().Lookup("content-files-formats"))

	command.Flags().Bool("skip-link-validation", false,
		"Links validation will be skipped")
	_ = vip.BindPFlag("skip-link-validation", command.Flags().Lookup("skip-link-validation"))

	command.Flags().StringSlice("hosts-to-report", []string{},
		"When a link has a host from the given array it will get reported")
	_ = vip.BindPFlag("hosts-to-report", command.Flags().Lookup("hosts-to-report"))

	cacheDir := ""
	userHomeDir, err := os.UserHomeDir()
	if err == nil {
		// default value $HOME/.docforge
		cacheDir = filepath.Join(userHomeDir, DocforgeHomeDir)
	}
	command.Flags().String("cache-dir", cacheDir,
		"Cache directory, used for repository cache.")
	_ = vip.BindPFlag("cache-dir", command.Flags().Lookup("cache-dir"))
}
