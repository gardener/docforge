// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package linkresolver_test

import (
	"errors"

	"github.com/gardener/docforge/cmd/hugo"
	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/nodeplugins/markdown/linkresolver"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

// absLinkFixture builds a LinkResolver and source node for the Operations/README.md scenario.
// Manifest: Operations/README.md (source: operations-readme.md)
//
//	Operations/assets/garden-overview.drawio.png
//	Operations/assets/network-connectivity.drawio.png
//	Installation/README.md (source: other-section.md)
func absLinkFixture() (linkresolver.LinkResolver, *manifest.Node, string) {
	reg := registry.NewRegistry(repositoryhost.NewLocalTest(manifests, "https://github.com/gardener/docforge", "tests"))
	lr := linkresolver.LinkResolver{
		Repositoryhosts: reg,
		Hugo:            hugo.Hugo{Enabled: false},
		SourceToNode:    make(map[string][]*manifest.Node),
	}
	nodes, err := manifest.ResolveManifest(
		"https://github.com/gardener/docforge/blob/master/abslink.yaml",
		lr.Repositoryhosts,
	)
	Expect(err).NotTo(HaveOccurred())
	for _, n := range nodes {
		if n.Source != "" {
			lr.SourceToNode[n.Source] = append(lr.SourceToNode[n.Source], n)
		}
		for _, s := range n.MultiSource {
			lr.SourceToNode[s] = append(lr.SourceToNode[s], n)
		}
	}
	src := "https://github.com/gardener/docforge/blob/master/operations-readme.md"
	srcNode := lr.SourceToNode[src][0]
	return lr, srcNode, src
}

var _ = Describe("Absolute link rewriting", func() {
	Context("#ResolveResourceLink absolute-to-relative rewrites", func() {

		DescribeTable("rewrites in-repo absolute links to relative paths",
			func(link, expectedLink string, expectStripErr bool) {
				lr, node, src := absLinkFixture()
				got, err := lr.ResolveResourceLink(link, node, src)
				if expectStripErr {
					var se linkresolver.ErrStripLink
					Expect(errors.As(err, &se)).To(BeTrue(), "expected ErrStripLink")
					return
				}
				Expect(err).NotTo(HaveOccurred())
				Expect(got).To(Equal(expectedLink))
			},

			// Class A — absolute-from-source-root asset links -> relative to Operations/README.md
			Entry("Class A: asset in sibling dir",
				"https://github.com/gardener/docforge/blob/master/assets/garden-overview.drawio.png",
				"assets/garden-overview.drawio.png",
				false,
			),
			Entry("Class A: second asset in sibling dir",
				"https://github.com/gardener/docforge/blob/master/assets/network-connectivity.drawio.png",
				"assets/network-connectivity.drawio.png",
				false,
			),

			// Class A — absolute link to another forged doc in a different section
			Entry("Class A: link to a forged doc in another section",
				"https://github.com/gardener/docforge/blob/master/other-section.md",
				"../Installation/README.md",
				false,
			),

			// Class B — full blob URL into the forged set (same as Class A above since
			// the test registry uses the same host/org/repo; explicitly labelled for clarity)
			Entry("Class B: full blob URL resolves to asset",
				"https://github.com/gardener/docforge/blob/master/assets/garden-overview.drawio.png",
				"assets/garden-overview.drawio.png",
				false,
			),

			// Pass-through cases — must be unchanged
			Entry("relative link not in manifest passes through as absolute URL",
				"clickhere.md",
				// resolves in the repo to blob/master/clickhere.md, but not in abslink manifest
				// → passed through as the resolved absolute blob URL
				"https://github.com/gardener/docforge/blob/master/clickhere.md",
				false,
			),
			// Absolute URL on the same host but pointing to a file NOT in the manifest
			// (e.g. a different repo on the same GitHub instance) passes through unchanged.
			Entry("absolute URL to same-host file not in manifest passes through",
				"https://github.com/gardener/docforge/blob/master/linkresolution3.md",
				"https://github.com/gardener/docforge/blob/master/linkresolution3.md",
				false,
			),
			Entry("anchor-only link unchanged",
				"#section",
				"#section",
				false,
			),
		)
	})
})
