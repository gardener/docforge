// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"github.com/gardener/docforge/cmd/hugo"
	"github.com/gardener/docforge/pkg/readers/repositoryhosts"
	"github.com/gardener/docforge/pkg/writers"
)

// Options encapsulates the parameters for creating
// new Reactor objects with NewReactor
type Options struct {
	DocumentWorkersCount         int      `mapstructure:"document-workers"`
	ValidationWorkersCount       int      `mapstructure:"validation-workers"`
	FailFast                     bool     `mapstructure:"fail-fast"`
	DestinationPath              string   `mapstructure:"destination"`
	ResourcesPath                string   `mapstructure:"resources-download-path"`
	ManifestPath                 string   `mapstructure:"manifest"`
	ResourceDownloadWorkersCount int      `mapstructure:"download-workers"`
	GhInfoDestination            string   `mapstructure:"github-info-destination"`
	DryRun                       bool     `mapstructure:"dry-run"`
	Resolve                      bool     `mapstructure:"resolve"`
	ExtractedFilesFormats        []string `mapstructure:"extracted-files-formats"`
	ValidateLinks                bool     `mapstructure:"validate-links"`
}

// Writers struct that collects all the writesr
type Writers struct {
	ResourceDownloadWriter writers.Writer
	GitInfoWriter          writers.Writer
	Writer                 writers.Writer
	DryRunWriter           writers.DryRunWriter
}

// Config configuration of the reactor
type Config struct {
	Options
	Writers
	hugo.Hugo
	RepositoryHosts []repositoryhosts.RepositoryHost
}
