// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package markdown_test

import (
	"bytes"

	"github.com/gardener/docforge/pkg/nodeplugins/markdown/document/markdown"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/yuin/goldmark/renderer"
)

func renderPolicyResult(src string, opts ...renderer.Option) (string, []markdown.Finding) {
	pr := markdown.NewPolicyRenderer(opts...)
	doc, err := markdown.Parse(markdown.New(), []byte(src))
	Expect(err).NotTo(HaveOccurred())
	buf := &bytes.Buffer{}
	err = pr.Render(buf, []byte(src), doc)
	Expect(err).NotTo(HaveOccurred())
	return buf.String(), pr.Result()
}

var _ = Describe("PolicyRenderer", func() {
	Describe("StripBlock — Type1 script/style blocks are replaced", func() {
		It("strips top-level <script> block", func() {
			out, findings := renderPolicyResult("<script>\nalert(1)\n</script>\n")
			Expect(out).To(Equal("<!-- docforge: stripped block -->\n"))
			Expect(findings).To(HaveLen(1))
			Expect(findings[0].Decision).To(Equal(markdown.StripBlock))
		})

		It("strips top-level <style> block", func() {
			out, findings := renderPolicyResult("<style>\nbody{}\n</style>\n")
			Expect(out).To(Equal("<!-- docforge: stripped block -->\n"))
			Expect(findings).To(HaveLen(1))
			Expect(findings[0].Decision).To(Equal(markdown.StripBlock))
		})

		It("strips <SCRIPT> (uppercase tag name)", func() {
			out, findings := renderPolicyResult("<SCRIPT>\nalert(1)\n</SCRIPT>\n")
			Expect(out).To(Equal("<!-- docforge: stripped block -->\n"))
			Expect(findings).To(HaveLen(1))
			Expect(findings[0].Decision).To(Equal(markdown.StripBlock))
		})

		It("strips <script setup> (tag with attributes)", func() {
			out, findings := renderPolicyResult("<script setup>\nconst x = 1\n</script>\n")
			Expect(out).To(Equal("<!-- docforge: stripped block -->\n"))
			Expect(findings).To(HaveLen(1))
			Expect(findings[0].Decision).To(Equal(markdown.StripBlock))
		})

		It("strips <script> inside a blockquote, preserving the '> ' prefix", func() {
			out, findings := renderPolicyResult("> <script>\n> alert(1)\n> </script>\n")
			Expect(out).To(Equal("> <!-- docforge: stripped block -->\n"))
			Expect(findings).To(HaveLen(1))
			Expect(findings[0].Decision).To(Equal(markdown.StripBlock))
		})

		It("strips <script> inside a list item, preserving indentation", func() {
			out, findings := renderPolicyResult("- item\n\n  <script>\n  alert(1)\n  </script>\n")
			Expect(out).To(Equal("- item\n  \n  <!-- docforge: stripped block -->\n"))
			Expect(findings).To(HaveLen(1))
			Expect(findings[0].Decision).To(Equal(markdown.StripBlock))
		})

		It("strips <script> inside a blockquote that is inside a list item", func() {
			out, findings := renderPolicyResult("- item\n\n  > <script>\n  > alert(1)\n  > </script>\n")
			Expect(out).To(Equal("- item\n  \n  > <!-- docforge: stripped block -->\n"))
			Expect(findings).To(HaveLen(1))
			Expect(findings[0].Decision).To(Equal(markdown.StripBlock))
		})
	})

	Describe("no policy effect — tags that do not match are left unchanged", func() {
		It("<scriptx> is emitted unchanged with no finding", func() {
			out, findings := renderPolicyResult("<scriptx>\nfoo\n</scriptx>\n")
			Expect(out).To(Equal("<scriptx>\nfoo\n</scriptx>\n"))
			Expect(findings).To(BeEmpty())
		})

		It("fenced code block containing <script> is emitted unchanged with no finding", func() {
			out, findings := renderPolicyResult("```html\n<script>alert(1)</script>\n```\n")
			Expect(out).To(Equal("```html\n<script>alert(1)</script>\n```\n"))
			Expect(findings).To(BeEmpty())
		})

		It("indented code block containing <script> is emitted unchanged with no finding", func() {
			out, findings := renderPolicyResult("    <script>alert(1)</script>\n")
			Expect(out).To(Equal("```\n<script>alert(1)</script>\n```\n"))
			Expect(findings).To(BeEmpty())
		})
	})

	Describe("WarnBlock — non-Type1 blocks or Type1 non-script/style are warned but emitted", func() {
		It("<pre> containing <script> emits unchanged and records a WarnBlock finding", func() {
			out, findings := renderPolicyResult("<pre>\n<script>alert(1)</script>\n</pre>\n")
			Expect(out).To(Equal("<pre>\n<script>alert(1)</script>\n</pre>\n"))
			Expect(findings).To(HaveLen(1))
			Expect(findings[0].Decision).To(Equal(markdown.WarnBlock))
		})

		It("<div> containing <script> (Type6) emits unchanged and records a WarnBlock finding", func() {
			out, findings := renderPolicyResult("<div>\n<script>alert(1)</script>\n</div>\n\n")
			Expect(out).To(Equal("<div>\n<script>alert(1)</script>\n</div>\n"))
			Expect(findings).To(HaveLen(1))
			Expect(findings[0].Decision).To(Equal(markdown.WarnBlock))
		})
	})

	Describe("SuppressDocument — inline script/style records a finding but emits unchanged", func() {
		It("inline <script> records SuppressDocument findings and emits text unchanged", func() {
			out, findings := renderPolicyResult("text <script>alert(1)</script> more\n")
			Expect(out).To(Equal("text <script>alert(1)</script> more\n"))
			Expect(findings).NotTo(BeEmpty())
			for _, f := range findings {
				Expect(f.Decision).To(Equal(markdown.SuppressDocument))
			}
		})
	})

	Describe("trusted — policy is not applied to trusted documents", func() {
		It("trusted <script> block is emitted unchanged with no finding", func() {
			out, findings := renderPolicyResult("<script>\nalert(1)\n</script>\n", markdown.WithTrusted(true))
			Expect(out).To(Equal("<script>\nalert(1)\n</script>\n"))
			Expect(findings).To(BeEmpty())
		})

		It("trusted <style> block is emitted unchanged with no finding", func() {
			out, findings := renderPolicyResult("<style>\nbody{}\n</style>\n", markdown.WithTrusted(true))
			Expect(out).To(Equal("<style>\nbody{}\n</style>\n"))
			Expect(findings).To(BeEmpty())
		})
	})
})
