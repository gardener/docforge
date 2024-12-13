// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"github.com/gardener/docforge/cmd/hugo"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	"github.com/gardener/docforge/pkg/writers"
)

// Options encapsulates the parameters for creating
// new Reactor objects
type Options struct {
	ValidationWorkersCount       int      `mapstructure:"validation-workers"`
	FailFast                     bool     `mapstructure:"fail-fast"`
	DestinationPath              string   `mapstructure:"destination"`
	ResourcesDownloadPath        string   `mapstructure:"resources-download-path"`
	ResourcesWebsitePath         string   `mapstructure:"resources-website-path"`
	ManifestPath                 string   `mapstructure:"manifest"`
	ResourceDownloadWorkersCount int      `mapstructure:"download-workers"`
	GhInfoDestination            string   `mapstructure:"github-info-destination"`
	DryRun                       bool     `mapstructure:"dry-run"`
	ContentFileFormats           []string `mapstructure:"content-files-formats"`
	HostsToReport                []string `mapstructure:"hosts-to-report"`
	SkipLinkValidation           bool     `mapstructure:"skip-link-validation"`
}

// Writers struct that collects all the writesr
type Writers struct {
	ResourceDownloadWriter writers.Writer
	GitInfoWriter          writers.Writer
	Writer                 writers.Writer
}

// Config configuration of the reactor
type Config struct {
	Options
	Writers
	hugo.Hugo
	RepositoryHosts []repositoryhost.Interface
}
