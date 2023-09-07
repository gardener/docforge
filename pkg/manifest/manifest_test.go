package manifest_test

// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

import (
	"embed"
	"errors"
	"fmt"
	"strings"

	_ "embed"

	"github.com/gardener/docforge/pkg/manifest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
)

//go:embed examples/*
var examples embed.FS

//go:embed results/*
var results embed.FS

type fakeFiles struct{}

func (f *fakeFiles) ManifestFromUrl(url string) (string, error) {
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

func (f *fakeFiles) FileTreeFromUrl(url string) ([]string, error) {
	files := map[string][]string{}
	files["https://test/website"] = []string{"blog/2023/_index.md"}
	files["https://test/blogs"] = []string{"2023/one", "2023/two.md"}
	if res, ok := files[url]; !ok {
		return nil, errors.New("err")
	} else {
		return res, nil
	}
	// files["/docs/development"] = []string{"local-setup.md"}
	// files["/docs/operations"] = []string{"local-setup.md"}
	// files["/docs/usage"] = []string{"local-setup.md"}
	// files["/docs/tutorials"] = []string{"local-setup.md"}

	// switch url {
	// case "https://test/website":
	// 	return []string{"blog/2023/_index.md"}, nil
	// case "pathValid":
	// 	return []string{"pV/1/a", "pV/1/2/A"}, nil
	// case "pathYataa":
	// 	return []string{"pY/b", "pY/1/B"}, nil
	// case "yataa2":
	// 	return []string{"pY/1/C"}, nil
	// default:
	// 	return []string{}, nil
	// }

}

func buildManifestFiles(exampleName string) ([]*manifest.Node, error) {
	var (
		err   error
		files []*manifest.Node
	)
	fc := manifest.FileCollector{}
	if err := manifest.ResolveManifest(exampleName, &fakeFiles{}, &fc); err != nil {
		return nil, err
	}
	if files, err = fc.Extract(); err != nil {
		return nil, err
	}
	return files, nil
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
