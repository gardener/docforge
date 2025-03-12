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
	return []manifest.NodeTransformation{checkFileTypeFormats}
}

func checkFileTypeFormats(node *manifest.Node, _ *manifest.Node, r registry.Interface, contentFileFormats []string) (bool, error) {
	if node.Type != "file" {
		return false, nil
	}
	files := append(node.FileType.MultiSource, node.FileType.Source, node.FileType.File)
	for _, file := range files {
		// we do || file == "" to skip empty fields
		if !slices.ContainsFunc(contentFileFormats, func(fileFormat string) bool { return strings.HasSuffix(file, fileFormat) || file == "" }) {
			return false, fmt.Errorf("file format of %s isn't supported", file)
		}
	}
	return false, nil
}
