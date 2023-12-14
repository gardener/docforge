package app

import (
	"context"
	"fmt"
	"sync"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/readers/repositoryhosts"
	documentworker "github.com/gardener/docforge/pkg/workers/document"
	"github.com/gardener/docforge/pkg/workers/downloader"
	"github.com/gardener/docforge/pkg/workers/githubinfo"
	"github.com/gardener/docforge/pkg/workers/linkvalidator"
	"github.com/gardener/docforge/pkg/workers/taskqueue"
	"k8s.io/klog/v2"
)

func exec(ctx context.Context) error {
	var (
		rhs     []repositoryhosts.RepositoryHost
		options options
	)

	err := vip.Unmarshal(&options)
	klog.Infof("Manifest: %s", options.ManifestPath)
	for resource, mapped := range options.ResourceMappings {
		klog.Infof("%s -> %s", resource, mapped)
	}
	klog.Infof("Output dir: %s", options.DestinationPath)
	if err != nil {
		return err
	}
	if rhs, err = initRepositoryHosts(ctx, options.RepositoryHostOptions, options.ParsingOptions); err != nil {
		return err
	}

	config := getReactorConfig(options.Options, options.Hugo, rhs)
	manifestURL := options.ManifestPath
	var (
		ghInfo      githubinfo.GitHubInfo
		ghInfoTasks taskqueue.QueueController
	)
	reactorWG := &sync.WaitGroup{}

	rhRegistry := repositoryhosts.NewRegistry(config.RepositoryHosts...)
	documentNodes, err := manifest.ResolveManifest(manifestURL, rhRegistry)
	if err != nil {
		return fmt.Errorf("failed to resolve manifest %s. %+v", config.ManifestPath, err)
	}
	if config.Resolve {
		fmt.Println(documentNodes[0])
	}

	dScheduler, downloadTasks, err := downloader.New(config.ResourceDownloadWorkersCount, config.FailFast, reactorWG, rhRegistry, config.ResourceDownloadWriter)
	if err != nil {
		return err
	}
	v, validatorTasks, err := linkvalidator.New(config.ValidationWorkersCount, config.FailFast, reactorWG, rhRegistry)
	if err != nil {
		return err
	}
	if !config.ValidateLinks {
		v = nil
	}
	docProcessor, docTasks, err := documentworker.New(config.DocumentWorkersCount, config.FailFast, reactorWG, documentNodes, config.ResourcesPath, dScheduler, v, rhRegistry, config.Hugo, config.Writer)
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

	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
		if config.DryRun {
			config.DryRunWriter.Flush()
		}
	}()
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
