package filetypefilter

import (
	"slices"
	"strings"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/registry"
)

// FileTypeFilter is the object representing the file type filtering plugin
type FileTypeFilter struct {
	ContentFileFormats []string
}

// PluginNodeTransformations returns the node transformations for the file type filtering plugin
func (d *FileTypeFilter) PluginNodeTransformations() []manifest.NodeTransformation {
	return []manifest.NodeTransformation{d.checkFileTypeFormats}
}

func (d *FileTypeFilter) checkFileTypeFormats(node *manifest.Node, parent *manifest.Node, r registry.Interface) (bool, error) {
	if node.Type != "file" || parent == nil {
		return false, nil
	}

	checkAndUpdateNodeParent := func(file string) {
		if file != "" && !slices.ContainsFunc(d.ContentFileFormats, func(fileFormat string) bool {
			return strings.HasSuffix(file, fileFormat)
		}) {
			// node needs to be removed by setting it to nil as it contains a file with unsupported format
			i := slices.IndexFunc(parent.Structure, func(ptr *manifest.Node) bool {
				return ptr == node
			})
			if i >= 0 {
				parent.Structure[i] = nil
			}
		}
	}

	for _, file := range node.FileType.MultiSource {
		checkAndUpdateNodeParent(file)
	}
	checkAndUpdateNodeParent(node.FileType.Source)
	checkAndUpdateNodeParent(node.FileType.File)

	return false, nil
}
