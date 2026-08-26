// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

	if err := cleanDestination(config.CleanDestination, config.DryRun, config.DestinationPath); err != nil {
		return err
	}

	manifestURL := options.ManifestPath
	// Auto-detect local manifest path: if the value is a local filesystem path rather than
	// a remote resource URL, resolve it to an absolute file:// URL and register a LocalPath
	// host so the registry can serve it.  This is the single dispatch point for
	// local-manifest vs remote-manifest; the sources referenced inside the manifest are
	// orthogonal and continue to be routed by their own URLs (remote → GitHub host,
	// local-relative → LocalPath host via ResolveRelativeLink).
	if repositoryhost.IsLocalPath(manifestURL) {
		rawPath := strings.TrimPrefix(manifestURL, "file://")
		absPath, err := filepath.Abs(rawPath)
		if err != nil {
			return fmt.Errorf("failed to resolve manifest path %q: %w", manifestURL, err)
		}
		// Resolve symlinks so localDir is scoped to the real directory, not a symlink to it.
		// This ensures the LocalPath containment check uses consistent real paths.
		realPath, err := filepath.EvalSymlinks(absPath)
		if err != nil {
			return fmt.Errorf("failed to resolve manifest path %q: %w", manifestURL, err)
		}
		manifestURL = "file://" + realPath
		// Scope the host to the manifest's real directory so that a malicious remote
		// sub-manifest cannot inject file:// URLs pointing outside that directory.
		localRH = append(localRH, repositoryhost.NewLocalPath(&osshim.OsShim{}, filepath.Dir(realPath)))
	}

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
	mdPlugin, mdTasks, err := markdown.NewPlugin(config.DocumentWorkersCount, config.FailFast, reactorWGStage1, documentNodes, rhRegistry, config.Hugo, config.Writer, config.ResourceDownloadWorkersCount, config.GitInfoWriter)
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
