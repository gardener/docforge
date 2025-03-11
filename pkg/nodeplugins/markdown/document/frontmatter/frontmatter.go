// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package frontmatter

import (
	"fmt"
	"strings"

	"github.com/gardener/docforge/pkg/manifest"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate -header ../../../../../license_prefix.txt

//counterfeiter:generate . NodeMeta

// NodeMeta represents node meta operations
type NodeMeta interface {
	Meta() map[string]interface{}
	SetMeta(map[string]interface{})
}

// MoveMultiSourceFrontmatterToTopDocument moves MultiSource frontmatter to top document
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

// MergeDocumentAndNodeFrontmatter merges frontmatter from document and node object
func MergeDocumentAndNodeFrontmatter(nodeAst NodeMeta, node *manifest.Node) {
	if nodeAst == nil || node == nil {
		return
	}
	docFrontmatter := nodeAst.Meta()
	for frontMatterProperty, frontMatterValue := range node.Frontmatter {
		if frontMatterProperty == "aliases" && docFrontmatter["aliases"] != nil {
			docFrontmatterAliases, _ := docFrontmatter["aliases"].([]interface{})
			nodeFrontmatterAliases, _ := frontMatterValue.([]interface{})
			for _, docFrontmatterAlias := range docFrontmatterAliases {
				nodeFrontmatterAliases = append(nodeFrontmatterAliases, fmt.Sprintf("%s", docFrontmatterAlias))
			}
			docFrontmatter["aliases"] = nodeFrontmatterAliases
		} else {
			docFrontmatter[frontMatterProperty] = frontMatterValue
		}
	}
	// doc frontmatter has been computed. Copy it to node
	if node.Frontmatter == nil {
		node.Frontmatter = map[string]interface{}{}
	}
	for frontMatterProperty, frontMatterValue := range docFrontmatter {
		node.Frontmatter[frontMatterProperty] = frontMatterValue
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
	title = cases.Title(language.English).String(title)
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
