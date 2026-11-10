// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package document_test

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/nodeplugins/markdown/document"
	"github.com/gardener/docforge/pkg/nodeplugins/markdown/linkresolver"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	"github.com/gardener/docforge/pkg/sitegen"
	"github.com/gardener/docforge/pkg/writers/writersfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// srcDocURL returns the test registry URL for docs/filename.
func srcDocURL(filename string) string {
	return "https://github.com/gardener/docforge/blob/master/docs/" + filename
}

// newTestCfg returns a sitegen config matching the one used in the suite BeforeEach.
func newTestCfg() sitegen.SimpleConfig {
	return sitegen.SimpleConfig{
		IsEnabled:    true,
		BaseURLValue: "baseURL",
		IndexFiles:   []string{"readme.md", "readme", "read.me", "index.md", "index"},
	}
}

// newPolicyWorker creates a Worker backed by the shared manifests embed.FS.
// failOnStripped controls whether policy violations return errors.
func newPolicyWorker(failOnStripped bool) (*document.Worker, *writersfakes.FakeWriter) {
	r := registry.NewRegistry(repositoryhost.NewLocalTest(manifests, "https://github.com/gardener/docforge", "tests"))
	cfg := newTestCfg()
	nodes, err := manifest.ResolveManifest("https://github.com/gardener/docforge/blob/master/docs/manifest.yaml", r)
	if err != nil {
		panic(fmt.Sprintf("newPolicyWorker: %v", err))
	}
	lr := linkresolver.New(nodes, r, cfg)
	w := &writersfakes.FakeWriter{}
	dw := document.NewDocumentWorker(lr, r, cfg, w)
	dw.SetFailOnStrippedContent(failOnStripped)
	return dw, w
}

// trustedNode returns a manifest.Node with AllowsScriptBlocks()==true by resolving
// tests/trust_manifest.yaml (which flags the node with allowScriptBlocks: true).
// nodes[0] is the synthetic root container; nodes[1] is the actual file node.
func trustedNode() *manifest.Node {
	r := registry.NewRegistry(repositoryhost.NewLocalTest(manifests, "https://github.com/gardener/docforge", "tests"))
	nodes, err := manifest.ResolveManifest("https://github.com/gardener/docforge/blob/master/trust_manifest.yaml", r)
	if err != nil {
		panic(fmt.Sprintf("trustedNode: %v", err))
	}
	return nodes[1]
}

// isPlaceholderOnly returns true when cnt is a YAML-only placeholder document (no body).
func isPlaceholderOnly(cnt []byte) bool {
	s := string(cnt)
	return strings.HasPrefix(s, "---\n") && strings.HasSuffix(s, "---\n") && strings.Count(s, "---\n") == 2
}

var _ = Describe("Document worker — content policy", func() {

	// ------------------------------------------------------------------
	// Fix 1: SanitizeFrontmatter runs AFTER MergeDocumentAndNodeFrontmatter
	// ------------------------------------------------------------------

	Describe("StripHead", func() {
		It("removes head that comes ONLY from manifest node frontmatter (untrusted)", func() {
			dw, w := newPolicyWorker(false)
			node := &manifest.Node{
				FileType: manifest.FileType{File: "no_head.md", Source: srcDocURL("no_head.md")},
				Type:     "file",
				Path:     "out",
				Frontmatter: map[string]interface{}{
					"head": []interface{}{map[string]interface{}{"tag": "link"}},
				},
			}
			Expect(dw.ProcessNode(context.TODO(), node)).To(Succeed())
			_, _, cnt, _ := w.WriteArgsForCall(0)
			Expect(string(cnt)).NotTo(ContainSubstring("head:"))
			// n.Frontmatter must not retain head (permanently deleted on the content path)
			Expect(node.Frontmatter).NotTo(HaveKey("head"))
		})

		It("removes head from source file frontmatter (untrusted)", func() {
			dw, w := newPolicyWorker(false)
			node := &manifest.Node{
				FileType: manifest.FileType{File: "with_head.md", Source: srcDocURL("with_head.md")},
				Type:     "file",
				Path:     "out",
			}
			Expect(dw.ProcessNode(context.TODO(), node)).To(Succeed())
			_, _, cnt, _ := w.WriteArgsForCall(0)
			Expect(string(cnt)).NotTo(ContainSubstring("head:"))
		})

		It("keeps head for a trusted node even when head comes from manifest frontmatter", func() {
			dw, w := newPolicyWorker(false)
			node := trustedNode()
			node.Frontmatter = map[string]interface{}{
				"head": []interface{}{map[string]interface{}{"tag": "link"}},
			}
			Expect(dw.ProcessNode(context.TODO(), node)).To(Succeed())
			_, _, cnt, _ := w.WriteArgsForCall(0)
			Expect(string(cnt)).To(ContainSubstring("head:"))
		})

		It("byte-identical output when source has no head and node has no head (two runs)", func() {
			dw1, w1 := newPolicyWorker(false)
			dw2, w2 := newPolicyWorker(false)
			n1 := &manifest.Node{FileType: manifest.FileType{File: "no_head.md", Source: srcDocURL("no_head.md")}, Type: "file", Path: "out"}
			n2 := &manifest.Node{FileType: manifest.FileType{File: "no_head.md", Source: srcDocURL("no_head.md")}, Type: "file", Path: "out"}
			Expect(dw1.ProcessNode(context.TODO(), n1)).To(Succeed())
			Expect(dw2.ProcessNode(context.TODO(), n2)).To(Succeed())
			_, _, cnt1, _ := w1.WriteArgsForCall(0)
			_, _, cnt2, _ := w2.WriteArgsForCall(0)
			Expect(cnt1).To(Equal(cnt2))
		})

		It("StripHead + flag: writes sanitized output then returns error", func() {
			dw, w := newPolicyWorker(true)
			node := &manifest.Node{
				FileType: manifest.FileType{File: "with_head.md", Source: srcDocURL("with_head.md")},
				Type:     "file",
				Path:     "out",
			}
			err := dw.ProcessNode(context.TODO(), node)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("stripped frontmatter"))
			Expect(w.WriteCallCount()).To(Equal(1))
			_, _, cnt, _ := w.WriteArgsForCall(0)
			Expect(string(cnt)).NotTo(ContainSubstring("head:"))
		})
	})

	// ------------------------------------------------------------------
	// Fix 2: Refuse in multiSource → placeholder (fail closed)
	// ------------------------------------------------------------------

	Describe("Refuse", func() {
		It("replaces the whole node with a placeholder when a non-markdown source is present", func() {
			dw, w := newPolicyWorker(false)
			node := &manifest.Node{
				FileType: manifest.FileType{
					File:        "test_out.md",
					MultiSource: []string{srcDocURL("target.md"), srcDocURL("fake.vue")},
				},
				Type: "file",
				Path: "out",
			}
			Expect(dw.ProcessNode(context.TODO(), node)).To(Succeed())
			_, _, cnt, _ := w.WriteArgsForCall(0)
			Expect(isPlaceholderOnly(cnt)).To(BeTrue(), "expected placeholder only, got: %s", string(cnt))
		})

		It("does not include any bytes from the accepted source when a refused source is present", func() {
			dw, w := newPolicyWorker(false)
			node := &manifest.Node{
				FileType: manifest.FileType{
					File:        "test_out.md",
					MultiSource: []string{srcDocURL("target.md"), srcDocURL("fake.vue")},
				},
				Type: "file",
				Path: "out",
			}
			Expect(dw.ProcessNode(context.TODO(), node)).To(Succeed())
			_, _, cnt, _ := w.WriteArgsForCall(0)
			Expect(string(cnt)).NotTo(ContainSubstring("Tested markdown file"))
		})

		It("placeholder does not contain the source URL", func() {
			dw, w := newPolicyWorker(false)
			node := &manifest.Node{
				FileType: manifest.FileType{File: "test_out.md", Source: srcDocURL("fake.js")},
				Type:     "file",
				Path:     "out",
			}
			Expect(dw.ProcessNode(context.TODO(), node)).To(Succeed())
			_, _, cnt, _ := w.WriteArgsForCall(0)
			Expect(string(cnt)).NotTo(ContainSubstring("fake.js"))
			Expect(string(cnt)).NotTo(ContainSubstring("github.com"))
		})

		It("with --fail-on-stripped-content: writes placeholder then returns error", func() {
			dw, w := newPolicyWorker(true)
			node := &manifest.Node{
				FileType: manifest.FileType{
					File:        "test_out.md",
					MultiSource: []string{srcDocURL("target.md"), srcDocURL("fake.vue")},
				},
				Type: "file",
				Path: "out",
			}
			err := dw.ProcessNode(context.TODO(), node)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("policy violation"))
			Expect(w.WriteCallCount()).To(Equal(1), "output must be written before the error")
			_, _, cnt, _ := w.WriteArgsForCall(0)
			Expect(isPlaceholderOnly(cnt)).To(BeTrue())
		})

		It("without --fail-on-stripped-content: returns nil", func() {
			dw, _ := newPolicyWorker(false)
			node := &manifest.Node{
				FileType: manifest.FileType{File: "test_out.md", Source: srcDocURL("fake.vue")},
				Type:     "file",
				Path:     "out",
			}
			Expect(dw.ProcessNode(context.TODO(), node)).To(Succeed())
		})

		It("placeholder title matches SuppressDocument placeholder for same node name", func() {
			dw1, w1 := newPolicyWorker(false)
			dw2, w2 := newPolicyWorker(false)
			// Refuse node (fake.vue)
			n1 := &manifest.Node{
				FileType: manifest.FileType{File: "my_doc.md", Source: srcDocURL("fake.vue")},
				Type:     "file",
				Path:     "out",
			}
			// SuppressDocument node (inline <script>)
			n2 := &manifest.Node{
				FileType: manifest.FileType{File: "my_doc.md", Source: srcDocURL("inline_script.md")},
				Type:     "file",
				Path:     "out",
			}
			Expect(dw1.ProcessNode(context.TODO(), n1)).To(Succeed())
			Expect(dw2.ProcessNode(context.TODO(), n2)).To(Succeed())
			_, _, cnt1, _ := w1.WriteArgsForCall(0)
			_, _, cnt2, _ := w2.WriteArgsForCall(0)
			Expect(cnt1).To(Equal(cnt2))
		})
	})

	// ------------------------------------------------------------------
	// SuppressDocument
	// ------------------------------------------------------------------

	Describe("SuppressDocument", func() {
		It("inline script in one source → whole node is placeholder", func() {
			dw, w := newPolicyWorker(false)
			node := &manifest.Node{
				FileType: manifest.FileType{File: "inline_script.md", Source: srcDocURL("inline_script.md")},
				Type:     "file",
				Path:     "out",
			}
			Expect(dw.ProcessNode(context.TODO(), node)).To(Succeed())
			_, _, cnt, _ := w.WriteArgsForCall(0)
			Expect(isPlaceholderOnly(cnt)).To(BeTrue())
			Expect(string(cnt)).To(ContainSubstring("Inline Script"))
		})

		It("multiSource with inline script → no bytes from either source", func() {
			dw, w := newPolicyWorker(false)
			node := &manifest.Node{
				FileType: manifest.FileType{
					File:        "multi.md",
					MultiSource: []string{srcDocURL("target.md"), srcDocURL("inline_script.md")},
				},
				Type: "file",
				Path: "out",
			}
			Expect(dw.ProcessNode(context.TODO(), node)).To(Succeed())
			_, _, cnt, _ := w.WriteArgsForCall(0)
			Expect(isPlaceholderOnly(cnt)).To(BeTrue())
			Expect(string(cnt)).NotTo(ContainSubstring("Tested markdown file"))
			Expect(string(cnt)).NotTo(ContainSubstring("alert(1)"))
		})

		It("SuppressDocument + flag: writes placeholder then returns error", func() {
			dw, w := newPolicyWorker(true)
			node := &manifest.Node{
				FileType: manifest.FileType{File: "inline_script.md", Source: srcDocURL("inline_script.md")},
				Type:     "file",
				Path:     "out",
			}
			err := dw.ProcessNode(context.TODO(), node)
			Expect(err).To(HaveOccurred())
			Expect(w.WriteCallCount()).To(Equal(1))
			_, _, cnt, _ := w.WriteArgsForCall(0)
			Expect(isPlaceholderOnly(cnt)).To(BeTrue())
		})

		It("placeholder does not contain the source URL", func() {
			dw, w := newPolicyWorker(false)
			node := &manifest.Node{
				FileType: manifest.FileType{File: "inline_script.md", Source: srcDocURL("inline_script.md")},
				Type:     "file",
				Path:     "out",
			}
			Expect(dw.ProcessNode(context.TODO(), node)).To(Succeed())
			_, _, cnt, _ := w.WriteArgsForCall(0)
			Expect(string(cnt)).NotTo(ContainSubstring("inline_script"))
			Expect(string(cnt)).NotTo(ContainSubstring("github.com"))
		})
	})

	// ------------------------------------------------------------------
	// StripBlock
	// ------------------------------------------------------------------

	Describe("StripBlock", func() {
		It("script block → stripped comment in output, not a placeholder", func() {
			dw, w := newPolicyWorker(false)
			node := &manifest.Node{
				FileType: manifest.FileType{File: "block_script.md", Source: srcDocURL("block_script.md")},
				Type:     "file",
				Path:     "out",
			}
			Expect(dw.ProcessNode(context.TODO(), node)).To(Succeed())
			_, _, cnt, _ := w.WriteArgsForCall(0)
			Expect(isPlaceholderOnly(cnt)).To(BeFalse())
			Expect(string(cnt)).To(ContainSubstring("<!-- docforge: stripped block -->"))
			Expect(string(cnt)).NotTo(ContainSubstring("<script>"))
		})

		It("worker always uses policy renderer: script block is never passed through", func() {
			dw, w := newPolicyWorker(false)
			node := &manifest.Node{
				FileType: manifest.FileType{File: "block_script.md", Source: srcDocURL("block_script.md")},
				Type:     "file",
				Path:     "out",
			}
			Expect(dw.ProcessNode(context.TODO(), node)).To(Succeed())
			_, _, cnt, _ := w.WriteArgsForCall(0)
			Expect(string(cnt)).NotTo(ContainSubstring("<script>"))
		})

		It("StripBlock + flag: writes stripped output then returns error", func() {
			dw, w := newPolicyWorker(true)
			node := &manifest.Node{
				FileType: manifest.FileType{File: "block_script.md", Source: srcDocURL("block_script.md")},
				Type:     "file",
				Path:     "out",
			}
			err := dw.ProcessNode(context.TODO(), node)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("policy violation"))
			Expect(w.WriteCallCount()).To(Equal(1))
			_, _, cnt, _ := w.WriteArgsForCall(0)
			Expect(string(cnt)).To(ContainSubstring("<!-- docforge: stripped block -->"))
		})

		It("StripBlock without flag: returns nil", func() {
			dw, _ := newPolicyWorker(false)
			node := &manifest.Node{
				FileType: manifest.FileType{File: "block_script.md", Source: srcDocURL("block_script.md")},
				Type:     "file",
				Path:     "out",
			}
			Expect(dw.ProcessNode(context.TODO(), node)).To(Succeed())
		})

		It("no-violation node with flag: no error", func() {
			dw, _ := newPolicyWorker(true)
			node := &manifest.Node{
				FileType: manifest.FileType{File: "no_head.md", Source: srcDocURL("no_head.md")},
				Type:     "file",
				Path:     "out",
			}
			Expect(dw.ProcessNode(context.TODO(), node)).To(Succeed())
		})
	})

	// ------------------------------------------------------------------
	// Content-less _index.md + head in manifest frontmatter
	// ------------------------------------------------------------------

	Describe("Content-less node frontmatter sanitization", func() {
		It("node.Frontmatter is never mutated: original has head before and after ProcessNode", func() {
			dw, w := newPolicyWorker(false)
			originalHead := []interface{}{map[string]interface{}{"tag": "link"}}
			node := &manifest.Node{
				FileType: manifest.FileType{File: "_index.md"},
				Type:     "file",
				Path:     "section",
				Frontmatter: map[string]interface{}{
					"title": "Section",
					"head":  originalHead,
				},
			}
			Expect(dw.ProcessNode(context.TODO(), node)).To(Succeed())
			// Original node.Frontmatter is unchanged (no mutation).
			Expect(node.Frontmatter["head"]).To(Equal(originalHead))
			// The node passed to Write is a copy with head removed.
			Expect(w.WriteCallCount()).To(Equal(1))
			_, _, _, gotNode := w.WriteArgsForCall(0)
			Expect(gotNode.Frontmatter).NotTo(HaveKey("head"))
		})

		It("StripHead on content-less node + flag: writes sanitized copy then errors; original unchanged", func() {
			dw, w := newPolicyWorker(true)
			originalHead := []interface{}{map[string]interface{}{"tag": "link"}}
			node := &manifest.Node{
				FileType: manifest.FileType{File: "_index.md"},
				Type:     "file",
				Path:     "section",
				Frontmatter: map[string]interface{}{
					"title": "Section",
					"head":  originalHead,
				},
			}
			err := dw.ProcessNode(context.TODO(), node)
			Expect(err).To(HaveOccurred())
			Expect(w.WriteCallCount()).To(Equal(1), "write must occur before the policy error")
			_, _, _, gotNode := w.WriteArgsForCall(0)
			Expect(gotNode.Frontmatter).NotTo(HaveKey("head"))
			// Original node.Frontmatter must not have been mutated.
			Expect(node.Frontmatter["head"]).To(Equal(originalHead))
		})

		It("50 content-less untrusted nodes with head processed concurrently: no mutation", func() {
			const numNodes = 50
			r := registry.NewRegistry(repositoryhost.NewLocalTest(manifests, "https://github.com/gardener/docforge", "tests"))
			cfg := newTestCfg()
			nodes, err := manifest.ResolveManifest("https://github.com/gardener/docforge/blob/master/docs/manifest.yaml", r)
			Expect(err).NotTo(HaveOccurred())
			lr := linkresolver.New(nodes, r, cfg)

			type nodeResult struct {
				original *manifest.Node
				origHead interface{}
				err      error
			}
			results := make([]nodeResult, numNodes)
			fakeWriters := make([]*writersfakes.FakeWriter, numNodes)
			dws := make([]*document.Worker, numNodes)
			for i := 0; i < numNodes; i++ {
				fakeWriters[i] = &writersfakes.FakeWriter{}
				dws[i] = document.NewDocumentWorker(lr, r, cfg, fakeWriters[i])
			}

			var wg sync.WaitGroup
			for i := 0; i < numNodes; i++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					head := []interface{}{map[string]interface{}{"tag": fmt.Sprintf("meta%d", idx)}}
					n := &manifest.Node{
						FileType: manifest.FileType{File: fmt.Sprintf("section%d.md", idx)},
						Type:     "file",
						Path:     fmt.Sprintf("path%d", idx),
						Frontmatter: map[string]interface{}{
							"title": fmt.Sprintf("Section %d", idx),
							"head":  head,
						},
					}
					results[idx] = nodeResult{original: n, origHead: head}
					results[idx].err = dws[idx].ProcessNode(context.Background(), n)
				}(i)
			}
			wg.Wait()

			for i := 0; i < numNodes; i++ {
				Expect(results[i].err).NotTo(HaveOccurred(), "node %d should not error", i)
				// Original node must not have been mutated.
				Expect(results[i].original.Frontmatter["head"]).To(Equal(results[i].origHead),
					"node %d original head was mutated", i)
				// The copy passed to Write must not have head.
				Expect(fakeWriters[i].WriteCallCount()).To(Equal(1))
				_, _, _, gotNode := fakeWriters[i].WriteArgsForCall(0)
				Expect(gotNode.Frontmatter).NotTo(HaveKey("head"),
					"node %d written frontmatter should not have head", i)
			}
		})
	})

	// ------------------------------------------------------------------
	// Byte-identity: multiSource without findings
	// ------------------------------------------------------------------

	Describe("Byte-identity: multiSource without policy findings", func() {
		It("2-source output contains all content from both sources", func() {
			dw, w := newPolicyWorker(false)
			multiNode := &manifest.Node{
				FileType: manifest.FileType{
					File:        "combined.md",
					MultiSource: []string{srcDocURL("target.md"), srcDocURL("target2.md")},
				},
				Type: "file",
				Path: "out",
			}
			Expect(dw.ProcessNode(context.TODO(), multiNode)).To(Succeed())
			_, _, cnt, _ := w.WriteArgsForCall(0)
			Expect(string(cnt)).To(ContainSubstring("Tested markdown file 1"))
			Expect(string(cnt)).To(ContainSubstring("Tested markdown file 2"))
		})

		It("same single-source node processed twice produces identical output", func() {
			dw1, w1 := newPolicyWorker(false)
			dw2, w2 := newPolicyWorker(false)
			n1 := &manifest.Node{
				FileType: manifest.FileType{File: "no_head.md", Source: srcDocURL("no_head.md")},
				Type:     "file",
				Path:     "out",
			}
			n2 := &manifest.Node{
				FileType: manifest.FileType{File: "no_head.md", Source: srcDocURL("no_head.md")},
				Type:     "file",
				Path:     "out",
			}
			Expect(dw1.ProcessNode(context.TODO(), n1)).To(Succeed())
			Expect(dw2.ProcessNode(context.TODO(), n2)).To(Succeed())
			_, _, cnt1, _ := w1.WriteArgsForCall(0)
			_, _, cnt2, _ := w2.WriteArgsForCall(0)
			Expect(cnt1).To(Equal(cnt2))
		})
	})

	// ------------------------------------------------------------------
	// Concurrent processing: 50 nodes, mixed trusted/untrusted
	// ------------------------------------------------------------------

	Describe("Concurrent processing", func() {
		It("processes 50 nodes concurrently; per-node findings are correct", func() {
			r := registry.NewRegistry(repositoryhost.NewLocalTest(manifests, "https://github.com/gardener/docforge", "tests"))
			cfg := newTestCfg()
			nodes, err := manifest.ResolveManifest("https://github.com/gardener/docforge/blob/master/docs/manifest.yaml", r)
			Expect(err).NotTo(HaveOccurred())
			lr := linkresolver.New(nodes, r, cfg)

			const numNodes = 50
			type result struct {
				content []byte
				err     error
			}
			results := make([]result, numNodes)
			fakeWriters := make([]*writersfakes.FakeWriter, numNodes)
			dws := make([]*document.Worker, numNodes)

			for i := 0; i < numNodes; i++ {
				fakeWriters[i] = &writersfakes.FakeWriter{}
				dws[i] = document.NewDocumentWorker(lr, r, cfg, fakeWriters[i])
			}

			var wg sync.WaitGroup
			for i := 0; i < numNodes; i++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					var src string
					if idx%2 == 0 {
						src = srcDocURL("block_script.md")
					} else {
						src = srcDocURL("no_head.md")
					}
					node := &manifest.Node{
						FileType: manifest.FileType{
							File:   fmt.Sprintf("doc%d.md", idx),
							Source: src,
						},
						Type: "file",
						Path: fmt.Sprintf("path%d", idx),
					}
					results[idx].err = dws[idx].ProcessNode(context.Background(), node)
					if fakeWriters[idx].WriteCallCount() > 0 {
						_, _, cnt, _ := fakeWriters[idx].WriteArgsForCall(0)
						results[idx].content = cnt
					}
				}(i)
			}
			wg.Wait()

			for i := 0; i < numNodes; i++ {
				Expect(results[i].err).NotTo(HaveOccurred(), "node %d should not error", i)
				if i%2 == 0 {
					Expect(string(results[i].content)).To(
						ContainSubstring("<!-- docforge: stripped block -->"),
						"even node %d (block_script.md) should have stripped block", i)
				} else {
					Expect(string(results[i].content)).NotTo(
						ContainSubstring("<!-- docforge: stripped block -->"),
						"odd node %d (no_head.md) should not have stripped block", i)
				}
			}
		})
	})
})
