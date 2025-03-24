package manifest_test

// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

import (
	"embed"
	"fmt"
	"testing"

	_ "embed"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
)

func TestManifest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Manifest Suite")
}

//go:embed tests/results/*
var results embed.FS

//go:embed all:tests/*
var repo embed.FS

var _ = Describe("Manifest test", func() {
	DescribeTable("Testing manifest file",
		func(example string) {
			var expected []*manifest.Node
			exampleFile := fmt.Sprintf("manifests/%s.yaml", example)
			resultFile := fmt.Sprintf("tests/results/%s.yaml", example)
			resultBytes, err := results.ReadFile(resultFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(yaml.Unmarshal([]byte(resultBytes), &expected)).NotTo(HaveOccurred())

			r := registry.NewRegistry(repositoryhost.NewLocalTest(repo, "https://github.com/gardener/docforge", "tests"))

			url := "https://github.com/gardener/docforge/blob/master/" + exampleFile
			allNodes, err := manifest.ResolveManifest(url, r)
			Expect(err).ToNot(HaveOccurred())
			files := []*manifest.Node{}
			for _, node := range allNodes {
				if node.Type == "file" {
					node.RemoveParent()
					files = append(files, node)
				}
			}
			Expect(len(files)).To(Equal(len(expected)))
			for i := range files {
				if expected[i].Frontmatter == nil {
					expected[i].Frontmatter = map[string]interface{}{}
				}
				Expect(*files[i]).To(Equal(*expected[i]))
			}
		},
		Entry("covering _index.md use cases", "index_md_with_properties"),
		Entry("covering directory merges", "merging"),
		Entry("covering manifest use cases", "manifest"),
		Entry("covering multisource", "multisource"),
		Entry("covering fileTree filtering", "fileTree_filtering"),
	)

	Describe("When there are dirs with frontmatter collision", func() {
		r := registry.NewRegistry(repositoryhost.NewLocalTest(repo, "https://github.com/gardener/docforge", "tests"))

		url := "https://github.com/gardener/docforge/blob/master/manifests/colliding_dir_frontmatters.yaml"
		_, err := manifest.ResolveManifest(url, r)
		Expect(err.Error()).To(ContainSubstring("there are multiple dirs with name foo and path . that have frontmatter. Please only use one"))
	})
})
