package persona_test

// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

import (
	"embed"
	"testing"

	"github.com/gardener/docforge/pkg/core/manifest"
	"github.com/gardener/docforge/pkg/plugins/persona"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	"github.com/gardener/docforge/pkg/writers/writersfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
)

func TestPersonaPlugin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Persona Suite")
}

//go:embed tests/results/*
var results embed.FS

//go:embed all:tests/*
var repo embed.FS

var _ = Describe("Persona test", func() {
	It("Processes resolvePersonaFolders", func() {
		var expected []*manifest.Node
		resultFile := "tests/results/persona_filtering.yaml"
		resultBytes, err := results.ReadFile(resultFile)
		Expect(err).ToNot(HaveOccurred())
		Expect(yaml.Unmarshal([]byte(resultBytes), &expected)).NotTo(HaveOccurred())

		resultTPLFile := "tests/results/persona-filtering.js"
		resultTPLBytes, err := results.ReadFile(resultTPLFile)
		Expect(err).ToNot(HaveOccurred())

		r := registry.NewRegistry(repositoryhost.NewLocalTest(repo, "https://github.com/gardener/docforge", "tests"))

		url := "https://github.com/gardener/docforge/blob/master/manifests/persona_filtering.yaml"

		// Use unified persona plugin for manifest transformations
		writer := writersfakes.FakeWriter{}
		personaPlugin := persona.New(&writer)
		personaTransformations := personaPlugin.ManifestTransformations()

		allNodes, err := manifest.ResolveManifest(url, r, personaTransformations...)
		Expect(err).ToNot(HaveOccurred())
		files := []*manifest.Node{}
		for _, node := range allNodes {
			if node.Type == "file" {
				node.RemoveParent()
				files = append(files, node)
			}
		}

		// Set the final node structure for processing
		Expect(personaPlugin.FinalNodeStructure(allNodes)).NotTo(HaveOccurred())

		// Process the root node to generate the JavaScript file
		Expect(personaPlugin.Process(allNodes[0])).NotTo(HaveOccurred())

		_, _, data, _, _ := writer.WriteArgsForCall(0)
		Expect(string(data)).To(Equal(string(resultTPLBytes)))
		Expect(len(files)).To(Equal(len(expected)))
		for i := range files {
			Expect(files[i]).To(Equal(expected[i]))
		}

	})
})
