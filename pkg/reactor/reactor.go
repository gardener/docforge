// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reactor

import (
	"context"
	"io"
	"os"
	"text/template"

	"github.com/gardener/docforge/pkg/processors"
	"k8s.io/klog/v2"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/writers"
)

// Options encapsulates the parameters for creating
// new Reactor objects with NewReactor
type Options struct {
	MaxWorkersCount              int
	MinWorkersCount              int
	FailFast                     bool
	DestinationPath              string
	ResourcesPath                string
	ManifestAbsPath              string
	ResourceDownloadWorkersCount int
	RewriteEmbedded              bool
	processors.Processor
	ResourceDownloadWriter writers.Writer
	GitInfoWriter          writers.Writer
	Writer                 writers.Writer
	ResourceHandlers       []resourcehandlers.ResourceHandler
	DryRunWriter           writers.DryRunWriter
	Resolve                bool
	GlobalLinksConfig      *api.Links
}

// NewReactor creates a Reactor from Options
func NewReactor(o *Options) *Reactor {
	var gitInfoController GitInfoController
	rhRegistry := resourcehandlers.NewRegistry(o.ResourceHandlers...)
	downloadController := NewDownloadController(nil, o.ResourceDownloadWriter, o.ResourceDownloadWorkersCount, o.FailFast, rhRegistry)
	if o.GitInfoWriter != nil {
		gitInfoController = NewGitInfoController(nil, o.GitInfoWriter, o.ResourceDownloadWorkersCount, o.FailFast, rhRegistry)
	}
	worker := &DocumentWorker{
		Writer:               o.Writer,
		Reader:               &GenericReader{rhRegistry},
		NodeContentProcessor: NewNodeContentProcessor(o.ResourcesPath, o.GlobalLinksConfig, downloadController, o.FailFast, o.RewriteEmbedded, rhRegistry),
		Processor:            o.Processor,
		GitHubInfoController: gitInfoController,
		templates:            map[string]*template.Template{},
	}
	docController := NewDocumentController(worker, o.MaxWorkersCount, o.FailFast)
	r := &Reactor{
		FailFast:           o.FailFast,
		ResourceHandlers:   rhRegistry,
		DocController:      docController,
		DownloadController: downloadController,
		GitInfoController:  gitInfoController,
		DryRunWriter:       o.DryRunWriter,
		Resolve:            o.Resolve,
		manifestAbsPath:    o.ManifestAbsPath,
	}
	return r
}

// Reactor orchestrates the documentation build workflow
type Reactor struct {
	FailFast           bool
	ResourceHandlers   resourcehandlers.Registry
	DocController      DocumentController
	DownloadController DownloadController
	GitInfoController  GitInfoController
	DryRunWriter       writers.DryRunWriter
	Resolve            bool
	manifestAbsPath    string
}

// Run starts build operation on documentation
func (r *Reactor) Run(ctx context.Context, manifest *api.Documentation, dryRun bool) error {
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		if r.Resolve {
			if err := printResolved(ctx, manifest, os.Stdout); err != nil {
				klog.Errorf("failed to print resolved manifest: %s", err.Error())
			}
		}
		cancel()
		if dryRun {
			r.DryRunWriter.Flush()
		}
	}()

	if err := ResolveManifest(ctx, manifest, r.ResourceHandlers, r.manifestAbsPath); err != nil {
		return err
	}

	klog.V(4).Info("Building documentation structure\n\n")
	if err := r.Build(ctx, manifest.Structure); err != nil {
		return err
	}

	return nil
}

func printResolved(ctx context.Context, manifest *api.Documentation, writer io.Writer) error {
	// for _, node := range manifest.Structure {
	// 	if links := resolveNodeLinks(node, manifest.Links); len(links) > 0 {
	// 		for _, l := range links {
	// 			l := mergeLinks(node.ResolvedLinks, l)
	// 			node.ResolvedLinks = l
	// 		}
	// 	}
	// 	// remove resolved links for container nodes
	// 	if node.Nodes != nil {
	// 		node.ResolvedLinks = nil
	// 	}
	// }
	s, err := api.Serialize(manifest)
	if err != nil {
		return err
	}
	writer.Write([]byte(s))
	writer.Write([]byte("\n\n"))
	return nil
}
