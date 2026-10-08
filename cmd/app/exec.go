// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/gardener/docforge/pkg/core"
	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/manifestplugins/alias"
	"github.com/gardener/docforge/pkg/manifestplugins/docsy"
	"github.com/gardener/docforge/pkg/manifestplugins/filetypefilter"
	manifestmarkdown "github.com/gardener/docforge/pkg/manifestplugins/markdown"
	"github.com/gardener/docforge/pkg/nodeplugins"
	"github.com/gardener/docforge/pkg/nodeplugins/downloader"
	"github.com/gardener/docforge/pkg/nodeplugins/markdown"
	"github.com/gardener/docforge/pkg/osfakes/osshim"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	"github.com/gardener/docforge/pkg/sitegen"
	"github.com/spf13/viper"

	"github.com/gardener/docforge/cmd/hugo"
	"github.com/gardener/docforge/cmd/vitepress"
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

	siteGenAdapter := resolveSiteGenAdapter(vip.GetString("site-generator"), options.Hugo, options.VitePress)
	config := getReactorConfig(options.Options, siteGenAdapter, rhs)

	if err := cleanDestination(config.CleanDestination, config.DryRun, config.DestinationPath); err != nil {
		return err
	}

	manifestURL := options.ManifestPath

	rhRegistry := registry.NewRegistry(append(localRH, config.RepositoryHosts...)...)

	pluginTransformations := []manifest.NodeTransformation{}
	if options.Alias.AliasesEnabled {
		aliasPlugin := alias.Alias{}
		pluginTransformations = append(pluginTransformations, aliasPlugin.PluginNodeTransformations()...)
	}
	if options.Markdown.MarkdownEnabled {
		markdownPlugin := manifestmarkdown.Markdown{}
		pluginTransformations = append(pluginTransformations, markdownPlugin.PluginNodeTransformations()...)
	}
	if options.Docsy.EditThisPageEnabled {
		docsyPlugin := docsy.Docsy{}
		pluginTransformations = append(pluginTransformations, docsyPlugin.PluginNodeTransformations()...)
	}

	if len(options.Options.ContentFileFormats) > 0 {
		fileTypeFilterPlugin := filetypefilter.FileTypeFilter{ContentFileFormats: options.Options.ContentFileFormats}
		pluginTransformations = append(pluginTransformations, fileTypeFilterPlugin.PluginNodeTransformations()...)
	}

	documentNodes, err := manifest.ResolveManifest(manifestURL, rhRegistry, pluginTransformations...)
	if err != nil {
		return fmt.Errorf("failed to resolve manifest %s. %+v", config.ManifestPath, err)
	}
	if config.DryRun {
		fmt.Println(documentNodes[0])
	}

	additionalNodePlugins := []nodeplugins.Interface{}
	// Stage 1
	reactorWGStage1 := &sync.WaitGroup{}
	mdPlugin, mdTasks, err := markdown.NewPlugin(config.DocumentWorkersCount, config.FailFast, reactorWGStage1, documentNodes, rhRegistry, config.SiteGen, config.Writer, config.ResourceDownloadWorkersCount, config.GitInfoWriter)
	if err != nil {
		return err
	}
	dPlugin, downloadTasks, err := downloader.NewPlugin(config.ResourceDownloadWorkersCount, config.FailFast, reactorWGStage1, rhRegistry, config.Writer)
	if err != nil {
		return err
	}
	if err := core.Run(ctx, documentNodes, reactorWGStage1, append([]nodeplugins.Interface{mdPlugin, dPlugin}, additionalNodePlugins...), append(mdTasks, downloadTasks)); err != nil {
		return err
	}
	// Stage 2 ...

	rhRegistry.LogRateLimits(ctx)
	return nil
}

// resolveSiteGenAdapter selects the sitegen.Config adapter based on the
// --site-generator flag. When site-generator is empty (unset), it falls back
// to the legacy --hugo flag so existing configs continue to work unchanged.
func resolveSiteGenAdapter(siteGenerator string, h hugo.Hugo, vp vitepress.VitePress) sitegen.Config {
	switch siteGenerator {
	case "vitepress":
		vp.Enabled = true
		return vitepress.NewAdapter(vp)
	case "hugo":
		h.Enabled = true
		return hugo.NewAdapter(h)
	case "none":
		h.Enabled = false
		return hugo.NewAdapter(h)
	default: // "" — not set: honour the legacy --hugo flag value
		return hugo.NewAdapter(h)
	}
}

func cleanDestination(clean, dryRun bool, destinationPath string) error {
	if !clean || dryRun {
		return nil
	}
	if destinationPath == "" {
		return fmt.Errorf("--clean-destination requires --destination to be set")
	}
	if err := os.RemoveAll(destinationPath); err != nil {
		return fmt.Errorf("failed to clean destination %q: %w", destinationPath, err)
	}
	return nil
}
