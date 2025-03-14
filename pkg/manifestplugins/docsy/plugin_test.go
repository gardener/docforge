package docsy_test

// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

import (
	"embed"
	"fmt"
	"testing"

	_ "embed"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/manifestplugins/docsy"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
)

func TestDocsyPlugin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Docsy Suite")
}

//go:embed tests/results/*
var results embed.FS

//go:embed all:tests/*
var repo embed.FS

var _ = Describe("Docsy test", func() {
	DescribeTable("Process editThisPage",
		func(example string) {
			var expected []*manifest.Node
			exampleFile := fmt.Sprintf("manifests/%s.yaml", example)
			resultFile := fmt.Sprintf("tests/results/%s.yaml", example)
			resultBytes, err := results.ReadFile(resultFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(yaml.Unmarshal([]byte(resultBytes), &expected)).NotTo(HaveOccurred())

			r := registry.NewRegistry(repositoryhost.NewLocalTest(repo, "https://github.com/gardener/docforge", "tests"))

			url := "https://github.com/gardener/docforge/blob/master/" + exampleFile
			docsyPlugin := docsy.Docsy{}
			additionalTransformations := docsyPlugin.PluginNodeTransformations()
			allNodes, err := manifest.ResolveManifest(url, r, additionalTransformations...)
			Expect(err).ToNot(HaveOccurred())
			files := []*manifest.Node{}
			for _, node := range allNodes {
				if node.Type == "file" {
					files = append(files, node)
				}
			}

			Expect(len(files)).To(Equal(len(expected)))
			for i := range files {
				if expected[i].Frontmatter == nil {
					expected[i].Frontmatter = map[string]interface{}{}
				}
				Expect(files[i].Frontmatter).To(Equal(expected[i].Frontmatter))
			}
		},
		Entry("covering _index.md use cases", "index_md_with_properties"),
		Entry("covering type file", "file"),
		Entry("covering type dir", "dir"),
		Entry("covering multisource", "multisource"),
	)
})
