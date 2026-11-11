// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package document_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	gast "github.com/yuin/goldmark/ast"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/nodeplugins/markdown/document"
	fmpkg "github.com/gardener/docforge/pkg/nodeplugins/markdown/document/frontmatter"
	mdpkg "github.com/gardener/docforge/pkg/nodeplugins/markdown/document/markdown"
	"github.com/gardener/docforge/pkg/nodeplugins/markdown/linkresolver"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	"github.com/gardener/docforge/pkg/sitegen"
	"github.com/gardener/docforge/pkg/writers/writersfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/klog/v2"
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

// isPlaceholderOnly returns true when cnt is a placeholder document with the suppression body.
func isPlaceholderOnly(cnt []byte) bool {
	const body = "\n<!-- docforge: document suppressed due to active content -->\n"
	s := string(cnt)
	return strings.HasPrefix(s, "---\n") &&
		strings.Count(s, "---\n") == 2 &&
		strings.HasSuffix(s, body)
}

// placeholderBytes returns the exact bytes writePlaceholder produces for title.
// Only correct for simple titles without YAML special characters.
func placeholderBytes(title string) []byte {
	return []byte("---\ntitle: " + title + "\n---\n\n<!-- docforge: document suppressed due to active content -->\n")
}

// captureWarnings runs fn with klog redirected to a buffer and returns everything
// logged. klog cascades a warning to every severity <= WARNING, so a single finding
// yields more than one line; tests must compare counts against a known baseline
// rather than asserting an absolute number.
func captureWarnings(fn func()) string {
	var buf bytes.Buffer
	klog.LogToStderr(false)
	klog.SetOutput(&buf)
	defer func() {
		klog.SetOutput(os.Stderr)
		klog.LogToStderr(true)
	}()
	fn()
	klog.Flush()
	return buf.String()
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

		// F1 regression: a non-Markdown first source must not let "head" from a later
		// Markdown source bypass sanitization.
		It("non-markdown first source: head from a later markdown source is stripped and recorded (untrusted)", func() {
			dw, w := newPolicyWorker(false)
			node := &manifest.Node{
				FileType: manifest.FileType{
					File:        "combo.md",
					Source:      srcDocURL("target3.html"),
					MultiSource: []string{srcDocURL("with_head.md")},
				},
				Type: "file",
				Path: "out",
			}
			var out string
			capture := captureWarnings(func() {
				Expect(dw.ProcessNode(context.TODO(), node)).To(Succeed())
			})
			_, _, cnt, _ := w.WriteArgsForCall(0)
			out = string(cnt)
			Expect(out).NotTo(ContainSubstring("head:"))
			// a StripHead finding was recorded by the per-source sanitization pass
			Expect(capture).To(ContainSubstring("removed 'head' frontmatter key"))
		})

		It("non-markdown first source + flag: writes sanitized output then returns error (untrusted)", func() {
			dw, w := newPolicyWorker(true)
			node := &manifest.Node{
				FileType: manifest.FileType{
					File:        "combo.md",
					Source:      srcDocURL("target3.html"),
					MultiSource: []string{srcDocURL("with_head.md")},
				},
				Type: "file",
				Path: "out",
			}
			err := dw.ProcessNode(context.TODO(), node)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("stripped frontmatter"))
			Expect(w.WriteCallCount()).To(Equal(1))
			_, _, cnt, _ := w.WriteArgsForCall(0)
			Expect(string(cnt)).NotTo(ContainSubstring("head:"))
		})

		It("non-markdown first source: head is kept for a trusted node", func() {
			dw, w := newPolicyWorker(false)
			node := trustedNode()
			node.FileType = manifest.FileType{
				File:        "combo.md",
				Source:      srcDocURL("target3.html"),
				MultiSource: []string{srcDocURL("with_head.md")},
			}
			Expect(node.AllowsScriptBlocks()).To(BeTrue())
			Expect(dw.ProcessNode(context.TODO(), node)).To(Succeed())
			_, _, cnt, _ := w.WriteArgsForCall(0)
			Expect(string(cnt)).To(ContainSubstring("head:"))
		})

		It("non-markdown first source with clean markdown: output byte-identical to the old (link-modifier) path", func() {
			dw, w := newPolicyWorker(false)
			node := &manifest.Node{
				FileType: manifest.FileType{
					File:        "combo.md",
					Source:      srcDocURL("target3.html"),
					MultiSource: []string{srcDocURL("no_head.md")},
				},
				Type: "file",
				Path: "out",
			}
			Expect(dw.ProcessNode(context.TODO(), node)).To(Succeed())
			_, _, workerOut, _ := w.WriteArgsForCall(0)

			// Old path: non-markdown source is written raw, markdown source rendered by
			// NewLinkModifierRenderer, concatenated with no separator.
			htmlRaw, err := manifests.ReadFile("tests/docs/target3.html")
			Expect(err).NotTo(HaveOccurred())
			mdSrc, err := manifests.ReadFile("tests/docs/no_head.md")
			Expect(err).NotTo(HaveOccurred())
			mdAst, err := mdpkg.Parse(mdpkg.New(), mdSrc)
			Expect(err).NotTo(HaveOccurred())
			var mdBuf bytes.Buffer
			Expect(mdpkg.NewLinkModifierRenderer().Render(&mdBuf, mdSrc, mdAst)).To(Succeed())
			expected := append(append([]byte{}, htmlRaw...), mdBuf.Bytes()...)

			Expect(workerOut).To(Equal(expected))
		})

		It("markdown multiSource with head: head stripped exactly once (no duplicate finding)", func() {
			count := func(node *manifest.Node) int {
				dw, _ := newPolicyWorker(false)
				out := captureWarnings(func() {
					Expect(dw.ProcessNode(context.TODO(), node)).To(Succeed())
				})
				return strings.Count(out, "removed 'head' frontmatter key")
			}
			single := count(&manifest.Node{
				FileType: manifest.FileType{File: "with_head.md", Source: srcDocURL("with_head.md")},
				Type:     "file",
				Path:     "out",
			})
			multi := count(&manifest.Node{
				FileType: manifest.FileType{
					File:        "combo.md",
					Source:      srcDocURL("with_head.md"),
					MultiSource: []string{srcDocURL("no_head.md")},
				},
				Type: "file",
				Path: "out",
			})
			Expect(single).To(BeNumerically(">", 0))
			Expect(multi).To(Equal(single), "head must be stripped the same number of times as a single source (no duplicate finding)")
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
			Expect(cnt).To(Equal(placeholderBytes("Test Out")), "expected exact placeholder, got: %s", string(cnt))
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
			Expect(cnt).To(Equal(placeholderBytes("Test Out")))
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
			Expect(cnt).To(Equal(placeholderBytes("Inline Script")))
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
			Expect(cnt).To(Equal(placeholderBytes("Multi")))
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
			Expect(cnt).To(Equal(placeholderBytes("Inline Script")))
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
		It("content-less copy: top-level map copied with head removed; original keeps head; nested values shared (writer only reads)", func() {
			dw, w := newPolicyWorker(false)
			originalHead := []interface{}{map[string]interface{}{"tag": "link"}}
			sharedParams := map[string]interface{}{"inner": "value"}
			node := &manifest.Node{
				FileType: manifest.FileType{File: "_index.md"},
				Type:     "file",
				Path:     "section",
				Frontmatter: map[string]interface{}{
					"title":  "Section",
					"head":   originalHead,
					"params": sharedParams,
				},
			}
			Expect(dw.ProcessNode(context.TODO(), node)).To(Succeed())
			// Original top-level map keeps all its keys (not mutated).
			Expect(node.Frontmatter).To(HaveKey("head"))
			Expect(node.Frontmatter["head"]).To(Equal(originalHead))
			// The node passed to Write is a shallow copy with head removed from the copy.
			Expect(w.WriteCallCount()).To(Equal(1))
			_, _, _, gotNode := w.WriteArgsForCall(0)
			Expect(gotNode.Frontmatter).NotTo(HaveKey("head"))
			// Nested values are shared between original and copy (shallow copy). This is
			// safe because the writer (hugoIndexContent) only marshals them, never mutates.
			// Demonstrate the sharing: a mutation of the original nested map is visible
			// through the written copy.
			sharedParams["mutated"] = true
			gotParams, ok := gotNode.Frontmatter["params"].(map[string]interface{})
			Expect(ok).To(BeTrue())
			Expect(gotParams).To(HaveKey("mutated"))
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

// newPolicyWorkerWithCfg creates a Worker with the given sitegen config.
func newPolicyWorkerWithCfg(cfg sitegen.Config) (*document.Worker, *writersfakes.FakeWriter) {
	r := registry.NewRegistry(repositoryhost.NewLocalTest(manifests, "https://github.com/gardener/docforge", "tests"))
	nodes, err := manifest.ResolveManifest("https://github.com/gardener/docforge/blob/master/docs/manifest.yaml", r)
	if err != nil {
		panic(fmt.Sprintf("newPolicyWorkerWithCfg: %v", err))
	}
	lr := linkresolver.New(nodes, r, cfg)
	w := &writersfakes.FakeWriter{}
	return document.NewDocumentWorker(lr, r, cfg, w), w
}

var _ = Describe("Document worker — byte-identity for clean content", func() {

	// none (nil config): worker output equals NewLinkModifierRenderer output for same source.
	// Valid because nil config skips ComputeNodeTitle, so no frontmatter is injected.
	It("none (nil config): untrusted clean node byte-equal to NewLinkModifierRenderer output", func() {
		dw, fw := newPolicyWorkerWithCfg(nil)
		node := &manifest.Node{
			FileType: manifest.FileType{File: "no_head.md", Source: srcDocURL("no_head.md")},
			Type:     "file",
			Path:     "out",
		}
		Expect(dw.ProcessNode(context.TODO(), node)).To(Succeed())
		_, _, workerOut, _ := fw.WriteArgsForCall(0)

		src, err := manifests.ReadFile("tests/docs/no_head.md")
		Expect(err).NotTo(HaveOccurred())
		docAst, err := mdpkg.Parse(mdpkg.New(), src)
		Expect(err).NotTo(HaveOccurred())
		var lmrBuf bytes.Buffer
		Expect(mdpkg.NewLinkModifierRenderer().Render(&lmrBuf, src, docAst)).To(Succeed())

		Expect(workerOut).To(Equal(lmrBuf.Bytes()))
	})

	// vitepress mode: for the same pre-processed AST, PolicyRenderer and
	// NewLinkModifierRenderer produce identical bytes (policy layer is transparent).
	It("vitepress mode: PolicyRenderer byte-equal to NewLinkModifierRenderer for same preprocessed AST", func() {
		vitepressCfg := sitegen.SimpleConfig{
			IsEnabled:       true,
			IndexFiles:      []string{"index.md", "_index.md"},
			IndexFileTarget: "index.md",
		}

		src, err := manifests.ReadFile("tests/docs/no_head.md")
		Expect(err).NotTo(HaveOccurred())

		md := mdpkg.New()
		policyAst, err := mdpkg.Parse(md, src)
		Expect(err).NotTo(HaveOccurred())
		lmrAst, err := mdpkg.Parse(md, src)
		Expect(err).NotTo(HaveOccurred())

		n1 := &manifest.Node{FileType: manifest.FileType{File: "no_head.md"}, Path: "out"}
		n2 := &manifest.Node{FileType: manifest.FileType{File: "no_head.md"}, Path: "out"}
		fmpkg.ComputeNodeTitle(policyAst.(*gast.Document), n1, vitepressCfg)
		fmpkg.MergeDocumentAndNodeFrontmatter(policyAst.(*gast.Document), n1)
		fmpkg.ComputeNodeTitle(lmrAst.(*gast.Document), n2, vitepressCfg)
		fmpkg.MergeDocumentAndNodeFrontmatter(lmrAst.(*gast.Document), n2)

		var policyBuf, lmrBuf bytes.Buffer
		Expect(mdpkg.NewPolicyRenderer(mdpkg.WithTrusted(false)).Render(&policyBuf, src, policyAst)).To(Succeed())
		Expect(mdpkg.NewLinkModifierRenderer().Render(&lmrBuf, src, lmrAst)).To(Succeed())

		Expect(policyBuf.Bytes()).To(Equal(lmrBuf.Bytes()))
	})
})
