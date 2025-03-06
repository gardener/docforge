// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package markdown_test

import (
	"bytes"

	"github.com/gardener/docforge/pkg/nodeplugins/markdown/document/markdown"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/yuin/goldmark/ast"
)

var _ = Describe("Parser", func() {
	var (
		md  string
		doc ast.Node
		err error
	)
	BeforeEach(func() {
		md = "---\ntitle: test\n---\n\n## Heading level 2\n\nI really like using Markdown.\n"
	})
	JustBeforeEach(func() {
		doc, err = markdown.Parse(markdown.New(), []byte(md))
	})
	When("Parse markdown", func() {
		It("parse the markdown successfully", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(doc).NotTo(BeNil())
			d, ok := doc.(*ast.Document)
			Expect(ok).To(BeTrue())
			Expect(d.Meta()).Should(HaveKeyWithValue("title", "test"))
		})
		Context("frontmatter invalid", func() {
			BeforeEach(func() {
				md = "---\na = b\n---\n\n## Heading level 2\n\nI really like using Markdown.\n"
			})
			It("should fails", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("a = b"))
			})
		})
		Context("add frontmatter", func() {
			var (
				buf *bytes.Buffer
				m   map[string]interface{}
			)
			BeforeEach(func() {
				md = "## Heading level 2\n\nI really like using Markdown.\n"
				m = make(map[string]interface{})
				m["title"] = "test"
				buf = &bytes.Buffer{}
			})
			JustBeforeEach(func() {
				Expect(doc).NotTo(BeNil())
				d, ok := doc.(*ast.Document)
				Expect(ok).To(BeTrue())
				d.SetMeta(m)
				rnd := markdown.NewLinkModifierRenderer()
				Expect(rnd.Render(buf, []byte(md), d)).To(Succeed())
			})
			It("adds provided frontmatter", func() {
				Expect(buf.String()).To(Equal("---\ntitle: test\n---\n\n## Heading level 2\n\nI really like using Markdown.\n"))
			})
		})
	})
})
