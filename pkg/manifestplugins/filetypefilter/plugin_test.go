package filetypefilter_test

// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

import (
	"embed"
	"testing"

	_ "embed"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/manifestplugins/filetypefilter"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestFileTypeFilterPlugin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "FileTypeFilter Suite")
}

//go:embed all:tests/*
var repo embed.FS

var _ = Describe("Docsy test", func() {
	It("References a resource in source that isn't allowed", func() {
		exampleFile := "manifests/unsupported_file_format.yaml"

		r := registry.NewRegistry(repositoryhost.NewLocalTest(repo, "https://github.com/gardener/docforge", "tests"))

		url := "https://github.com/gardener/docforge/blob/master/" + exampleFile
		contentFileFormats := []string{".md", ".yaml"}
		fileTypeFilterPlugin := filetypefilter.FileTypeFilter{ContentFileFormats: contentFileFormats}
		_, err := manifest.ResolveManifest(url, r, fileTypeFilterPlugin.PluginNodeTransformations()...)
		Expect(err.Error()).To(ContainSubstring("invalid.file isn't supported"))

	})
})
