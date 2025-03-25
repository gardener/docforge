// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/gardener/docforge/pkg/core"
	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/manifestplugins/alias"
	"github.com/gardener/docforge/pkg/manifestplugins/docsy"
	"github.com/gardener/docforge/pkg/manifestplugins/filetypefilter"
	manifestmarkdown "github.com/gardener/docforge/pkg/manifestplugins/markdown"
	"github.com/gardener/docforge/pkg/manifestplugins/persona"
	"github.com/gardener/docforge/pkg/nodeplugins"
	"github.com/gardener/docforge/pkg/nodeplugins/downloader"
	"github.com/gardener/docforge/pkg/nodeplugins/markdown"
	"github.com/gardener/docforge/pkg/osfakes/osshim"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	"github.com/spf13/viper"
)

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

	pluginTransformations := []manifest.NodeTransformation{}
	if options.Docsy.EditThisPageEnabled {
		docsyPlugin := docsy.Docsy{}
		pluginTransformations = append(pluginTransformations, docsyPlugin.PluginNodeTransformations()...)
	}
	if options.Persona.PersonaFilterEnabled {
		personaPlugin := persona.Persona{}
		pluginTransformations = append(pluginTransformations, personaPlugin.PluginNodeTransformations()...)
	}
	if options.Alias.AliasesEnabled {
		aliasPlugin := alias.Alias{}
		pluginTransformations = append(pluginTransformations, aliasPlugin.PluginNodeTransformations()...)
	}
	if options.Markdown.MarkdownEnabled {
		markdownPlugin := manifestmarkdown.Markdown{}
		pluginTransformations = append(pluginTransformations, markdownPlugin.PluginNodeTransformations()...)
	}

	fileTypeFilterPlugin := filetypefilter.FileTypeFilter{ContentFileFormats: options.Options.ContentFileFormats}
	pluginTransformations = append(pluginTransformations, fileTypeFilterPlugin.PluginNodeTransformations()...)

	documentNodes, err := manifest.ResolveManifest(manifestURL, rhRegistry, pluginTransformations...)
	if err != nil {
		return fmt.Errorf("failed to resolve manifest %s. %+v", config.ManifestPath, err)
	}
	if config.DryRun {
		fmt.Println(documentNodes[0])
	}

	// Stage 1
	reactorWGStage1 := &sync.WaitGroup{}
	mdPlugin, mdTasks, err := markdown.NewPlugin(config.DocumentWorkersCount, config.FailFast, reactorWGStage1, documentNodes, rhRegistry, config.Hugo, config.Writer, config.SkipLinkValidation, config.ValidationWorkersCount, config.HostsToReport, config.ResourceDownloadWorkersCount, config.GitInfoWriter)
	if err != nil {
		return err
	}
	dPlugin, downloadTasks, err := downloader.NewPlugin(config.ResourceDownloadWorkersCount, config.FailFast, reactorWGStage1, rhRegistry, config.Writer)
	if err != nil {
		return err
	}
	if err := core.Run(ctx, documentNodes, reactorWGStage1, []nodeplugins.Interface{mdPlugin, dPlugin}, append(mdTasks, downloadTasks)); err != nil {
		return err
	}
	// Stage 2 ...

	rhRegistry.LogRateLimits(ctx)
	return nil
}
