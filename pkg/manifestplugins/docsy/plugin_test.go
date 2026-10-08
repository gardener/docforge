package docsy_test

// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

import (
	"embed"
	"fmt"
	"testing"

	_ "embed"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/manifestplugins/docsy"
	"github.com/gardener/docforge/pkg/manifestplugins/markdown"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	"github.com/gardener/docforge/pkg/sitegen"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
)

func TestDocsyPlugin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Docsy Suite")
}

//go:embed tests/results/*
var results embed.FS

//go:embed all:tests/*
var repo embed.FS

var _ = Describe("Docsy test", func() {
	DescribeTable("Process editThisPage",
		func(example string) {
			var expected []*manifest.Node
			exampleFile := fmt.Sprintf("manifests/%s.yaml", example)
			resultFile := fmt.Sprintf("tests/results/%s.yaml", example)
			resultBytes, err := results.ReadFile(resultFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(yaml.Unmarshal([]byte(resultBytes), &expected)).NotTo(HaveOccurred())

			r := registry.NewRegistry(repositoryhost.NewLocalTest(repo, "https://github.com/gardener/docforge", "tests"))

			url := "https://github.com/gardener/docforge/blob/master/" + exampleFile
			markdownPlugin := markdown.Markdown{}
			additionalTransformations := markdownPlugin.PluginNodeTransformations()
			docsyPlugin := docsy.Docsy{}
			additionalTransformations = append(additionalTransformations, docsyPlugin.PluginNodeTransformations()...)
			allNodes, err := manifest.ResolveManifest(url, r, additionalTransformations...)
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
		Entry("covering _index.md use cases", "index_md_with_properties"),
		Entry("covering type file", "file"),
	)
})

// TestDocsyFromField verifies that path_base_for_github_subdir.from uses the
// correct output filename (honouring sitegen.Config.UsesManifestNameInEditPath and
// IndexFileName) and strips any structural directories configured via
// StructuralDirs.
func TestDocsyFromField(t *testing.T) {
	r := registry.NewRegistry(repositoryhost.NewLocalTest(repo, "https://github.com/gardener/docforge", "tests"))

	cases := []struct {
		name         string
		cfg          sitegen.Config
		wantFrom     string
		wantTo       string
		manifestName string // default "index_md_with_properties"; override for README.md cases
		nodeFile     string // default "_index.md"; override for README.md cases
		nodePath     string // default "foo/bar/two"
	}{
		{
			name:     "nil Config: .from uses original filename",
			cfg:      nil,
			wantFrom: "foo/bar/two/_index.md",
			wantTo:   "_index.md",
		},
		{
			name: "Hugo mode: _index.md stays _index.md in .from",
			cfg: sitegen.SimpleConfig{
				IsEnabled:            true,
				IndexFileTarget:      "_index.md",
				ManifestNameInEditPath: true,
			},
			wantFrom: "foo/bar/two/_index.md",
			wantTo:   "_index.md",
		},
		{
			name: "VitePress mode: _index.md becomes index.md in .from",
			cfg: sitegen.SimpleConfig{
				IsEnabled:       true,
				IndexFileTarget: "index.md",
			},
			wantFrom: "foo/bar/two/index.md",
			wantTo:   "_index.md",
		},
		{
			// Hugo mode: structural dirs must NOT be stripped — this preserves byte-identity
			// with docforge-main (old TrimPrefix("hugo/") was always a no-op).
			name: "Hugo mode with structural dirs: dir prefix NOT stripped",
			cfg: sitegen.SimpleConfig{
				IsEnabled:            true,
				IndexFileTarget:      "_index.md",
				StructDirs:           []string{"foo"},
				ManifestNameInEditPath: true,
			},
			wantFrom: "foo/bar/two/_index.md",
			wantTo:   "_index.md",
		},
		{
			name: "VitePress with structural dir foo: index.md name and dir stripped",
			cfg: sitegen.SimpleConfig{
				IsEnabled:       true,
				IndexFileTarget: "index.md",
				StructDirs:      []string{"foo"},
			},
			wantFrom: "bar/two/index.md",
			wantTo:   "_index.md",
		},
		{
			// Hugo preserves the manifest filename in .from even when the output
			// will be _index.md — legacy byte-identity with pre-fix releases.
			name: "Hugo mode README.md node: legacy value keeps README.md in .from",
			cfg: sitegen.SimpleConfig{
				IsEnabled:            true,
				IndexFiles:           []string{"README.md"},
				IndexFileTarget:      "_index.md",
				ManifestNameInEditPath: true,
			},
			wantFrom:     "foo/bar/two/README.md",
			wantTo:       "README.md",
			manifestName: "readme_section",
			nodeFile:     "README.md",
		},
		{
			// VitePress renames README.md to index.md in .from.
			name: "VitePress mode README.md node: renamed to index.md in .from",
			cfg: sitegen.SimpleConfig{
				IsEnabled:       true,
				IndexFiles:      []string{"README.md"},
				IndexFileTarget: "index.md",
			},
			wantFrom:     "foo/bar/two/index.md",
			wantTo:       "README.md",
			manifestName: "readme_section",
			nodeFile:     "README.md",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			manifestName := c.manifestName
			if manifestName == "" {
				manifestName = "index_md_with_properties"
			}
			manifestURL := "https://github.com/gardener/docforge/blob/master/manifests/" + manifestName + ".yaml"

			nodeFile := c.nodeFile
			if nodeFile == "" {
				nodeFile = "_index.md"
			}

			markdownPlugin := markdown.Markdown{}
			dp := docsy.Docsy{Config: c.cfg}
			transforms := append(markdownPlugin.PluginNodeTransformations(), dp.PluginNodeTransformations()...)

			nodes, err := manifest.ResolveManifest(manifestURL, r, transforms...)
			if err != nil {
				t.Fatalf("ResolveManifest: %v", err)
			}

			var target *manifest.Node
			for _, n := range nodes {
				if n.Type == "file" && n.File == nodeFile && n.Path == "foo/bar/two" {
					target = n
					break
				}
			}
			if target == nil {
				t.Fatalf("no %s file node at foo/bar/two found", nodeFile)
			}

			pbgsd, ok := target.Frontmatter["path_base_for_github_subdir"].(map[interface{}]interface{})
			if !ok {
				t.Fatalf("path_base_for_github_subdir missing or wrong type: %T",
					target.Frontmatter["path_base_for_github_subdir"])
			}
			gotFrom, _ := pbgsd["from"].(string)
			gotTo, _ := pbgsd["to"].(string)

			if gotFrom != c.wantFrom {
				t.Errorf(".from = %q, want %q", gotFrom, c.wantFrom)
			}
			if gotTo != c.wantTo {
				t.Errorf(".to = %q, want %q", gotTo, c.wantTo)
			}
		})
	}
}
