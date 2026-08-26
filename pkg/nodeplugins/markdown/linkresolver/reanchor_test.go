// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package linkresolver_test

import (
	"github.com/gardener/docforge/pkg/nodeplugins/markdown/linkresolver"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("#ReAnchorRootAbsolute", func() {
	structuralDirs := []string{"content", "static"}

	DescribeTable("re-anchors root-absolute links deterministically",
		func(link, resourcePath, expected string) {
			Expect(linkresolver.ReAnchorRootAbsolute(link, resourcePath, structuralDirs)).To(Equal(expected))
		},
		Entry("real artifact image",
			"/docs/getting-started/images/podrick/scene-0.webp",
			"hugo/content/docs/getting-started/podrick.md",
			"/hugo/content/docs/getting-started/images/podrick/scene-0.webp"),
		Entry("markdown link",
			"/docs/other/page.md",
			"hugo/content/docs/a.md",
			"/hugo/content/docs/other/page.md"),
		Entry("double-segment content",
			"/content/y.webp",
			"content/docs/content/x.md",
			"/content/content/y.webp"),
		Entry("no structural dir in resourcePath",
			"/docs/x.png",
			"website/documentation/getting-started/a.md",
			"/docs/x.png"),
		Entry("root slash unchanged", "/", "hugo/content/docs/a.md", "/"),
		Entry("protocol-relative host unchanged", "//host/x.png", "hugo/content/docs/a.md", "//host/x.png"),
		Entry("empty unchanged", "", "hugo/content/docs/a.md", ""),
		Entry("dot-relative unchanged", "./x.md", "hugo/content/docs/a.md", "./x.md"),
		Entry("dot-dot-relative unchanged", "../y.md", "hugo/content/docs/a.md", "../y.md"),
		Entry("fragment unchanged", "#frag", "hugo/content/docs/a.md", "#frag"),
		Entry("query unchanged", "?q=1", "hugo/content/docs/a.md", "?q=1"),
	)
})
