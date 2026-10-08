// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"os"
	"path"
	"slices"
	"sort"
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
		docsyPlugin := docsy.Docsy{Config: siteGenAdapter}
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
	if err := validateOutputCollisions(documentNodes, siteGenAdapter); err != nil {
		return err
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

// validateOutputCollisions detects cases where two or more manifest document nodes
// (.md files) would write to the same output path after index-file renaming. The
// check is case-insensitive so that README.md vs readme.md are treated as a
// collision. All collisions are reported in a single error; no file is written on
// error.
// Non-document resources (images, CSS, JS …) are intentionally excluded: they are
// not subject to index-file renaming and a general file-overwrite check would
// produce false positives for manifests that intentionally override a resource from
// one source with another.
func validateOutputCollisions(nodes []*manifest.Node, cfg sitegen.Config) error {
	type entry struct {
		origPath string
		source   string
	}
	seen := make(map[string][]entry)

	for _, node := range nodes {
		if node.Type != "file" {
			continue
		}
		// Only check Markdown documents; skip images, CSS, JS, etc.
		if !strings.HasSuffix(strings.ToLower(node.Name()), ".md") {
			continue
		}
		outName := node.Name()
		if cfg != nil && cfg.Enabled() && cfg.IsIndexFile(outName) {
			if target := cfg.IndexFileName(); target != "" {
				outName = target
			}
		}
		key := strings.ToLower(path.Join(node.Path, outName))

		src := node.Source
		if src == "" && len(node.MultiSource) > 0 {
			src = strings.Join(node.MultiSource, ", ")
		}
		seen[key] = append(seen[key], entry{origPath: node.NodePath(), source: src})
	}

	var collisions []string
	for outPath, entries := range seen {
		if len(entries) < 2 {
			continue
		}
		msg := fmt.Sprintf("output path %q is claimed by %d nodes:", outPath, len(entries))
		for _, e := range entries {
			if e.source != "" {
				msg += fmt.Sprintf("\n  %s (source: %q)", e.origPath, e.source)
			} else {
				msg += fmt.Sprintf("\n  %s (stub, no source)", e.origPath)
			}
		}
		collisions = append(collisions, msg)
	}
	if len(collisions) == 0 {
		return nil
	}
	sort.Strings(collisions)
	return fmt.Errorf("%d output-path collision(s) detected:\n%s",
		len(collisions), strings.Join(collisions, "\n"))
}
