// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package downloader

import (
	"context"
	"fmt"
	"path"
	"path/filepath"

	"github.com/gardener/docforge/pkg/core"
	"github.com/gardener/docforge/pkg/core/manifest"
	"github.com/gardener/docforge/pkg/core/registry"
	"github.com/gardener/docforge/pkg/osshim/filesystem"
)

// writeFile writes a simple file to the filesystem
func writeFile(fs filesystem.Interface, rootPath, name, path string, content []byte) error {
	if len(content) == 0 {
		return nil
	}

	p := filepath.Join(rootPath, path)
	if err := fs.MkdirAll(p, 0755); err != nil {
		return err
	}

	filePath := filepath.Join(p, name)
	if err := fs.WriteFile(filePath, content, 0644); err != nil {
		return fmt.Errorf("error writing %s: %v", filePath, err)
	}
	return nil
}

// ResourceDownloadWorker is the structure that processes downloads
type ResourceDownloadWorker struct {
	registry registry.Interface
	fs       filesystem.Interface
	rootPath string
}

// Plugin handles resource downloading
type Plugin struct {
	downloader ResourceDownloadWorker
}

// New creates a new downloader plugin
func New(registry registry.Interface, fs filesystem.Interface, rootPath string) *Plugin {
	return &Plugin{
		downloader: ResourceDownloadWorker{
			registry: registry,
			fs:       fs,
			rootPath: rootPath,
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
func (p *Plugin) ProcessNew(node *manifest.Node) []chan core.Status {
	out := make(chan core.Status)
	go func() {
		defer close(out)
		err := p.downloader.Download(context.TODO(), node.Source, node.NodePath())
		out <- core.NewStatus(err)
	}()
	return []chan core.Status{out}
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

	if err = writeFile(d.fs, d.rootPath, path.Base(destinationPath), path.Dir(destinationPath), blob); err != nil {
		return err
	}
	return nil
}

// Download is a convenience method for testing that delegates to the internal downloader
func (p *Plugin) Download(ctx context.Context, source string, destinationPath string) error {
	return p.downloader.Download(ctx, source, destinationPath)
}
