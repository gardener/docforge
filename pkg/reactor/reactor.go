// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reactor

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate -header ../../license_prefix.txt

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gardener/docforge/pkg/document"
	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/reactor/documentworker"
	"github.com/gardener/docforge/pkg/reactor/downloader"
	"github.com/gardener/docforge/pkg/reactor/githubinfo"
	"github.com/gardener/docforge/pkg/reactor/jobs"
	"github.com/gardener/docforge/pkg/reactor/linkvalidator"
	"github.com/gardener/docforge/pkg/readers"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/hashicorp/go-multierror"
	"k8s.io/klog/v2"
)

// NewReactor creates a Reactor from Config
func NewReactor(o Config) (*Reactor, error) {
	var (
		ghInfo      githubinfo.GitHubInfo
		ghInfoTasks jobs.QueueController
		err         error
	)
	reactorWG := &sync.WaitGroup{}

	rhRegistry := resourcehandlers.NewRegistry(o.ResourceHandlers...)

	dScheduler, downloadTasks, err := downloader.New(o.ResourceDownloadWorkersCount, o.FailFast, reactorWG, &readers.GenericReader{
		ResourceHandlers: rhRegistry,
	}, o.ResourceDownloadWriter)
	if err != nil {
		return nil, err
	}
	v, validatorTasks, err := linkvalidator.New(o.ValidationWorkersCount, o.FailFast, reactorWG, http.DefaultClient, rhRegistry)
	if err != nil {
		return nil, err
	}
	if !o.ValidateLinks {
		v = nil
	}
	if o.GitInfoWriter != nil {
		ghInfo, ghInfoTasks, err = githubinfo.New(o.ResourceDownloadWorkersCount, o.FailFast, reactorWG, &readers.GenericReader{
			ResourceHandlers: rhRegistry,
			IsGitHubInfo:     true,
		}, o.GitInfoWriter)
		if err != nil {
			return nil, err
		}
	}
	worker, docTasks, err := documentworker.New(o.DocumentWorkersCount, o.FailFast, reactorWG, &readers.GenericReader{ResourceHandlers: rhRegistry}, o.Writer, document.NewNodeContentProcessor(o.ResourcesPath, dScheduler, v, rhRegistry, o.Hugo), ghInfo)
	if err != nil {
		return nil, err
	}

	return &Reactor{
		Config:           o,
		ResourceHandlers: rhRegistry,
		DocumentWorker:   worker,
		DocumentTasks:    docTasks,
		DownloadTasks:    downloadTasks,
		GitHubInfoTasks:  ghInfoTasks,
		ValidatorTasks:   validatorTasks,
		reactorWaitGroup: reactorWG,
		sources:          make(map[string][]*manifest.Node),
	}, nil
}

// Run starts build operation on documentation
func (r *Reactor) Run(ctx context.Context, manifestUrl string) error {
	ctx, cancel := context.WithCancel(ctx)
	m := &manifest.Node{}
	var err error
	defer func() {
		cancel()
		if r.Config.DryRun {
			r.Config.DryRunWriter.Flush()
		}
	}()
	if m, err = manifest.ResolveManifest(manifestUrl, r.ResourceHandlers); err != nil {
		return fmt.Errorf("failed to resolve manifest %s. %+v", r.Config.ManifestPath, err)
	}
	if r.Config.Resolve {
		fmt.Println(m)
	}
	var errors *multierror.Error

	r.DownloadTasks.Start(ctx)
	r.ValidatorTasks.Start(ctx)
	if r.GitHubInfoTasks != nil {
		r.GitHubInfoTasks.Start(ctx)
	}
	r.DocumentWorker.NodeContentProcessor.Prepare(m.Structure)
	r.DocumentTasks.Start(ctx)

	documentNodes := manifest.GetAllNodes(m)
	for _, node := range documentNodes {
		r.DocumentTasks.AddTask(node)
	}
	r.reactorWaitGroup.Wait()
	r.DocumentTasks.Stop()
	if r.GitHubInfoTasks != nil {
		r.GitHubInfoTasks.Stop()
	}
	r.ValidatorTasks.Stop()
	r.DownloadTasks.Stop()

	klog.Infof("Document tasks processed: %d\n", r.DocumentTasks.GetProcessedTasksCount())
	klog.Infof("Download tasks processed: %d\n", r.DownloadTasks.GetProcessedTasksCount())
	if r.GitHubInfoTasks != nil {
		klog.Infof("GitHub info tasks processed: %d\n", r.GitHubInfoTasks.GetProcessedTasksCount())
	}
	klog.Infof("Validation tasks processed: %d\n", r.ValidatorTasks.GetProcessedTasksCount())

	for _, rhHost := range []string{"https://github.com", "https://github.tools.sap", "https://github.wdf.sap.corp"} {
		rh := r.ResourceHandlers.Get(rhHost)
		u, err := url.Parse(rhHost)
		if err != nil {
			return err
		}
		if rh != nil {
			l, rr, rt, err := rh.GetRateLimit(ctx)
			if err != nil {
				klog.Warningf("Error getting RateLimit for %s: %v\n", u.Host, err)
			} else if l > 0 && rr > 0 {
				klog.Infof("%s RateLimit: %d requests per hour, Remaining: %d, Reset after: %s\n", u.Host, l, rr, time.Until(rt).Round(time.Second))
			}
		}
	}

	errors = multierror.Append(errors, r.DocumentTasks.GetErrorList())
	errors = multierror.Append(errors, r.DownloadTasks.GetErrorList())
	errors = multierror.Append(errors, r.ValidatorTasks.GetErrorList())
	if r.GitHubInfoTasks != nil {
		errors = multierror.Append(errors, r.GitHubInfoTasks.GetErrorList())
	}
	return errors.ErrorOrNil()
}
