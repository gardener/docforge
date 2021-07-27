// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package hugo

import (
	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/markdown"
	"github.com/gardener/docforge/pkg/writers"
	"gopkg.in/yaml.v3"
	"path/filepath"
)

// FSWriter is implementation of Writer interface for writing blobs
// to the file system at a designated path in a Hugo-specific way
type FSWriter struct {
	Writer         writers.Writer
}

// Write implements writers#Write and will create section file #path/#name/_index.md'
// for node without content, but with frontmatter properties.
func (w *FSWriter) Write(name, path string, docBlob []byte, node *api.Node) error {
	if node != nil {
		if docBlob == nil && node.Properties != nil && node.Properties["frontmatter"] != nil {
			var (
				err      error
				b        []byte
				_docBlob []byte
			)
			if b, err = yaml.Marshal(node.Properties["frontmatter"]); err != nil {
				return err
			}
			_name := "_index.md"
			if _docBlob, err = markdown.InsertFrontMatter(b, []byte("")); err != nil {
				return err
			}
			if err := w.Writer.Write(_name, filepath.Join(path, name), _docBlob, node); err != nil {
				return err
			}
		}
	}

	if err := w.Writer.Write(name, path, docBlob, node); err != nil {
		return err
	}
	return nil
}
