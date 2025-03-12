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
	"github.com/gardener/docforge/pkg/manifestplugins/docsy"
	"github.com/gardener/docforge/pkg/manifestplugins/filetypefilter"
	"github.com/gardener/docforge/pkg/nodeplugins"
	"github.com/gardener/docforge/pkg/nodeplugins/downloader"
	"github.com/gardener/docforge/pkg/nodeplugins/markdown"
	"github.com/gardener/docforge/pkg/osfakes/osshim"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
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
	klog.Infof("Manifest: %s", options.ManifestPath)
	localRH := []repositoryhost.Interface{}
	for resource, mapped := range options.ResourceMappings {
		localRH = append(localRH, repositoryhost.NewLocal(&osshim.OsShim{}, resource, mapped))
		klog.Infof("%s -> %s", resource, mapped)
	}
	klog.Infof("Output dir: %s", options.DestinationPath)
	if err != nil {
		return err
	}
	if rhs, err = initRepositoryHosts(ctx, options.InitOptions); err != nil {
		return err
	}

	config := getReactorConfig(options.Options, options.Hugo, rhs)
	manifestURL := options.ManifestPath
	reactorWG := &sync.WaitGroup{}

	rhRegistry := registry.NewRegistry(append(localRH, config.RepositoryHosts...)...)

	pluginTransformations := []manifest.NodeTransformation{}
	if options.Docsy.EditThisPageEnabled {
		docsyPlugin := docsy.Docsy{}
		pluginTransformations = append(pluginTransformations, docsyPlugin.PluginNodeTransformations()...)
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

	mdPlugin, mdTasks, err := markdown.NewPlugin(config.DocumentWorkersCount, config.FailFast, reactorWG, documentNodes, rhRegistry, config.Hugo, config.Writer, config.SkipLinkValidation, config.ValidationWorkersCount, config.HostsToReport, config.ResourceDownloadWorkersCount, config.GitInfoWriter)
	if err != nil {
		return err
	}
	dPlugin, downloadTasks, err := downloader.NewPlugin(config.ResourceDownloadWorkersCount, config.FailFast, reactorWG, rhRegistry, config.Writer)
	if err != nil {
		return err
	}
	err = core.Run(ctx, documentNodes, reactorWG, []nodeplugins.Interface{mdPlugin, dPlugin}, append(mdTasks, downloadTasks))
	rhRegistry.LogRateLimits(ctx)
	return err
}
