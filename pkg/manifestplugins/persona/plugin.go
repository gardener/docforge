package persona

import (
	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/registry"
)

// Persona is the object representing the persona filtering plugin
type Persona struct{}

// PluginNodeTransformations returns the node transformations for the persona filtering plugin
func (d *Persona) PluginNodeTransformations() []manifest.NodeTransformation {
	return []manifest.NodeTransformation{resolvePersonaFolders}
}

func resolvePersonaFolders(node *manifest.Node, parent *manifest.Node, _ registry.Interface) (bool, error) {
	if node.Type == "dir" && (node.Dir == "development" || node.Dir == "operations" || node.Dir == "usage") {
		for _, child := range node.Structure {
			addPersonaAliasesForNode(child, node.Dir)
		}
		parent.Structure = append(parent.Structure, node.Structure...)
		manifest.RemoveNodeFromParent(node, parent)
	}
	return true, nil
}

func addPersonaAliasesForNode(node *manifest.Node, personaDir string) {
	var dirToPersona = map[string]string{"usage": "Users", "operations": "Operators", "development": "Developers"}
	if node.Type == "file" {
		if node.Frontmatter == nil {
			node.Frontmatter = map[string]interface{}{}
		}
		node.Frontmatter["persona"] = dirToPersona[personaDir]
	}
	for _, child := range node.Structure {
		addPersonaAliasesForNode(child, personaDir)
	}
}
