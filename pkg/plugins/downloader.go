// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package plugins

import (
	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/nodeplugins"
	"github.com/gardener/docforge/pkg/nodeplugins/downloader"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/writers"
)

// DownloaderPlugin handles resource downloading
type DownloaderPlugin struct {
	downloaderProcessor nodeplugins.Interface
}

// NewDownloaderPlugin creates a new downloader plugin
func NewDownloaderPlugin(registry registry.Interface, writer writers.Writer) *DownloaderPlugin {
	return &DownloaderPlugin{
		downloaderProcessor: downloader.NewPlugin(registry, writer),
	}
}

// Name returns the plugin name for identification
func (p *DownloaderPlugin) Name() string {
	return "downloader"
}

// ManifestTransformations returns transformations to apply during manifest parsing
func (p *DownloaderPlugin) ManifestTransformations() []manifest.NodeTransformation {
	return nil // No manifest transformations
}

// FinalNodeStructure is called after manifest resolution with the final document structure
func (p *DownloaderPlugin) FinalNodeStructure(documentNodes []*manifest.Node) error {
	return nil // No initialization needed
}

// Processor returns the processor name for node processing
func (p *DownloaderPlugin) Processor() string {
	return "downloader"
}

// Process processes a node using the old synchronous method
func (p *DownloaderPlugin) Process(node *manifest.Node) error {
	return p.downloaderProcessor.Process(node)
}

// ProcessNew processes a node using the new channel-based method
func (p *DownloaderPlugin) ProcessNew(node *manifest.Node) []chan nodeplugins.Status {
	return p.downloaderProcessor.ProcessNew(node)
}
