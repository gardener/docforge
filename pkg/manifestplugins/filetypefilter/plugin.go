package filetypefilter

import (
	"fmt"
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
			// node needs to be removed as it contains a file with unsupported format
			parent.Structure = slices.DeleteFunc(parent.Structure, func(ptr *manifest.Node) bool {
				return ptr == node
			})
		}
	}

	for _, file := range node.FileType.MultiSource {
		checkAndUpdateNodeParent(file)
	}
	checkAndUpdateNodeParent(node.FileType.Source)
	checkAndUpdateNodeParent(node.FileType.File)

	if len(parent.Structure) == 0 {
		return false, fmt.Errorf("node\n %v\n has no files with supported formats: %v", parent.String(), d.ContentFileFormats)
	}

	return false, nil
}
