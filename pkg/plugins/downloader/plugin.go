// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package downloader

import (
	"context"
	"path"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/plugins"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/writers"
)

// ResourceDownloadWorker is the structure that processes downloads
type ResourceDownloadWorker struct {
	registry registry.Interface
	writer   writers.Writer
}

// Plugin handles resource downloading
type Plugin struct {
	downloader ResourceDownloadWorker
}

// New creates a new downloader plugin
func New(registry registry.Interface, writer writers.Writer) *Plugin {
	return &Plugin{
		downloader: ResourceDownloadWorker{
			registry: registry,
			writer:   writer,
		},
	}
}

// Name returns the plugin name for identification
func (p *Plugin) Name() string {
	return "downloader"
}

// ManifestTransformations returns transformations to apply during manifest parsing
func (p *Plugin) ManifestTransformations() []manifest.NodeTransformation {
	return nil // No manifest transformations needed
}

// FinalNodeStructure is called after manifest resolution with the final document structure
func (p *Plugin) FinalNodeStructure(documentNodes []*manifest.Node) error {
	return nil // No initialization needed
}

// Processor returns the processor name for node processing
func (p *Plugin) Processor() string {
	return "downloader"
}

// Process processes a node using the old synchronous method
func (p *Plugin) Process(node *manifest.Node) error {
	return nil // Downloader only uses channel-based processing
}

// ProcessNew processes a node using the new channel-based method
func (p *Plugin) ProcessNew(node *manifest.Node) []chan plugins.Status {
	out := make(chan plugins.Status)
	go func() {
		defer close(out)
		err := p.downloader.Download(context.TODO(), node.Source, node.NodePath())
		out <- plugins.NewStatus(err)
	}()
	return []chan plugins.Status{out}
}

// Download downloads source to destinationPath
func (d *ResourceDownloadWorker) Download(ctx context.Context, source string, destinationPath string) error {
	resourceURL, err := d.registry.ResourceURL(source)
	if err != nil {
		return err
	}
	blob, err := d.registry.Read(ctx, resourceURL.ResourceURL())
	if err != nil {
		return err
	}

	if err = d.writer.Write(path.Base(destinationPath), path.Dir(destinationPath), blob, nil, nil); err != nil {
		return err
	}
	return nil
}

// Download is a convenience method for testing that delegates to the internal downloader
func (p *Plugin) Download(ctx context.Context, source string, destinationPath string) error {
	return p.downloader.Download(ctx, source, destinationPath)
}
