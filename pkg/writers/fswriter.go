package writers

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

// FSWriter is implementation of Writer interface for writing blobs to the file system
type FSWriter struct {
	Root string
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
	}
	filePath := filepath.Join(p, name)
	if err := ioutil.WriteFile(filePath, docBlob, 0644); err != nil {
		fmt.Printf("Error writing %s: %v\n", filepath.Join(f.Root, path, name), err)
		return err
	}

	return nil
}
