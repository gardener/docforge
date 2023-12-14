// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0
package frontmatter

import (
	"fmt"
	"strings"

	"github.com/gardener/docforge/pkg/manifest"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate -header ../../../../license_prefix.txt

//counterfeiter:generate . NodeMeta
type NodeMeta interface {
	Meta() map[string]interface{}
	SetMeta(map[string]interface{})
}

func MoveMultiSourceFrontmatterToTopDocument(dc []NodeMeta) {
	if len(dc) < 2 {
		return
	}
	aggregated := make(map[string]interface{})
	for i := len(dc) - 1; i >= 0; i-- {
		for k, v := range dc[i].Meta() {
			aggregated[k] = v
		}
		dc[i].SetMeta(nil)
	}
	dc[0].SetMeta(aggregated)
}

func MergeDocumentAndNodeFrontmatter(nodeAst NodeMeta, node *manifest.Node) {
	if nodeAst == nil || node == nil {
		return
	}
	docFrontmatter := nodeAst.Meta()
	for k, v := range node.Frontmatter {
		if k == "aliases" && docFrontmatter["aliases"] != nil {
			asArray1, _ := docFrontmatter["aliases"].([]interface{})
			asArray2, _ := v.([]interface{})
			for _, yataa := range asArray1 {
				asArray2 = append(asArray2, fmt.Sprintf("%s", yataa))

			}
			docFrontmatter["aliases"] = asArray2
		} else {
			docFrontmatter[k] = v
		}
	}
	nodeAst.SetMeta(docFrontmatter)
}

// ComputeNodeTitle Determines node title from its name or its parent name if
// it is eligible to be index file, and then normalizes either
// as a title - removing `-`, `_`, `.md` and converting to title
// case.
func ComputeNodeTitle(nodeAst NodeMeta, node *manifest.Node, IndexFileNames []string, hugoEnabled bool) {
	if !hugoEnabled || nodeAst == nil {
		return
	}
	docFrontmatter := nodeAst.Meta()
	if docFrontmatter == nil {
		docFrontmatter = map[string]interface{}{}
	}
	title := node.Name()
	// index node with parent
	if nodeIsIndexFile(node.Name(), IndexFileNames) && node.Parent() != nil && node.Parent().Path != "" {
		title = node.Parent().Name()
	} else if nodeIsIndexFile(node.Name(), IndexFileNames) && node.Parent() != nil && node.Parent().Path == "" {
		// root index node
		title = "Root"
	}
	title = strings.TrimSuffix(title, ".md")
	title = strings.ReplaceAll(title, "_", " ")
	title = strings.ReplaceAll(title, "-", " ")
	title = strings.Title(title)
	if _, ok := docFrontmatter["title"]; !ok {
		docFrontmatter["title"] = title
	}
	nodeAst.SetMeta(docFrontmatter)
}

// Compares a node name to the configured list of index file
// and a default name '_index.md' to determine if this node
// is an index document node.
func nodeIsIndexFile(name string, IndexFileNames []string) bool {
	for _, s := range IndexFileNames {
		if strings.EqualFold(name, s) {
			return true
		}
	}
	return name == "_index.md"
}
