package persona_test

// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

import (
	"embed"
	"path/filepath"
	"testing"

	"github.com/gardener/docforge/pkg/core/manifest"
	"github.com/gardener/docforge/pkg/core/registry"
	"github.com/gardener/docforge/pkg/core/registry/repositoryhost"
	"github.com/gardener/docforge/pkg/osfakes/osshim/osshimfakes"
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

		// Setup fake filesystem with in-memory storage
		fakeFs := &osshimfakes.FakeOs{}
		fileStore := make(map[string][]byte)

		// Stub filesystem operations to use in-memory storage
		fakeFs.WriteFileCalls(func(path string, data []byte, perm int) error {
			fileStore[path] = data
			return nil
		})

		// Use fake filesystem with a mock root path
		tempDir := "/mock/temp/dir"
		personaPlugin := persona.New(fakeFs, tempDir)
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

		// Verify filesystem operations were called correctly
		expectedPath := filepath.Join(tempDir, testNode.Path, testNode.Name())
		Expect(fakeFs.WriteFileCallCount()).To(Equal(1))
		writtenPath, writtenData, writtenPerm := fakeFs.WriteFileArgsForCall(0)
		Expect(writtenPath).To(Equal(expectedPath))
		Expect(string(writtenData)).To(Equal(string(resultTPLBytes)))
		Expect(writtenPerm).To(Equal(0644))

		// Verify manifest processing results
		Expect(len(files)).To(Equal(len(expected)))
		for i := range files {
			Expect(files[i]).To(Equal(expected[i]))
		}

	})
})
