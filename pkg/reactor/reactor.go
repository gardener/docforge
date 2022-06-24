// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reactor

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate -header ../../license_prefix.txt

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/jobs"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/writers"
	"k8s.io/klog/v2"
)

// Options encapsulates the parameters for creating
// new Reactor objects with NewReactor
type Options struct {
	DocumentWorkersCount         int
	ValidationWorkersCount       int
	FailFast                     bool
	DestinationPath              string
	ResourcesPath                string
	ManifestPath                 string
	ResourceDownloadWorkersCount int
	ResourceDownloadWriter       writers.Writer
	GitInfoWriter                writers.Writer
	Writer                       writers.Writer
	ResourceHandlers             []resourcehandlers.ResourceHandler
	DryRunWriter                 writers.DryRunWriter
	Resolve                      bool
	Hugo                         *Hugo
}

// Hugo is the configuration options for creating HUGO implementations
type Hugo struct {
	Enabled        bool
	PrettyURLs     bool
	BaseURL        string
	IndexFileNames []string
}

// NewReactor creates a Reactor from Options
func NewReactor(o *Options) (*Reactor, error) {
	reactorWG := &sync.WaitGroup{}
	var ghInfo GitHubInfo
	var ghInfoTasks *jobs.JobQueue
	rhRegistry := resourcehandlers.NewRegistry(o.ResourceHandlers...)
	dWork, err := DownloadWorkFunc(&GenericReader{
		ResourceHandlers: rhRegistry,
	}, o.ResourceDownloadWriter)
	if err != nil {
		return nil, err
	}
	downloadTasks, err := jobs.NewJobQueue("Download", o.ResourceDownloadWorkersCount, dWork, o.FailFast, reactorWG)
	if err != nil {
		return nil, err
	}
	dScheduler := NewDownloadScheduler(downloadTasks)
	if o.GitInfoWriter != nil {
		ghInfoWork, err := GitHubInfoWorkerFunc(&GenericReader{
			ResourceHandlers: rhRegistry,
			IsGitHubInfo:     true,
		}, o.GitInfoWriter)
		if err != nil {
			return nil, err
		}
		ghInfoTasks, err = jobs.NewJobQueue("GitHubInfo", o.ResourceDownloadWorkersCount, ghInfoWork, o.FailFast, reactorWG)
		if err != nil {
			return nil, err
		}
		ghInfo = NewGitHubInfo(ghInfoTasks)
	}
	valWork, err := ValidateWorkerFunc(http.DefaultClient, rhRegistry)
	if err != nil {
		return nil, err
	}
	validatorTasks, err := jobs.NewJobQueue("Validator", o.ValidationWorkersCount, valWork, o.FailFast, reactorWG)
	if err != nil {
		return nil, err
	}
	v := NewValidator(validatorTasks)
	worker := &DocumentWorker{
		writer:               o.Writer,
		reader:               &GenericReader{ResourceHandlers: rhRegistry},
		NodeContentProcessor: NewNodeContentProcessor(o.ResourcesPath, dScheduler, v, rhRegistry, o.Hugo),
		gitHubInfo:           ghInfo,
	}
	docTasks, err := jobs.NewJobQueue("Document", o.DocumentWorkersCount, worker.Work, o.FailFast, reactorWG)
	if err != nil {
		return nil, err
	}
	r := &Reactor{
		Options:          o,
		ResourceHandlers: rhRegistry,
		DocumentWorker:   worker,
		DocumentTasks:    docTasks,
		DownloadTasks:    downloadTasks,
		GitHubInfoTasks:  ghInfoTasks,
		ValidatorTasks:   validatorTasks,
		reactorWaitGroup: reactorWG,
		sources:          make(map[string][]*api.Node),
	}
	return r, nil
}

// Reactor orchestrates the documentation build workflow
type Reactor struct {
	Options          *Options
	ResourceHandlers resourcehandlers.Registry
	DocumentWorker   *DocumentWorker
	DocumentTasks    *jobs.JobQueue
	DownloadTasks    *jobs.JobQueue
	GitHubInfoTasks  *jobs.JobQueue
	ValidatorTasks   *jobs.JobQueue
	// reactorWaitGroup used to determine when all parallel tasks are done
	reactorWaitGroup *sync.WaitGroup
	sources          map[string][]*api.Node
}

// Run starts build operation on documentation
func (r *Reactor) Run(ctx context.Context, manifest *api.Documentation, dryRun bool) error {
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		if r.Options.Resolve {
			if err := printResolved(manifest, os.Stdout); err != nil {
				klog.Errorf("failed to print resolved manifest: %s", err.Error())
			}
		}
		cancel()
		if dryRun {
			r.Options.DryRunWriter.Flush()
		}
	}()

	if err := r.ResolveManifest(ctx, manifest); err != nil {
		return fmt.Errorf("failed to resolve manifest: %s. %+v", r.Options.ManifestPath, err)
	}

	klog.V(4).Info("Building documentation structure\n\n")
	if err := r.Build(ctx, manifest.Structure); err != nil {
		return err
	}

	return nil
}

func printResolved(manifest *api.Documentation, writer io.Writer) error {
	s, err := api.Serialize(manifest)
	if err != nil {
		return fmt.Errorf("failed to serialize the manifest. %+v", err)
	}
	_, _ = writer.Write([]byte(s))
	_, _ = writer.Write([]byte("\n\n"))
	return nil
}
