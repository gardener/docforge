package alias

import (
	"fmt"
	"strings"

	"github.com/gardener/docforge/pkg/internal/link"
	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/registry"
)

// Alias is the object representing the alias plugin
type Alias struct{}

// PluginNodeTransformations returns the node transformations for the alias plugin
func (d *Alias) PluginNodeTransformations() []manifest.NodeTransformation {
	return []manifest.NodeTransformation{calculateAliases}
}
func calculateAliases(node *manifest.Node, parent *manifest.Node, _ registry.Interface) (bool, error) {
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
