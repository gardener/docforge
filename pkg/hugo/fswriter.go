// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package hugo

import (
	"fmt"
	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/markdown"
	"github.com/gardener/docforge/pkg/writers"
	"gopkg.in/yaml.v3"
	"k8s.io/klog/v2"
	"path/filepath"
)

// FSWriter is implementation of Writer interface for writing blobs
// to the file system at a designated path in a Hugo-specific way
type FSWriter struct {
	Writer writers.Writer
}

// Write implements writers#Write and will create section file #path/#name/_index.md'
// for node without content, but with frontmatter properties.
func (w *FSWriter) Write(name, path string, docBlob []byte, node *api.Node) error {
	if node != nil {
		if docBlob == nil && node.Properties != nil && node.Properties["frontmatter"] != nil {
			if len(node.Nodes) > 0 {
				// this is a container node -> propagate frontmatter properties to child _index.md if any
				for _, n := range node.Nodes {
					if n.Name == "_index.md" {
						klog.V(6).Infof("merge %s/_index.md frontmatter properties with: %v\n", api.Path(n, "/"), node.Properties["frontmatter"])
						if n.Properties == nil {
							n.Properties = map[string]interface{}{"frontmatter": node.Properties["frontmatter"]}
						} else {
							childFrontmatter := n.Properties["frontmatter"]
							if childFrontmatter == nil {
								n.Properties["frontmatter"] = node.Properties["frontmatter"]
							} else {
								mergedFrontmatter, ok := childFrontmatter.(map[string]interface{})
								if !ok {
									return fmt.Errorf("invalid frontmatter properties for node:  %s/%s", api.Path(n, "/"), n.Name)
								}
								parentFrontmatter, ok := node.Properties["frontmatter"].(map[string]interface{})
								if !ok {
									return fmt.Errorf("invalid frontmatter properties for node:  %s/%s", api.Path(node, "/"), node.Name)
								}
								for k, v := range parentFrontmatter {
									mergedFrontmatter[k] = v
								}
							}
						}
						klog.V(6).Infof("merged properties: %v", n.Properties["frontmatter"])
						return nil
					}
				}
			}
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
