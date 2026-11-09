package manifest_test

// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

import (
	"bytes"
	"embed"
	"fmt"
	"testing"

	_ "embed"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
	"k8s.io/klog/v2"
)

func TestManifest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Manifest Suite")
}

//go:embed tests/results/*
var results embed.FS

//go:embed all:tests/*
var repo embed.FS

var _ = Describe("Manifest test", func() {
	DescribeTable("Testing manifest file",
		func(example string) {
			var expected []*manifest.Node
			exampleFile := fmt.Sprintf("manifests/%s.yaml", example)
			resultFile := fmt.Sprintf("tests/results/%s.yaml", example)
			resultBytes, err := results.ReadFile(resultFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(yaml.Unmarshal([]byte(resultBytes), &expected)).NotTo(HaveOccurred())

			r := registry.NewRegistry(repositoryhost.NewLocalTest(repo, "https://github.com/gardener/docforge", "tests"))

			url := "https://github.com/gardener/docforge/blob/master/" + exampleFile
			allNodes, err := manifest.ResolveManifest(url, r)
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
				Expect(*files[i]).To(Equal(*expected[i]))
			}
		},
		Entry("covering _index.md use cases", "index_md_with_properties"),
		Entry("covering directory merges", "merging"),
		Entry("covering manifest use cases", "manifest"),
		Entry("covering fileTree filtering", "fileTree_filtering"),
		Entry("covering fileTree combined with manually added file", "filetree_with_manual_file"),
		Entry("covering weight-based ordering", "weight_ordering"),
	)

	Describe("When there are dirs with frontmatter collision", func() {
		It("should fail", func() {
			r := registry.NewRegistry(repositoryhost.NewLocalTest(repo, "https://github.com/gardener/docforge", "tests"))
			url := "https://github.com/gardener/docforge/blob/master/manifests/colliding_dir_frontmatters.yaml"

			_, err := manifest.ResolveManifest(url, r)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("there are multiple dirs with name foo and path . that have frontmatter. Please only use one"))
		})
	})

	Describe("Content trust model", func() {
		var r registry.Interface

		BeforeEach(func() {
			r = registry.NewRegistry(repositoryhost.NewLocalTest(repo, "https://github.com/gardener/docforge", "tests"))
		})

		resolve := func(fixture string) map[string]bool {
			url := "https://github.com/gardener/docforge/blob/master/manifests/" + fixture + ".yaml"
			nodes, err := manifest.ResolveManifest(url, r)
			Expect(err).NotTo(HaveOccurred())
			trust := map[string]bool{}
			for _, n := range nodes {
				if n.Type == "file" {
					trust[n.File] = n.AllowsScriptBlocks()
				}
			}
			return trust
		}

		// resolveWithLog captures klog output alongside the trust map.
		// klog writes to stderr by default (toStderr=true); SetOutput is only used when
		// toStderr=false, so we must flip it before capturing. These It blocks run
		// sequentially in ginkgo so there is no data race on the global klog state.
		resolveWithLog := func(fixture string) (map[string]bool, string) {
			var buf bytes.Buffer
			klog.LogToStderr(false)
			klog.SetOutput(&buf)
			defer func() {
				klog.Flush()
				klog.LogToStderr(true)
			}()
			trust := resolve(fixture)
			klog.Flush()
			return trust, buf.String()
		}

		It("trusts a file node with allowScriptBlocks: true", func() {
			trust := resolve("trust_file_trusted")
			Expect(trust["foo.txt"]).To(BeTrue())
			Expect(trust["two.txt"]).To(BeFalse())
		})

		It("inherits trust from a dir with allowScriptBlocks; sibling dir without flag stays untrusted", func() {
			trust := resolve("trust_dir_inheritance")
			Expect(trust["foo.txt"]).To(BeTrue())
			Expect(trust["two.txt"]).To(BeTrue())
			Expect(trust["one"]).To(BeFalse())
		})

		It("trusts all synthesized file nodes from a fileTree with allowScriptBlocks: true", func() {
			trust := resolve("trust_filetree")
			Expect(trust["foo.txt"]).To(BeTrue())
			Expect(trust["one"]).To(BeTrue())
			Expect(trust["two.txt"]).To(BeTrue())
		})

		It("does not trust nodes when a sub-manifest self-declares allowScriptBlocks", func() {
			// trust_sub_manifest_content.yaml declares allowScriptBlocks: true at its own top level;
			// snapshot/restore in readManifestContents discards it and emits a warning.
			// The invariant holds at any nesting depth (fromRoot = manifest == root).
			trust := resolve("trust_sub_manifest_self_declares")
			Expect(trust["foo.txt"]).To(BeFalse())
		})

		It("does not propagate trust into a sub-manifest even when the manifest-reference node has allowScriptBlocks: true", func() {
			// The parent (root manifest) sets allowScriptBlocks on the manifest-reference node.
			// computeTrust never trusts manifest-boundary nodes, and sub-manifest children
			// have fromRoot=false, so they remain untrusted. Direct root-manifest nodes are trusted.
			trust := resolve("trust_manifest_reference_no_effect")
			Expect(trust["foo.txt"]).To(BeTrue())  // direct root-manifest file
			Expect(trust["two.txt"]).To(BeFalse()) // file inside sub-manifest
		})

		It("does not trust any node when the root manifest self-declares allowScriptBlocks", func() {
			// The root manifest YAML has allowScriptBlocks: true at its top level.
			// snapshot/restore resets it to false and emits a warning.
			trust := resolve("trust_root_self_declare")
			Expect(trust["foo.txt"]).To(BeFalse())
		})

		It("ignores a YAML 'trusted' key (lowercase); AllowsScriptBlocks returns false", func() {
			// trusted is an unexported computed field; yaml.v2 cannot set it.
			trust := resolve("trust_yaml_cannot_set_computed")
			Expect(trust["foo.txt"]).To(BeFalse())
		})

		It("ignores capitalized and internal field names in YAML; AllowsScriptBlocks returns false", func() {
			// 'Trusted', 'TrustSource', 'trustManifest', 'trustDeclarant', 'trustDeclarantType'
			// are either unexported or unknown to yaml.v2 and are silently ignored.
			trust := resolve("trust_capitalized_fields")
			Expect(trust["foo.txt"]).To(BeFalse())
		})

		It("does not trust any node in a 3-level manifest chain even when each level declares allowScriptBlocks", func() {
			// root → mid (manifest-boundary, self-declares) → content (manifest-boundary, self-declares).
			// Trust never crosses a manifest boundary regardless of how many hops are involved.
			trust := resolve("trust_deep_chain")
			Expect(trust["foo.txt"]).To(BeFalse())
		})

		It("cannot override a parent-granted allowScriptBlocks by setting false in the remote manifest", func() {
			// trust_reference_value_preserved_content.yaml explicitly sets allowScriptBlocks: false to
			// try to revoke the parent's grant. snapshot/restore in readManifestContents ignores the
			// revocation (condition is false&&!true=false, so no warning). The parent's grant is preserved
			// for the manifest-boundary warning in computeTrust, but content in the sub-manifest is still
			// not trusted (manifest boundary). A direct root-manifest node with the flag IS trusted.
			trust := resolve("trust_reference_value_preserved")
			Expect(trust["foo.txt"]).To(BeFalse()) // inside sub-manifest, never trusted
			Expect(trust["two.txt"]).To(BeTrue())  // direct root-manifest node
		})

		It("trusts a multiSource file node with allowScriptBlocks: true; audit log includes all sources", func() {
			// A node with multiSource has HasContent()=true even when Source=="".
			// The per-node audit log must log all source URLs, not just Source.
			// The declaration-point summary must emit one line for the merged.md declaration.
			trust, log := resolveWithLog("trust_multisource")
			Expect(trust["merged.md"]).To(BeTrue())
			Expect(trust["one"]).To(BeFalse())
			// both source URLs appear in the audit line
			Expect(log).To(ContainSubstring("foo.txt"))
			Expect(log).To(ContainSubstring("two.txt"))
			// exactly one declaration-point summary line
			Expect(log).To(ContainSubstring("content-trust: trust declared"))
		})

		It("emits a declaration-point summary for a fileTree with allowScriptBlocks: true", func() {
			// The fileTree node is removed during Phase B; synthesized files carry trustDeclarant
			// set to the fileTree's NodePath (computed before removal). The summary log must
			// report type=fileTree and the correct trusted-nodes count (3 files: foo.txt, one, two.txt).
			_, log := resolveWithLog("trust_filetree")
			Expect(log).To(ContainSubstring("content-trust: trust declared"))
			Expect(log).To(ContainSubstring("type=fileTree"))
			Expect(log).To(ContainSubstring("trusted-nodes=3"))
		})

		It("emits a declaration-point summary for a dir with allowScriptBlocks: true", func() {
			// Two files inherit trust from trusted-docs dir; one file in sibling dir is untrusted.
			_, log := resolveWithLog("trust_dir_inheritance")
			Expect(log).To(ContainSubstring("content-trust: trust declared"))
			Expect(log).To(ContainSubstring("type=dir"))
			Expect(log).To(ContainSubstring("trusted-nodes=2"))
		})
	})
})
