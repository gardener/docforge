// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package hugo

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gardener/docforge/pkg/markdown"
	"gopkg.in/yaml.v3"
	"k8s.io/klog/v2"

	"github.com/gardener/docforge/pkg/api"
	nodeutil "github.com/gardener/docforge/pkg/util/node"
	"github.com/gardener/docforge/pkg/writers"
)

// FSWriter is implementation of Writer interface for writing blobs
// to the file system at a designated path in a Hugo-specific way
type FSWriter struct {
	Writer         writers.Writer
	IndexFileNames []string
}

// Write implements writers#Write and will rename files that match the
// list in hugo#FSWriter.IndexFileNames to _index.md on first match, first
// renamed basis to serve as section files.
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
			_name := "_index"
			if _docBlob, err = markdown.InsertFrontMatter(b, []byte("")); err != nil {
				return err
			}
			if err := w.Writer.Write(_name, filepath.Join(path, name), _docBlob, node); err != nil {
				return err
			}
		}

		// validate
		if node.Parent() != nil {
			if ns := getIndexNodes(node.Parent().Nodes); len(ns) > 1 {
				names := []string{}
				for _, n := range ns {
					names = append(names, n.Name)
				}
				p := nodeutil.Path(node, "/")
				return fmt.Errorf("multiple peer nodes with property index: true detected in %s: %s", p, strings.Join(names, ","))
			}
		}

		if hasIndexNode([]*api.Node{node}) {
			name = "_index"
		}
		// if IndexFileNames has values and index file has not been
		// identified, try to figure out index file out from node names.
		peerNodes := node.Peers()
		if len(w.IndexFileNames) > 0 && name != "_index" && name != "_index.md" && !hasIndexNode(peerNodes) {
			for _, s := range w.IndexFileNames {
				if strings.EqualFold(name, s) {
					klog.V(6).Infof("Renaming %s -> _index.md\n", filepath.Join(path, name))
					name = "_index"
					break
				}
			}
		}
	}

	if err := w.Writer.Write(name, path, docBlob, node); err != nil {
		return err
	}
	return nil
}

func hasIndexNode(nodes []*api.Node) bool {
	for _, n := range nodes {
		if n.Properties != nil {
			index := n.Properties["index"]
			if isIndex, ok := index.(bool); ok {
				return isIndex
			}
			if n.Name == "_index" || n.Name == "_index.md" {
				return true
			}
		}
	}
	return false
}

func getIndexNodes(nodes []*api.Node) []*api.Node {
	indexNodes := []*api.Node{}
	for _, n := range nodes {
		if n.Properties != nil {
			index := n.Properties["index"]
			if isIndex, ok := index.(bool); ok {
				if isIndex {
					indexNodes = append(indexNodes, n)
				}
			}
		}
	}
	return indexNodes
}
