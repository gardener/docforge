package manifest_test

// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

import (
	"embed"
	"errors"
	"fmt"
	"strings"
	"testing"

	_ "embed"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/readers/repositoryhosts/repositoryhostsfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
)

func TestManifest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Manifest Suite")
}

//go:embed tests/examples/*
var examples embed.FS

//go:embed tests/results/*
var results embed.FS

var _ = Describe("Manifest test", func() {
	Describe("F", func() {
		DescribeTable("Testing manifest file",
			func(example string) {
				var expected []*manifest.Node
				exampleFile := fmt.Sprintf("tests/examples/%s.yaml", example)
				resultFile := fmt.Sprintf("tests/results/%s.yaml", example)
				resultBytes, err := results.ReadFile(resultFile)
				Expect(err).ToNot(HaveOccurred())
				yaml.Unmarshal([]byte(resultBytes), &expected)

				fakeFiles := &repositoryhostsfakes.FakeRepositoryHost{}
				fakeFiles.ManifestFromURLCalls(func(url string) (string, error) {
					url = strings.TrimPrefix(url, "https://test")
					content, err := examples.ReadFile(url)
					return string(content), err
				})
				fakeFiles.ToAbsLinkCalls(func(url, link string) (string, error) {
					if strings.HasPrefix(link, "/") {
						return "https://test" + link, nil
					}
					return link, nil
				})
				fakeFiles.FileTreeFromURLCalls(func(url string) ([]string, error) {
					files := map[string][]string{}
					files["https://test/website"] = []string{"blog/2023/_index.md"}
					files["https://test/blogs"] = []string{"2023/one", "2023/two.md"}
					if res, ok := files[url]; !ok {
						return nil, errors.New("err")
					} else {
						return res, nil
					}
				})
				fakeR := repositoryhostsfakes.FakeRegistry{}
				fakeR.GetReturns(fakeFiles, nil)

				allNodes, err := manifest.ResolveManifest(exampleFile, &fakeR)
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
			Entry("covering _index.md use cases", "_index_md_with_properties"),
			Entry("covering fileTree use cases and dir merges", "filetree"),
			Entry("covering manifest use cases", "manifest"),
		)
	})
})
