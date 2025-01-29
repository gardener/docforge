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

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/osfakes/osshim"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	"github.com/gardener/docforge/pkg/workers/document"
	"github.com/gardener/docforge/pkg/workers/githubinfo"
	"github.com/gardener/docforge/pkg/workers/linkvalidator"
	"github.com/gardener/docforge/pkg/workers/resourcedownloader"
	"github.com/gardener/docforge/pkg/workers/taskqueue"
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
	var (
		ghInfo      githubinfo.GitHubInfo
		ghInfoTasks taskqueue.QueueController
	)
	reactorWG := &sync.WaitGroup{}

	rhRegistry := registry.NewRegistry(append(localRH, config.RepositoryHosts...)...)
	documentNodes, err := manifest.ResolveManifest(manifestURL, rhRegistry, options.Options.ContentFileFormats)
	if err != nil {
		return fmt.Errorf("failed to resolve manifest %s. %+v", config.ManifestPath, err)
	}
	if config.DryRun {
		fmt.Println(documentNodes[0])
	}

	dScheduler, downloadTasks, err := resourcedownloader.New(config.ResourceDownloadWorkersCount, config.FailFast, reactorWG, rhRegistry, config.ResourceDownloadWriter)
	if err != nil {
		return err
	}
	v, validatorTasks, err := linkvalidator.New(config.ValidationWorkersCount, config.FailFast, reactorWG, rhRegistry, config.HostsToReport)
	if err != nil {
		return err
	}
	docProcessor, docTasks, err := document.New(config.DocumentWorkersCount, config.FailFast, reactorWG, documentNodes, config.ResourcesWebsitePath, dScheduler, v, rhRegistry, config.Hugo, config.Writer, config.SkipLinkValidation)
	if err != nil {
		return err
	}

	qcc := taskqueue.NewQueueControllerCollection(reactorWG, downloadTasks, validatorTasks, docTasks)

	if config.GitInfoWriter != nil {
		ghInfo, ghInfoTasks, err = githubinfo.New(config.ResourceDownloadWorkersCount, config.FailFast, reactorWG, rhRegistry, config.GitInfoWriter)
		if err != nil {
			return err
		}
		for _, node := range documentNodes {
			ghInfo.WriteGitHubInfo(node)
		}
		qcc.Add(ghInfoTasks)
	}

	for _, node := range documentNodes {
		docProcessor.ProcessNode(node)
	}

	qcc.Start(ctx)
	qcc.Wait()
	qcc.Stop()
	qcc.LogTaskProcessed()
	rhRegistry.LogRateLimits(ctx)
	return qcc.GetErrorList().ErrorOrNil()
}
