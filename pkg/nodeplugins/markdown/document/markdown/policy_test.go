// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package markdown_test

import (
	"github.com/gardener/docforge/pkg/nodeplugins/markdown/document/markdown"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

// parseFirstHTMLBlock parses src as markdown and returns the first HTMLBlock node and its source.
func parseFirstHTMLBlock(src string) (*ast.HTMLBlock, []byte) {
	gm := goldmark.New(goldmark.WithExtensions(extension.GFM))
	srcBytes := []byte(src)
	doc := gm.Parser().Parse(text.NewReader(srcBytes))
	var found *ast.HTMLBlock
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if hb, ok := n.(*ast.HTMLBlock); ok && found == nil {
				found = hb
			}
		}
		return ast.WalkContinue, nil
	})
	return found, srcBytes
}

// parseFirstRawHTML parses src as markdown and returns the first RawHTML inline node and its source.
func parseFirstRawHTML(src string) (*ast.RawHTML, []byte) {
	gm := goldmark.New(goldmark.WithExtensions(extension.GFM))
	srcBytes := []byte(src)
	doc := gm.Parser().Parse(text.NewReader(srcBytes))
	var found *ast.RawHTML
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if rh, ok := n.(*ast.RawHTML); ok && found == nil {
				found = rh
			}
		}
		return ast.WalkContinue, nil
	})
	return found, srcBytes
}

var _ = Describe("DefaultPolicy", func() {
	var policy markdown.DefaultPolicy

	Describe("BlockDecision", func() {
		It("Type1 <script> → StripBlock", func() {
			n, src := parseFirstHTMLBlock("<script>\nalert(1)\n</script>\n")
			Expect(n).NotTo(BeNil())
			Expect(policy.BlockDecision(n, src)).To(Equal(markdown.StripBlock))
		})

		It("Type1 <style> → StripBlock", func() {
			n, src := parseFirstHTMLBlock("<style>\nbody{}\n</style>\n")
			Expect(n).NotTo(BeNil())
			Expect(policy.BlockDecision(n, src)).To(Equal(markdown.StripBlock))
		})

		It("Type1 <SCRIPT> (uppercase) → StripBlock", func() {
			n, src := parseFirstHTMLBlock("<SCRIPT>\nalert(1)\n</SCRIPT>\n")
			Expect(n).NotTo(BeNil())
			Expect(policy.BlockDecision(n, src)).To(Equal(markdown.StripBlock))
		})

		It("Type1 <script setup> → StripBlock", func() {
			n, src := parseFirstHTMLBlock("<script setup>\nconst x = 1\n</script>\n")
			Expect(n).NotTo(BeNil())
			Expect(policy.BlockDecision(n, src)).To(Equal(markdown.StripBlock))
		})

		It("<scriptx> → Keep (no match)", func() {
			n, src := parseFirstHTMLBlock("<scriptx>\nfoo\n</scriptx>\n")
			Expect(n).NotTo(BeNil())
			Expect(policy.BlockDecision(n, src)).To(Equal(markdown.Keep))
		})

		It("Type1 <pre> containing <script> → WarnBlock", func() {
			n, src := parseFirstHTMLBlock("<pre>\n<script>alert(1)</script>\n</pre>\n")
			Expect(n).NotTo(BeNil())
			Expect(policy.BlockDecision(n, src)).To(Equal(markdown.WarnBlock))
		})

		It("Type1 <pre> without script/style → Keep", func() {
			n, src := parseFirstHTMLBlock("<pre>\nhello world\n</pre>\n")
			Expect(n).NotTo(BeNil())
			Expect(policy.BlockDecision(n, src)).To(Equal(markdown.Keep))
		})

		It("Type6 <div> containing <script> → WarnBlock", func() {
			// <div> is Type6 in goldmark
			n, src := parseFirstHTMLBlock("<div>\n<script>alert(1)</script>\n</div>\n\n")
			Expect(n).NotTo(BeNil())
			Expect(policy.BlockDecision(n, src)).To(Equal(markdown.WarnBlock))
		})

		It("Type2 comment containing <script> → WarnBlock", func() {
			n, src := parseFirstHTMLBlock("<!-- x --><script>alert(1)</script>\n\n")
			Expect(n).NotTo(BeNil())
			Expect(policy.BlockDecision(n, src)).To(Equal(markdown.WarnBlock))
		})

		It("Type6 <div> without script/style → Keep", func() {
			n, src := parseFirstHTMLBlock("<div>\n<b>hello</b>\n</div>\n\n")
			Expect(n).NotTo(BeNil())
			Expect(policy.BlockDecision(n, src)).To(Equal(markdown.Keep))
		})
	})

	Describe("InlineDecision", func() {
		It("inline <script> → SuppressDocument", func() {
			n, src := parseFirstRawHTML("text <script>alert(1)</script> more")
			Expect(n).NotTo(BeNil())
			Expect(policy.InlineDecision(n, src)).To(Equal(markdown.SuppressDocument))
		})

		It("inline <style> → SuppressDocument", func() {
			n, src := parseFirstRawHTML("text <style>body{}</style> more")
			Expect(n).NotTo(BeNil())
			Expect(policy.InlineDecision(n, src)).To(Equal(markdown.SuppressDocument))
		})

		It("inline <b> → Keep", func() {
			n, src := parseFirstRawHTML("text <b>bold</b> more")
			Expect(n).NotTo(BeNil())
			Expect(policy.InlineDecision(n, src)).To(Equal(markdown.Keep))
		})
	})

	Describe("NonMarkdownDecision", func() {
		It(".vue/.js/.mjs/.ts → Refuse (including uppercase)", func() {
			for _, url := range []string{
				"https://github.com/org/repo/blob/main/foo.vue",
				"https://github.com/org/repo/blob/main/foo.js",
				"https://github.com/org/repo/blob/main/foo.mjs",
				"https://github.com/org/repo/blob/main/foo.ts",
				"https://github.com/org/repo/blob/main/foo.VUE",
				"https://github.com/org/repo/blob/main/foo.TS",
			} {
				Expect(policy.NonMarkdownDecision(url)).To(Equal(markdown.Refuse), url)
			}
		})

		It(".html → WarnNonMarkdown", func() {
			Expect(policy.NonMarkdownDecision("https://example.com/foo.html")).To(Equal(markdown.WarnNonMarkdown))
			Expect(policy.NonMarkdownDecision("https://example.com/foo.HTML")).To(Equal(markdown.WarnNonMarkdown))
		})

		It(".md/.png/.json → Keep", func() {
			for _, url := range []string{
				"https://example.com/foo.md",
				"https://example.com/foo.png",
				"https://example.com/foo.json",
			} {
				Expect(policy.NonMarkdownDecision(url)).To(Equal(markdown.Keep), url)
			}
		})
	})
})

var _ = Describe("SanitizeFrontmatter", func() {
	It("removes 'head' key for untrusted node and returns StripHead finding", func() {
		meta := map[string]any{
			"title": "My Doc",
			"head":  []any{"<script>alert(1)</script>"},
		}
		findings := markdown.SanitizeFrontmatter(meta, false)
		Expect(meta).NotTo(HaveKey("head"))
		Expect(meta["title"]).To(Equal("My Doc"))
		Expect(findings).To(HaveLen(1))
		Expect(findings[0].Decision).To(Equal(markdown.StripHead))
	})

	It("is a no-op when 'head' is absent", func() {
		meta := map[string]any{"title": "My Doc"}
		findings := markdown.SanitizeFrontmatter(meta, false)
		Expect(findings).To(BeEmpty())
		Expect(meta).To(HaveLen(1))
	})

	It("is a no-op for trusted nodes even when 'head' is present", func() {
		meta := map[string]any{"title": "My Doc", "head": "value"}
		findings := markdown.SanitizeFrontmatter(meta, true)
		Expect(meta).To(HaveKey("head"))
		Expect(findings).To(BeEmpty())
	})

	It("is idempotent: a second call on the same untrusted meta returns no finding", func() {
		meta := map[string]any{
			"title": "My Doc",
			"head":  []any{"<script>alert(1)</script>"},
		}
		first := markdown.SanitizeFrontmatter(meta, false)
		second := markdown.SanitizeFrontmatter(meta, false)
		Expect(first).To(HaveLen(1))
		Expect(second).To(BeEmpty())
	})
})
