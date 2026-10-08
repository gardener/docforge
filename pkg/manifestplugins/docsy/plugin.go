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

	// Compute the output filename used in .from.
	// Hugo (UsesManifestNameInEditPath=true) keeps the original manifest filename
	// (e.g. README.md) for byte-identity with previous releases, even though the
	// output file is _index.md. VitePress (UsesManifestNameInEditPath=false)
	// renames index files to IndexFileName() so .from reflects the served path.
	outName := node.Name()
	if d.Config != nil && d.Config.Enabled() && !d.Config.UsesManifestNameInEditPath() {
		if d.Config.IsIndexFile(outName) {
			if target := d.Config.IndexFileName(); target != "" {
				outName = target
			}
		}
	}
	// Build the output-relative .from path.
	// Structural-dir stripping also applies only in non-preserving modes.
	from := path.Join(node.Path, outName)
	if d.Config != nil && d.Config.Enabled() && !d.Config.UsesManifestNameInEditPath() {
		for _, dir := range d.Config.StructuralDirs() {
			from = strings.TrimPrefix(from, dir+"/")
		}
	}

	pathBaseGithubSubdir := map[interface{}]interface{}{}
	pathBaseGithubSubdir["from"] = from
	pathBaseGithubSubdir["to"] = path.Base(url.GetResourcePath())
	node.Frontmatter["path_base_for_github_subdir"] = pathBaseGithubSubdir
	params := map[interface{}]interface{}{}
	params["github_branch"] = url.GetRef()
	node.Frontmatter["params"] = params
	return false, nil
}
