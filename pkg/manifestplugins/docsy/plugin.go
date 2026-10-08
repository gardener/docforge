package docsy

import (
	"fmt"
	"path"
	"strings"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/sitegen"
)

// Docsy is the object representing the docsy plugin
type Docsy struct {
	Config sitegen.Config
}

// PluginNodeTransformations returns the node transformations for the docsy plugin
func (d *Docsy) PluginNodeTransformations() []manifest.NodeTransformation {
	return []manifest.NodeTransformation{d.editThisPage}
}

func (d *Docsy) editThisPage(node *manifest.Node, _ *manifest.Node, r registry.Interface) (bool, error) {
	isNotFile := node.Type != "file"
	isIndexFileWithoutSource := (node.File == "_index.md" || node.File == "index.md") && node.Source == ""
	hasMultipleSource := len(node.MultiSource) > 0
	isNotMarkdown := node.Processor != "markdown"

	shouldSkipNode := isNotFile || isIndexFileWithoutSource || hasMultipleSource || isNotMarkdown

	if shouldSkipNode {
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
	pathBaseGithubSubdir["from"] = editPathFrom(node, d.Config)
	pathBaseGithubSubdir["to"] = path.Base(url.GetResourcePath())
	node.Frontmatter["path_base_for_github_subdir"] = pathBaseGithubSubdir
	params := map[interface{}]interface{}{}
	params["github_branch"] = url.GetRef()
	node.Frontmatter["params"] = params
	return false, nil
}

// editPathFrom returns the output-relative path used in path_base_for_github_subdir.from.
// Hugo (UsesManifestNameInEditPath=true) keeps the original manifest filename for
// byte-identity with previous releases. VitePress (UsesManifestNameInEditPath=false)
// uses IndexFileName() so .from reflects the served path, and strips structural dirs.
func editPathFrom(node *manifest.Node, cfg sitegen.Config) string {
	useSitePath := cfg != nil && cfg.Enabled() && !cfg.UsesManifestNameInEditPath()
	outName := node.Name()
	if useSitePath && cfg.IsIndexFile(outName) {
		if target := cfg.IndexFileName(); target != "" {
			outName = target
		}
	}
	from := path.Join(node.Path, outName)
	if useSitePath {
		for _, dir := range cfg.StructuralDirs() {
			from = strings.TrimPrefix(from, dir+"/")
		}
	}
	return from
}
