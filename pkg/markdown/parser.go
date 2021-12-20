// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package markdown

import (
	"github.com/yuin/goldmark"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

var (
	// parser extension for GitHub Flavored Markdown & Frontmatter support
	extensions = []goldmark.Extender{
		extension.GFM,
		meta.Meta,
	}
	// goldmark.Markdown parser with GFM extensions
	gmParser = goldmark.New(goldmark.WithExtensions(extensions...))
)

// Parse markdown content and returns AST node or error
func Parse(source []byte) (ast.Node, error) {
	reader := text.NewReader(source)
	context := parser.NewContext()
	doc := gmParser.Parser().Parse(reader, parser.WithContext(context))
	fmb, err := meta.TryGet(context)
	if err != nil {
		return nil, err
	}
	if doc.Kind() == ast.KindDocument {
		doc.(*ast.Document).SetMeta(fmb)
	}
	return doc, nil
}
