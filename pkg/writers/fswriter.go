// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package writers

import (
	"fmt"
	"github.com/gardener/docforge/pkg/api"
	"io/ioutil"
	"os"
	"path/filepath"
)

// FSWriter is implementation of Writer interface for writing blobs to the file system
type FSWriter struct {
	Root string
	Ext  string
}

func (f *FSWriter) Write(name, path string, docBlob []byte, node *api.Node) error {
	p := filepath.Join(f.Root, path)
	if len(docBlob) <= 0 {
		if err := os.MkdirAll(p, os.ModePerm); err != nil {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(p, os.ModePerm); err != nil {
		return err
	}

	if len(f.Ext) > 0 {
		name = fmt.Sprintf("%s.%s", name, f.Ext)
	}

	filePath := filepath.Join(p, name)

	if err := ioutil.WriteFile(filePath, docBlob, 0644); err != nil {
		return fmt.Errorf("error writing %s: %v", filepath.Join(f.Root, path, name), err)
	}

	return nil
}
