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
	if node == nil || node.Type != "file" {
		return false, nil
	}

	changed := false
	checkAndUpdateNode := func(file string) {
		if !slices.ContainsFunc(d.ContentFileFormats, func(fileFormat string) bool {
			return strings.HasSuffix(file, fileFormat) || file == ""
		}) && parent != nil {
			if idx := slices.Index(parent.Structure, node); idx != -1 {
				parent.Structure[idx] = nil
				changed = true
			}
		}
	}

	for _, file := range node.FileType.MultiSource {
		checkAndUpdateNode(file)
	}
	checkAndUpdateNode(node.FileType.Source)
	checkAndUpdateNode(node.FileType.File)

	if parent != nil {
		parent.Structure = slices.DeleteFunc(parent.Structure, func(ptr *manifest.Node) bool {
			return ptr == nil
		})
		if changed && len(parent.Structure) == 0 {
			return false, fmt.Errorf("node\n %v\n has no files with supported formats: %v", parent.String(), d.ContentFileFormats)
		}
	}
	return false, nil
}
