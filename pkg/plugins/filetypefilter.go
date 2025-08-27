// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package plugins

import (
	"path/filepath"
	"slices"

	"github.com/gardener/docforge/pkg/core/manifest"
	"github.com/gardener/docforge/pkg/registry"
)

// FileTypeFilterPlugin filters content by file types
type FileTypeFilterPlugin struct {
	ContentFileFormats []string
}

// NewFileTypeFilterPlugin creates a new file type filter plugin
func NewFileTypeFilterPlugin(contentFileFormats []string) *FileTypeFilterPlugin {
	return &FileTypeFilterPlugin{
		ContentFileFormats: contentFileFormats,
	}
}

// Name returns the plugin name for identification
func (p *FileTypeFilterPlugin) Name() string {
	return "filetypefilter"
}

// ManifestTransformations returns transformations to apply during manifest parsing
func (p *FileTypeFilterPlugin) ManifestTransformations() []manifest.NodeTransformation {
	return []manifest.NodeTransformation{p.filterByFileType}
}

// FinalNodeStructure is called after manifest resolution with the final document structure
func (p *FileTypeFilterPlugin) FinalNodeStructure(documentNodes []*manifest.Node) error {
	return nil // No initialization needed
}

// Processor returns the processor name for node processing
func (p *FileTypeFilterPlugin) Processor() string {
	return "" // No node processing
}

// Process processes a node using the old synchronous method
func (p *FileTypeFilterPlugin) Process(*manifest.Node) error {
	return nil // Not used
}

// ProcessNew processes a node using the new channel-based method
func (p *FileTypeFilterPlugin) ProcessNew(*manifest.Node) []chan Status {
	return nil // Not used
}

// filterByFileType filters nodes by file extension
func (p *FileTypeFilterPlugin) filterByFileType(node *manifest.Node, parent *manifest.Node, _ registry.Interface) (bool, error) {
	if node.Type == "file" {
		fileExt := filepath.Ext(node.File)
		if !slices.Contains(p.ContentFileFormats, fileExt) {
			manifest.RemoveNodeFromParent(node, parent)
			return true, nil
		}
	}
	return false, nil
}
