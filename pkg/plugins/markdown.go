// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package plugins

import (
	"strings"

	"github.com/gardener/docforge/cmd/hugo"
	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/nodeplugins"
	"github.com/gardener/docforge/pkg/nodeplugins/markdown"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/writers"
)

// MarkdownPlugin handles both manifest transformations and node processing for markdown files
type MarkdownPlugin struct {
	registry           registry.Interface
	hugo               hugo.Hugo
	writer             writers.Writer
	skipLinkValidation bool
	markdownProcessor  nodeplugins.Interface // Created lazily in FinalNodeStructure
}

// NewMarkdownPlugin creates a new markdown plugin
func NewMarkdownPlugin(registry registry.Interface, hugo hugo.Hugo, writer writers.Writer, skipLinkValidation bool) *MarkdownPlugin {
	return &MarkdownPlugin{
		registry:           registry,
		hugo:               hugo,
		writer:             writer,
		skipLinkValidation: skipLinkValidation,
	}
}

// Name returns the plugin name for identification
func (p *MarkdownPlugin) Name() string {
	return "markdown"
}

// ManifestTransformations returns transformations to apply during manifest parsing
func (p *MarkdownPlugin) ManifestTransformations() []manifest.NodeTransformation {
	return []manifest.NodeTransformation{
		p.setMarkdownProcessor,
		p.propagateFrontmatter,
		p.propagateSkipValidation,
	}
}

// FinalNodeStructure is called after manifest resolution with the final document structure
func (p *MarkdownPlugin) FinalNodeStructure(documentNodes []*manifest.Node) error {
	// Now we can create the actual markdown processor with the final document structure
	processor := markdown.NewPlugin(documentNodes, p.registry, p.hugo, p.writer, p.skipLinkValidation)
	p.markdownProcessor = processor
	return nil
}

// Processor returns the processor name for node processing
func (p *MarkdownPlugin) Processor() string {
	return "markdown"
}

// Process processes a node using the old synchronous method
func (p *MarkdownPlugin) Process(node *manifest.Node) error {
	return p.markdownProcessor.Process(node)
}

// ProcessNew processes a node using the new channel-based method
func (p *MarkdownPlugin) ProcessNew(node *manifest.Node) []chan nodeplugins.Status {
	return p.markdownProcessor.ProcessNew(node)
}

// setMarkdownProcessor sets the processor for markdown files
func (p *MarkdownPlugin) setMarkdownProcessor(node *manifest.Node, parent *manifest.Node, _ registry.Interface) (bool, error) {
	if node.Type == "file" && strings.HasSuffix(node.File, ".md") {
		node.Processor = "markdown"
	}
	return false, nil
}

// propagateFrontmatter propagates frontmatter from parent to child nodes
func (p *MarkdownPlugin) propagateFrontmatter(node *manifest.Node, parent *manifest.Node, _ registry.Interface) (bool, error) {
	if parent != nil {
		newFM := map[string]interface{}{}
		for k, v := range parent.Frontmatter {
			if k != "aliases" {
				newFM[k] = v
			}
		}
		for k, v := range node.Frontmatter {
			newFM[k] = v
		}
		node.Frontmatter = newFM
	}
	return false, nil
}

// propagateSkipValidation propagates skip validation settings
func (p *MarkdownPlugin) propagateSkipValidation(node *manifest.Node, parent *manifest.Node, _ registry.Interface) (bool, error) {
	if parent != nil && parent.Frontmatter != nil {
		if skipVal := parent.Frontmatter["skip_validation"]; skipVal != nil {
			if node.Frontmatter == nil {
				node.Frontmatter = map[string]interface{}{}
			}
			if node.Frontmatter["skip_validation"] == nil {
				node.Frontmatter["skip_validation"] = skipVal
			}
		}
	}
	return false, nil
}
