// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/gardener/docforge/pkg/core"
	"github.com/gardener/docforge/pkg/core/manifest"
	"github.com/gardener/docforge/pkg/core/registry"
	"github.com/gardener/docforge/pkg/core/registry/repositoryhost"
	"github.com/gardener/docforge/pkg/osfakes/osshim"
	"github.com/gardener/docforge/pkg/plugins"
	"github.com/gardener/docforge/pkg/plugins/downloader"
	"github.com/gardener/docforge/pkg/plugins/markdown"
	"github.com/gardener/docforge/pkg/plugins/persona"
	"github.com/spf13/viper"
)

// TODO remove the ignore
//
//gocyclo:ignore
func exec(ctx context.Context, vip *viper.Viper) error {
	var (
		rhs     []repositoryhost.Interface
		options options
	)

	err := vip.Unmarshal(&options)
	existsPath := slices.ContainsFunc(options.HugoStructuralDirs, func(dir string) bool {
		return strings.Contains(dir, "/")
	})
	if existsPath {
		return fmt.Errorf("hugo-structural-dirs contains a path instead a directory name")
	}
	localRH := []repositoryhost.Interface{}
	for resource, mapped := range options.ResourceMappings {
		localRH = append(localRH, repositoryhost.NewLocal(&osshim.OsShim{}, resource, mapped))
	}
	if err != nil {
		return err
	}
	if rhs, err = initRepositoryHosts(ctx, options.InitOptions); err != nil {
		return err
	}

	config := getReactorConfig(options.Options, options.Hugo, rhs)
	manifestURL := options.ManifestPath

	rhRegistry := registry.NewRegistry(append(localRH, config.RepositoryHosts...)...)

	// Create unified plugin registry and register all plugins upfront
	pluginRegistry := plugins.NewRegistry()

	// Register downloader plugin
	pluginRegistry.Register(downloader.New(rhRegistry, config.Writer))

	// Register all plugins based on configuration
	if options.Alias.AliasesEnabled {
		pluginRegistry.Register(&plugins.AliasPlugin{})
	}
	if options.Markdown.MarkdownEnabled {
		pluginRegistry.Register(markdown.New(rhRegistry, config.Hugo, config.Writer, config.SkipLinkValidation))
	}
	if options.Docsy.EditThisPageEnabled {
		pluginRegistry.Register(&plugins.DocsyPlugin{})
	}
	if len(options.Options.ContentFileFormats) > 0 {
		pluginRegistry.Register(plugins.NewFileTypeFilterPlugin(options.Options.ContentFileFormats))
	}
	if options.Persona.PersonaFilterEnabled {
		pluginRegistry.Register(persona.New(config.Writer))
	}

	// Phase 1: Get manifest transformations and resolve manifest
	pluginTransformations := pluginRegistry.GetManifestTransformations()
	documentNodes, err := manifest.ResolveManifest(manifestURL, rhRegistry, pluginTransformations...)
	if err != nil {
		return fmt.Errorf("failed to resolve manifest %s. %+v", config.ManifestPath, err)
	}
	if config.DryRun {
		fmt.Println(documentNodes[0])
	}

	// Phase 2: Set final node structure for all plugins
	if err := pluginRegistry.SetFinalNodeStructure(documentNodes); err != nil {
		return err
	}

	// Phase 3: Get node processors and run processing
	nodeProcessors := pluginRegistry.GetNodeProcessors()
	if err := core.Run(ctx, documentNodes, nodeProcessors, options.DeferredLinkValidation, rhRegistry, config.HostsToReport, config.ValidationWorkersCount); err != nil {
		return err
	}

	rhRegistry.LogRateLimits(ctx)
	return nil
}
