// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package writers

import (
	"bytes"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/gardener/docforge/pkg/core/manifest"
	"github.com/gardener/docforge/pkg/osfakes/osshim"
	"gopkg.in/yaml.v3"
)

// FSWriter is implementation of Writer interface for writing blobs to the file system
type FSWriter struct {
	Root string
	Ext  string
	Hugo bool
	FS   osshim.Os
}

func (f *FSWriter) Write(name, path string, docBlob []byte, node *manifest.Node, IndexFileNames []string) error {
	if slices.Contains(IndexFileNames, name) {
		name = "_index.md"
	}
	//generate _index.md content
	if f.Hugo && name == "_index.md" && node != nil && node.Frontmatter != nil && docBlob == nil {
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
	p := filepath.Join(f.Root, path)
	if len(docBlob) == 0 {
		return nil
	}
	if err := f.FS.MkdirAll(p, 0755); err != nil {
		return err
	}
	if len(f.Ext) > 0 {
		name = fmt.Sprintf("%s.%s", name, f.Ext)
	}
	filePath := filepath.Join(p, name)
	if err := f.FS.WriteFile(filePath, docBlob, 0644); err != nil {
		return fmt.Errorf("error writing %s: %v", filePath, err)
	}
	return nil
}
