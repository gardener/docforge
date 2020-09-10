package writers

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// FSWriter is implementation of Writer interface for writing blobs to the file system
type HugoFSWriter struct {
	Writer *FSWriter
}

func (w *HugoFSWriter) Write(name, path string, docBlob []byte) error {
	fmt.Printf("Writing %s\n", filepath.Join(w.Writer.Root, path, name))
	path = filepath.Clean(path)
	if err := w.Writer.Write(name, path, docBlob); err != nil {
		fmt.Printf("Error writing %s: %v\n", filepath.Join(w.Writer.Root, path, name), err)
		return err
	}
	if strings.HasSuffix(name, ".md") {
		// Assume README.md and index.md as good replacements for _index.md and use them if they exist
		// FIXME: we need also links to original file name rewritten or they will break after rename
		f, err := getIndexFile(path)
		if err != nil {
			return err
		}

		if f == nil {
			// Generate _index.md
			fmt.Printf("Generating _index.md for %s", path)
			if err = writeHugoSectionIndexFile(path, w.Writer.Root); err != nil {
				return err
			}
			return nil
		}

		fPath:= filepath.Join(w.Writer.Root, path, f.Name())
		indexPath:= filepath.Join(w.Writer.Root, path, "_index.md")
		fmt.Printf("Renaming %s to %s", fPath, indexPath)
		return os.Rename(fPath, indexPath)
	}
	return nil
}

// Creates _index.md in each folder in path, in case there's
// none there yet, and with basic front-matter properties -
// `title: File Name`, based on the contianing folder name and
// capitalized.
func writeHugoSectionIndexFile(path, root string) error {
	pathSegments := strings.Split(path, "/")
	for i, name := range pathSegments {
		_path := filepath.Join(pathSegments[:i+1]...)
		name := strings.Title(strings.ToLower(name))
		p := fmt.Sprintf("%s/%s/_index.md", root, _path)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			frontMatter := fmt.Sprintf("---\nTitle: %s\n---\n", name)
			if err := ioutil.WriteFile(p, []byte(frontMatter), 0644); err != nil {
				return err
			}
		}
	}
	return nil
}

func getIndexFile(folderPath string) (os.FileInfo, error) {
	files, err := ioutil.ReadDir(folderPath)
    if err != nil {
        return nil, err
    }
    for _, f := range files {
		name:= strings.ToLower(f.Name())
		if strings.HasPrefix(name, "_index.") {
			return f, nil
		}
		if strings.HasPrefix(name, "index.") {
			return f, nil
		}
		if strings.HasPrefix(name, "read.") { 
			return f, nil
		}
		if strings.HasPrefix(name, "readme."){
			return f, nil
		}
		if strings.HasPrefix(name, "overview."){
			return f, nil
		}
	}
	return nil, nil
}
