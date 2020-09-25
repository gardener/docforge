package hugo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gardener/docforge/pkg/writers"
)

// FSWriter is implementation of Writer interface for writing blobs
// to the file system at a designated path in a Hugo-specific way
type FSWriter struct {
	Writer         writers.Writer
	IndexFileNames []string
}

// Write implements writers#Write and will rename files that match the
// list in hugo#FSWriter.IndexFileNames to _index.md on first match, first
// renamed basis to serve as section files.
func (w *FSWriter) Write(name, path string, docBlob []byte) error {

	if strings.HasSuffix(name, ".md") {
		// If there's still no section file at this path, assess if the file
		// is a good candiate to become Hugo section file
		if _, err := os.Stat(filepath.Join(path, "_index.md")); os.IsNotExist(err) {
			for _, s := range w.IndexFileNames {
				if strings.ToLower(strings.TrimSuffix(name, ".md")) == s {
					fmt.Printf("Renaming %s -> _index.md\n", filepath.Join(path, name))
					name = "_index.md"
				}
			}
		} else {
			// TODO: see if we can generate _index.md section files when it's all done
		}

	}

	if err := w.Writer.Write(name, path, docBlob); err != nil {
		return err
	}

	return nil
}
