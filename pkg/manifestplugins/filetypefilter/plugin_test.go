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
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

func TestFileTypeFilterPlugin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "FileTypeFilter Suite")
}

//go:embed all:tests/*
var repo embed.FS

var _ = Describe("Docsy test", func() {
	DescribeTable("Errors",
		func(example string, errorMsg string) {
			exampleFile := fmt.Sprintf("manifests/%s.yaml", example)

			r := registry.NewRegistry(repositoryhost.NewLocalTest(repo, "https://github.com/gardener/docforge", "tests"))

			url := "https://github.com/gardener/docforge/blob/master/" + exampleFile
			contentFileFormats := []string{".md", ".yaml"}
			fileTypeFilterPlugin := filetypefilter.FileTypeFilter{ContentFileFormats: contentFileFormats}
			_, err := manifest.ResolveManifest(url, r, fileTypeFilterPlugin.PluginNodeTransformations()...)
			Expect(err.Error()).To(ContainSubstring(errorMsg))

		},
		Entry("when there are dirs with frontmatter collision", "colliding_dir_frontmatters", "there are multiple dirs with name foo and path . that have frontmatter. Please only use one"),
		Entry("referencing a resource in source that isn't allowed", "unsupported_file_format", "invalid.file isn't supported"),
	)
})
