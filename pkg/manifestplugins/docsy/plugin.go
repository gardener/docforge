package docsy

import (
	"fmt"
	"path"
	"strings"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/registry"
)

type Docsy struct{}

func (d *Docsy) PluginNodeTransformations() []manifest.NodeTransformation {
	return []manifest.NodeTransformation{editThisPage}
}

func editThisPage(node *manifest.Node, _ *manifest.Node, r registry.Interface, _ []string) (bool, error) {
	if node.Type != "file" ||
		(node.File == "_index.md" && node.Source == "") ||
		(len(node.MultiSource) > 0) {
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
	pathBaseGithubSubdir := map[string]interface{}{}
	pathBaseGithubSubdir["from"] = strings.TrimPrefix(node.NodePath(), "hugo/")
	pathBaseGithubSubdir["to"] = path.Base(url.GetResourcePath())
	node.Frontmatter["path_base_for_github_subdir"] = pathBaseGithubSubdir
	params := map[string]interface{}{}
	params["github_branch"] = url.GetRef()
	node.Frontmatter["params"] = params
	return false, nil
}
