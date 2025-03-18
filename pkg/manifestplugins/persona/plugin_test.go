package persona_test

// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

import (
	"embed"
	"fmt"
	"testing"

	_ "embed"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/manifestplugins/persona"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
)

func TestPersonaPlugin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Docsy Suite")
}

//go:embed tests/results/*
var results embed.FS

//go:embed all:tests/*
var repo embed.FS

var _ = Describe("Persona test", func() {
	DescribeTable("Process resolvePersonaFolders",
		func(example string) {
			var expected []*manifest.Node
			exampleFile := fmt.Sprintf("manifests/%s.yaml", example)
			resultFile := fmt.Sprintf("tests/results/%s.yaml", example)
			resultBytes, err := results.ReadFile(resultFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(yaml.Unmarshal([]byte(resultBytes), &expected)).NotTo(HaveOccurred())

			r := registry.NewRegistry(repositoryhost.NewLocalTest(repo, "https://github.com/gardener/docforge", "tests"))

			url := "https://github.com/gardener/docforge/blob/master/" + exampleFile
			personaPlugin := persona.Persona{}
			additionalTransformations := personaPlugin.PluginNodeTransformations()
			contentFileFormats := []string{".md", ".yaml"}
			allNodes, err := manifest.ResolveManifest(url, r, contentFileFormats, additionalTransformations...)
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
		Entry("covering persona filtering cases", "persona_filtering"),
	)
})
