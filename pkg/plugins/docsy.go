// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package plugins

import (
	"fmt"
	"path"
	"strings"

	"github.com/gardener/docforge/pkg/core"
	"github.com/gardener/docforge/pkg/core/manifest"
	"github.com/gardener/docforge/pkg/core/registry"
)

// DocsyPlugin handles docsy-specific metadata generation
type DocsyPlugin struct{}

// Name returns the plugin name for identification
func (p *DocsyPlugin) Name() string {
	return "docsy"
}

// ManifestTransformations returns transformations to apply during manifest parsing
func (p *DocsyPlugin) ManifestTransformations() []manifest.NodeTransformation {
	return []manifest.NodeTransformation{p.editThisPage}
}

// FinalNodeStructure is called after manifest resolution with the final document structure
func (p *DocsyPlugin) FinalNodeStructure(documentNodes []*manifest.Node) error {
	return nil // No initialization needed
}

// Processor returns the processor name for node processing
func (p *DocsyPlugin) Processor() string {
	return "" // No node processing
}

// Process processes a node using the old synchronous method
func (p *DocsyPlugin) Process(*manifest.Node) error {
	return nil // Not used
}

// ProcessNew processes a node using the new channel-based method
func (p *DocsyPlugin) ProcessNew(*manifest.Node) []chan core.Status {
	return nil // Not used
}

// editThisPage is the manifest transformation function
func (p *DocsyPlugin) editThisPage(node *manifest.Node, _ *manifest.Node, r registry.Interface) (bool, error) {
	if node.Type != "file" ||
		(node.File == "_index.md" && node.Source == "") ||
		(len(node.MultiSource) > 0) ||
		node.Processor != "markdown" {
		return false, nil
	}
	url, err := r.ResourceURL(node.Source)
	if err != nil {
		return false, fmt.Errorf("node %s: %w", node, err)
	}
	if node.Frontmatter == nil {
		node.Frontmatter = map[string]interface{}{}
	}
	node.Frontmatter["github_repo"] = url.RepositoryURLString()
	node.Frontmatter["github_subdir"] = path.Dir(url.GetResourcePath())
	pathBaseGithubSubdir := map[interface{}]interface{}{}
	pathBaseGithubSubdir["from"] = strings.TrimPrefix(node.NodePath(), "hugo/")
	pathBaseGithubSubdir["to"] = path.Base(url.GetResourcePath())
	node.Frontmatter["path_base_for_github_subdir"] = pathBaseGithubSubdir
	return false, nil
}
