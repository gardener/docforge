package markdown

import (
	"strings"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/registry"
)

// Markdown is the object representing the markdown plugin
type Markdown struct{}

// PluginNodeTransformations returns the node transformations for the markdown plugin
func (d *Markdown) PluginNodeTransformations() []manifest.NodeTransformation {
	return []manifest.NodeTransformation{setMarkdownProcessor, propagateFrontmatter, propagateSkipValidation}
}

func setMarkdownProcessor(node *manifest.Node, parent *manifest.Node, _ registry.Interface) (bool, error) {
	if node.Type == "file" && strings.HasSuffix(node.File, ".md") {
		node.Processor = "markdown"
	}
	return false, nil
}

func propagateFrontmatter(node *manifest.Node, parent *manifest.Node, _ registry.Interface) (bool, error) {
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

func propagateSkipValidation(node *manifest.Node, parent *manifest.Node, _ registry.Interface) (bool, error) {
	if parent != nil && parent.SkipValidation {
		node.SkipValidation = parent.SkipValidation
	}
	return false, nil
}
