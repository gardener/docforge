package filetypefilter_test

// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

import (
	"embed"
	"fmt"
	"testing"

	_ "embed"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/manifestplugins/filetypefilter"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
)

func TestFileTypeFilterPlugin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "FileTypeFilter Suite")
}

//go:embed tests/results/*
var results embed.FS

//go:embed all:tests/*
var repo embed.FS

var _ = Describe("Docsy test", func() {
	It("References a resource that isn't allowed", func() {
		var expected []*manifest.Node
		exampleFile := "manifests/filtered_file_format.yaml"
		resultFile := "tests/results/filtered_file_format.yaml"
		resultBytes, err := results.ReadFile(resultFile)
		Expect(err).ToNot(HaveOccurred())
		Expect(yaml.Unmarshal([]byte(resultBytes), &expected)).NotTo(HaveOccurred())

		r := registry.NewRegistry(repositoryhost.NewLocalTest(repo, "https://github.com/gardener/docforge", "tests"))

		url := "https://github.com/gardener/docforge/blob/master/" + exampleFile
		contentFileFormats := []string{".md", ".yaml"}
		fileTypeFilterPlugin := filetypefilter.FileTypeFilter{ContentFileFormats: contentFileFormats}
		allNodes, err := manifest.ResolveManifest(url, r, fileTypeFilterPlugin.PluginNodeTransformations()...)
		Expect(err).ToNot(HaveOccurred())
		files := []*manifest.Node{}
		for _, node := range allNodes {
			if node.Type == "file" {
				node.RemoveParent()
				files = append(files, node)
			}
		}

		Expect(len(files)).To(Equal(len(expected)))
		fmt.Printf("len: %v", len(files))
		for i := range files {
			Expect(files[i]).To(Equal(expected[i]))
		}
	})
})
