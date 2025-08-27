// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package plugins

import (
	"fmt"
	"strings"

	"github.com/gardener/docforge/pkg/core/manifest"
	"github.com/gardener/docforge/pkg/internal/link"
	"github.com/gardener/docforge/pkg/registry"
)

// AliasPlugin handles alias generation for nodes
type AliasPlugin struct{}

// Name returns the plugin name for identification
func (p *AliasPlugin) Name() string {
	return "alias"
}

// ManifestTransformations returns transformations to apply during manifest parsing
func (p *AliasPlugin) ManifestTransformations() []manifest.NodeTransformation {
	return []manifest.NodeTransformation{p.calculateAliases}
}

// FinalNodeStructure is called after manifest resolution with the final document structure
func (p *AliasPlugin) FinalNodeStructure(documentNodes []*manifest.Node) error {
	// No initialization needed for alias plugin
	return nil
}

// Processor returns the processor name for node processing
func (p *AliasPlugin) Processor() string {
	return ""
}

// Process processes a node using the old synchronous method
func (p *AliasPlugin) Process(node *manifest.Node) error {
	return nil
}

// ProcessNew processes a node using the new channel-based method
func (p *AliasPlugin) ProcessNew(node *manifest.Node) []chan Status {
	return nil
}

// calculateAliases is the manifest transformation function
func (p *AliasPlugin) calculateAliases(node *manifest.Node, parent *manifest.Node, _ registry.Interface) (bool, error) {
	var (
		nodeAliases  []interface{}
		childAliases []interface{}
		formatted    bool
	)
	if nodeAliases, formatted = node.Frontmatter["aliases"].([]interface{}); node.Frontmatter != nil && node.Frontmatter["aliases"] != nil && !formatted {
		return false, fmt.Errorf("node X \n\n%s\n has invalid alias format", node)
	}
	for _, nodeAliasI := range nodeAliases {
		for _, child := range node.Structure {
			if child.Frontmatter == nil {
				child.Frontmatter = map[string]interface{}{}
			}
			if child.Frontmatter["aliases"] == nil {
				child.Frontmatter["aliases"] = []interface{}{}
			}
			if childAliases, formatted = child.Frontmatter["aliases"].([]interface{}); !formatted {
				return false, fmt.Errorf("node \n\n%s\n has invalid alias format", child)
			}
			childAliasSuffix := strings.TrimSuffix(child.Name(), ".md")
			if child.Name() == "_index.md" {
				childAliasSuffix = ""
			}
			nodeAlias := fmt.Sprintf("%s", nodeAliasI)
			if !strings.HasPrefix(nodeAlias, "/") {
				return false, fmt.Errorf("there is a node with name %s that has an relative alias %s", node.Name(), nodeAlias)
			}
			aliasPath, err := link.Build(nodeAlias, childAliasSuffix, "/")
			if err != nil {
				return false, err
			}
			childAliases = append(childAliases, aliasPath)
			child.Frontmatter["aliases"] = childAliases
		}
	}
	return false, nil
}
