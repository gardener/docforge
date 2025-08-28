package markdown

import (
	"bytes"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/gardener/docforge/pkg/core/manifest"
	"github.com/gardener/docforge/pkg/osshim/filesystem"
	"gopkg.in/yaml.v3"
)

// writeDocument writes a document to the filesystem with Hugo-specific logic
func writeDocument(fs filesystem.Interface, rootPath string, hugo bool, name, path string, docBlob []byte, node *manifest.Node, indexFileNames []string) error {
	if slices.Contains(indexFileNames, name) {
		name = "_index.md"
	}

	// Generate _index.md content for Hugo
	if hugo && name == "_index.md" && node != nil && node.Frontmatter != nil && docBlob == nil {
		buf := bytes.Buffer{}
		_, _ = buf.Write([]byte("---\n"))
		fm, err := yaml.Marshal(node.Frontmatter)
		if err != nil {
			return err
		}
		_, _ = buf.Write(fm)
		_, _ = buf.Write([]byte("---\n"))
		docBlob = buf.Bytes()
	}

	p := filepath.Join(rootPath, path)
	if len(docBlob) == 0 {
		return nil
	}

	if err := fs.MkdirAll(p, 0755); err != nil {
		return err
	}

	filePath := filepath.Join(p, name)
	if err := fs.WriteFile(filePath, docBlob, 0644); err != nil {
		return fmt.Errorf("error writing %s: %v", filePath, err)
	}
	return nil
}
