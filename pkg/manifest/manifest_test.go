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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
)

func TestManifest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Manifest Suite")
}

//go:embed examples/*
var examples embed.FS

//go:embed results/*
var results embed.FS

type fakeFiles struct{}

func (f *fakeFiles) ManifestFromURL(url string) (string, error) {
	url = strings.TrimPrefix(url, "https://test")
	content, err := examples.ReadFile(url)
	return string(content), err
}
func (f *fakeFiles) BuildAbsLink(url, link string) (string, error) {
	if strings.HasPrefix(link, "/") {
		return "https://test" + link, nil
	}
	return link, nil
}

func (f *fakeFiles) FileTreeFromURL(url string) ([]string, error) {
	files := map[string][]string{}
	files["https://test/website"] = []string{"blog/2023/_index.md"}
	files["https://test/blogs"] = []string{"2023/one", "2023/two.md"}
	if res, ok := files[url]; !ok {
		return nil, errors.New("err")
	} else {
		return res, nil
	}
}

func collectFiles(n *manifest.Node) []*manifest.Node {
	if n.Type == "file" {
		n.RemoveParent()
		return []*manifest.Node{n}
	}
	out := []*manifest.Node{}
	for _, child := range n.Structure {
		out = append(out, collectFiles(child)...)
	}
	return out
}

func buildManifestFiles(exampleName string) ([]*manifest.Node, error) {
	var (
		err  error
		root *manifest.Node
	)
	if root, err = manifest.ResolveManifest(exampleName, &fakeFiles{}); err != nil {
		return nil, err
	}
	return collectFiles(root), nil
}

var _ = Describe("Manifest test", func() {
	Describe("F", func() {
		DescribeTable("Testing manifest file",
			func(example string) {
				var expected []*manifest.Node
				exampleFile := fmt.Sprintf("examples/%s.yaml", example)
				resultFile := fmt.Sprintf("results/%s.yaml", example)
				resultBytes, err := results.ReadFile(resultFile)
				Expect(err).ToNot(HaveOccurred())

				yaml.Unmarshal([]byte(resultBytes), &expected)
				files, err := buildManifestFiles(exampleFile)

				Expect(err).ToNot(HaveOccurred())
				Expect(len(files)).To(Equal(len(expected)))
				for i := range files {
					Expect(*files[i]).To(Equal(*expected[i]))
				}
			},
			Entry("covering _index.md use cases", "_index_md_with_properties"),
			Entry("covering fileTree use cases and dir merges", "filetree"),
			Entry("covering manifest use cases", "manifest"),
		)
	})
})
