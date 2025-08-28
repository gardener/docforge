package persona_test

// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

import (
	"embed"
	"os"
	"path/filepath"
	"testing"

	"github.com/gardener/docforge/pkg/core/manifest"
	"github.com/gardener/docforge/pkg/core/registry"
	"github.com/gardener/docforge/pkg/core/registry/repositoryhost"
	"github.com/gardener/docforge/pkg/osshim/filesystem"
	"github.com/gardener/docforge/pkg/plugins/persona"
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

		r := registry.NewRegistry(repositoryhost.NewLocal("https://github.com/gardener/docforge", "tests"))

		url := "https://github.com/gardener/docforge/blob/master/manifests/persona_filtering.yaml"

		// Create a temporary directory for test output
		tempDir := "/tmp/docforge_test_persona"

		// Create persona plugin with real filesystem (writes to temp dir)
		personaPlugin := persona.New(&filesystem.Local{}, tempDir)
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

		// Create a test node for the JavaScript file to be written
		testNode := &manifest.Node{
			Type: "file",
			Path: "js",
			FileType: manifest.FileType{
				File: "persona-filtering.js",
			},
		}

		// Process the test node to generate the JavaScript file
		Expect(personaPlugin.Process(testNode)).NotTo(HaveOccurred())

		// Verify the JavaScript file was written correctly
		expectedPath := filepath.Join(tempDir, testNode.Path, testNode.Name())
		writtenData, err := os.ReadFile(expectedPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(writtenData)).To(Equal(string(resultTPLBytes)))

		// Clean up test files
		os.RemoveAll(tempDir)

		// Verify manifest processing results
		Expect(len(files)).To(Equal(len(expected)))
		for i := range files {
			Expect(files[i]).To(Equal(expected[i]))
		}

	})
})
