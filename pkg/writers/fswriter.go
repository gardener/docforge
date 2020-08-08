package writers

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

// FSWriter is implementation of Writer interface for writing blobs to the file system
type FSWriter struct {
	Root string
}

func (f *FSWriter) Write(name, path string, docBlob []byte) error {
	p := filepath.Join(f.Root, path)
	if len(docBlob) < 0 {
		log.Println("skipping document with name", name)
	}

	if _, err := os.Stat(p); os.IsNotExist(err) {
		log.Println("mdkir: ", p)
		if err = os.MkdirAll(p, os.ModePerm); err != nil {
			return err
		}
	}
	filePath := filepath.Join(p, name)
	log.Println("writeFile: ", filePath)
	if err := ioutil.WriteFile(filePath, docBlob, 0644); err != nil {
		return err
	}

	return nil
}
