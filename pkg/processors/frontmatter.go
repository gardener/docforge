// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package processors

import (
	"strings"

	"github.com/gardener/docforge/pkg/markdown"

	"github.com/gardener/docforge/pkg/api"
	"gopkg.in/yaml.v3"
)

// FrontMatter is a processor implementation responsible to inject front-matter
// properties defined on nodes and default front-matter
type FrontMatter struct {
	// IndexFileNames defines a list of file names that indicate
	// their content can be used as section files.
	IndexFileNames []string
}

// Process implements Processor#Process
func (f *FrontMatter) Process(documentBlob []byte, node *api.Node) ([]byte, error) {
	// Process only document nodes
	if len(node.Source) == 0 && len(node.ContentSelectors) == 0 && node.Template == nil {
		return documentBlob, nil
	}
	var (
		nodeFmBytes, fmBytes, content []byte
		props, fm, docFm              map[string]interface{}
		ok                            bool
		err                           error
	)
	// Frontmatter from node
	if props = node.Properties; props == nil {
		props = map[string]interface{}{}
	}
	if _fm, found := props["frontmatter"]; found {
		if fm, ok = _fm.(map[string]interface{}); !ok {
			panic("node frontmatter type cast failed")
		}
	}

	if fm == nil {
		// Minimal standard front matter, injected by default
		// if no other has been defined on a node
		// TODO: make this configurable option, incl. the default frontmatter we inject
		fm = map[string]interface{}{}
	}

	// document front matter
	if fmBytes, content, err = markdown.StripFrontMatter(documentBlob); err != nil {
		return nil, err
	}
	docFm = map[string]interface{}{}
	if err := yaml.Unmarshal(fmBytes, docFm); err != nil {
		return nil, err
	}

	for propertyKey, propertyValue := range docFm {
		if _, ok := fm[propertyKey]; !ok {
			fm[propertyKey] = propertyValue
		}
	}

	if _, ok := fm["title"]; !ok {
		fm["title"] = f.getNodeTitle(node)
	}

	nodeFmBytes, err = yaml.Marshal(fm)
	if err != nil {
		return nil, err
	}

	// TODO: merge node + doc frontmatter per configurable strategy:
	// - merge where node frontmatter entries win over document frontmatter
	// - merge where document frontmatter entries win over node frontmatter
	// - merge where document frontmatter are merged with node frontmatter ignoring duplicates (currently impl.)
	if documentBlob, err = markdown.InsertFrontMatter(nodeFmBytes, content); err != nil {
		return nil, err
	}
	return documentBlob, nil
}

// Determines node title from its name or its parent name if
// it is eligible to be index file, and then normalizes either
// as a title - removing `-`, `_`, `.md` and converting to title
// case.
func (f *FrontMatter) getNodeTitle(node *api.Node) string {
	title := node.Name
	if node.Parent() != nil && f.nodeIsIndexFile(node.Name) {
		title = node.Parent().Name
	}
	title = strings.TrimRight(title, ".md")
	title = strings.ReplaceAll(title, "_", " ")
	title = strings.ReplaceAll(title, "-", " ")
	title = strings.Title(title)
	return title
}

// Compares a node name to the configured list of index file
// and a default name '_index.md' to determin if this node
// is an index document node.
func (f *FrontMatter) nodeIsIndexFile(name string) bool {
	for _, s := range f.IndexFileNames {
		if strings.ToLower(name) == strings.ToLower(s) {
			return true
		}
	}
	return "_index.md" == name
}
