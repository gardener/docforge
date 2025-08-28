// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"github.com/gardener/docforge/cmd/hugo"
	"github.com/gardener/docforge/pkg/core/registry/repositoryhost"
	"github.com/gardener/docforge/pkg/osfakes/osshim"
)

// Options encapsulates the parameters for creating
// new Reactor objects
type Options struct {
	ValidationWorkersCount int      `mapstructure:"validation-workers"`
	DestinationPath        string   `mapstructure:"destination"`
	ManifestPath           string   `mapstructure:"manifest"`
	DryRun                 bool     `mapstructure:"dry-run"`
	ContentFileFormats     []string `mapstructure:"content-files-formats"`
	HostsToReport          []string `mapstructure:"hosts-to-report"`
	SkipLinkValidation     bool     `mapstructure:"skip-link-validation"`
	DeferredLinkValidation bool     `mapstructure:"deferred-link-validation"`
}

// Writers struct that collects filesystem interface and path
type Writers struct {
	FS       osshim.Os
	RootPath string
}

// Config configuration of the reactor
type Config struct {
	Options
	Writers
	hugo.Hugo
	RepositoryHosts []repositoryhost.Interface
}
