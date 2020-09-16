package writers

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// FSWriter is implementation of Writer interface for writing blobs to the file system
type FSWriter struct {
	Root string
	//FIXME: this is very temporary
	Hugo bool
}

func (f *FSWriter) Write(name, path string, docBlob []byte) error {
	fmt.Printf("Writing %s \n", filepath.Join(f.Root, path, name))
	if len(docBlob) <= 0 {
		return nil
	}
	p := filepath.Join(f.Root, path)
	if _, err := os.Stat(p); os.IsNotExist(err) {
		if err = os.MkdirAll(p, os.ModePerm); err != nil {
			return err
		}
		if f.Hugo && strings.HasSuffix(name, ".md") {
			if err:= writeHugoSectionIndex(path, f.Root); err!=nil {
				return err
			}
		}
	}
	filePath := filepath.Join(p, name)
	if err := ioutil.WriteFile(filePath, docBlob, 0644); err != nil {
		return err
	}

	return nil
}

// Creates _index.md in each folder in path with
// basic front-matter properties - `title: File Name`
// without extension and capitalized
func writeHugoSectionIndex(path, root string) error {
	pathSegments:= strings.Split(path, "/")
	for i, name:= range pathSegments {
		_path := strings.Join(pathSegments[:i+1], "/")
		name:= strings.Title(strings.ToLower(name))
		p:= fmt.Sprintf("%s/%s/_index.md", root, _path)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			frontMatter:= fmt.Sprintf("---\nTitle: %s\n---\n", name)
			if err := ioutil.WriteFile(p, []byte(frontMatter), 0644); err != nil {
				return err
			}
		}
	}
	return nil
}
