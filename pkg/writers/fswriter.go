// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package writers

import (
	"bytes"
	"fmt"
	"github.com/gardener/docforge/pkg/api"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"os"
	"path/filepath"
)

// FSWriter is implementation of Writer interface for writing blobs to the file system
type FSWriter struct {
	Root string
	Ext  string
	Hugo bool
}

func (f *FSWriter) Write(name, path string, docBlob []byte, node *api.Node) error {
	if f.Hugo && node != nil {
		if docBlob == nil && node.Properties != nil && node.Properties["frontmatter"] != nil {
			if len(node.Nodes) > 0 {
				for _, n := range node.Nodes {
					if n.Name == "_index.md" { // TODO: Unify section file check & ensure one section file per folder
						// has index child
						return nil
					}
				}
			}
			// transform params
			buf := bytes.Buffer{}
			_, _ = buf.Write([]byte("---\n"))
			fm, err := yaml.Marshal(node.Properties["frontmatter"])
			if err != nil {
				return err
			}
			_, _ = buf.Write(fm)
			_, _ = buf.Write([]byte("---\n"))
			docBlob = buf.Bytes()
			path = filepath.Join(path, name)
			name = "_index.md"
		}
	}

	p := filepath.Join(f.Root, path)

	if len(docBlob) == 0 {
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
		return fmt.Errorf("error writing %s: %v", filePath, err)
	}

	return nil
}
