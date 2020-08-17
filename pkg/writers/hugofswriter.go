package writers

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"fmt"
)

// FSWriter is implementation of Writer interface for writing blobs to the file system
type HugoFSWriter struct {
	Writer *FSWriter
}

func (f *HugoFSWriter) Write(name, path string, docBlob []byte) error {
	fmt.Printf("-- %v\n", path)
	path = filepath.Clean(path)
	if err:= f.Writer.Write(name, path, docBlob); err!=nil {
		return err
	}
	if strings.HasSuffix(name, ".md") {
		if err:= writeHugoSectionIndexFile(path, f.Writer.Root); err!=nil {
			return err
		}
	}
	return nil
}

// Creates _index.md in each folder in path, in case there's
// none there yet, and with basic front-matter properties -
// `title: File Name`, based on the contianing folder name and
// capitalized.
func writeHugoSectionIndexFile(path, root string) error {
	pathSegments:= strings.Split(path, "/")
	for i, name:= range pathSegments {
		_path := filepath.Join(pathSegments[:i+1]...)
		name:= strings.Title(strings.ToLower(name))
		p:= fmt.Sprintf("%s/%s/_index.md", root, _path)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			frontMatter:= fmt.Sprintf("---\nTitle: %s\n---\n", name)
			if err := ioutil.WriteFile(p, []byte(frontMatter), 0644); err != nil {
				return err
			}
		}
	}
	//TODO: assume README.md and index.md as good replacements for _index.md and rename them instead of creating new _index.md
	return nil
}
