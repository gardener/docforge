// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package plugins

import (
	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/nodeplugins"
	personanodeplugin "github.com/gardener/docforge/pkg/nodeplugins/persona"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/writers"
)

// PersonaPlugin handles both manifest transformations and node processing for persona filtering
type PersonaPlugin struct {
	writer           writers.Writer
	personaProcessor nodeplugins.Interface // Created lazily in FinalNodeStructure
}

// NewPersonaPlugin creates a new persona plugin
func NewPersonaPlugin(writer writers.Writer) *PersonaPlugin {
	return &PersonaPlugin{
		writer: writer,
	}
}

// Name returns the plugin name for identification
func (p *PersonaPlugin) Name() string {
	return "persona"
}

// ManifestTransformations returns transformations to apply during manifest parsing
func (p *PersonaPlugin) ManifestTransformations() []manifest.NodeTransformation {
	return []manifest.NodeTransformation{p.resolvePersonaFolders}
}

// FinalNodeStructure is called after manifest resolution with the final document structure
func (p *PersonaPlugin) FinalNodeStructure(documentNodes []*manifest.Node) error {
	// Now we can create the processor with the final document structure
	if len(documentNodes) > 0 {
		p.personaProcessor = &personanodeplugin.Plugin{Root: documentNodes[0], Writer: p.writer}
	}
	return nil
}

// Processor returns the processor name for node processing
func (p *PersonaPlugin) Processor() string {
	return "persona"
}

// Process processes a node using the old synchronous method
func (p *PersonaPlugin) Process(node *manifest.Node) error {
	return p.personaProcessor.Process(node)
}

// ProcessNew processes a node using the new channel-based method
func (p *PersonaPlugin) ProcessNew(node *manifest.Node) []chan nodeplugins.Status {
	return p.personaProcessor.ProcessNew(node)
}

// resolvePersonaFolders resolves persona directory structures
func (p *PersonaPlugin) resolvePersonaFolders(node *manifest.Node, parent *manifest.Node, _ registry.Interface) (bool, error) {
	if node.Type == "dir" && (node.Dir == "development" || node.Dir == "operations" || node.Dir == "usage") {
		for _, child := range node.Structure {
			p.addPersonaAliasesForNode(child, node.Dir)
		}
		parent.Structure = append(parent.Structure, node.Structure...)
		manifest.RemoveNodeFromParent(node, parent)
	}
	return true, nil
}

// addPersonaAliasesForNode adds persona metadata to nodes
func (p *PersonaPlugin) addPersonaAliasesForNode(node *manifest.Node, personaDir string) {
	var dirToPersona = map[string]string{"usage": "Users", "operations": "Operators", "development": "Developers"}
	if node.Type == "file" {
		if node.Frontmatter == nil {
			node.Frontmatter = map[string]interface{}{}
		}
		node.Frontmatter["persona"] = dirToPersona[personaDir]
	}
	for _, child := range node.Structure {
		p.addPersonaAliasesForNode(child, personaDir)
	}
}
