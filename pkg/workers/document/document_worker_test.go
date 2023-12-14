package document_test

import (
	"context"
	"embed"
	"fmt"
	"net/url"
	"strings"
	"testing"

	_ "embed"

	"github.com/gardener/docforge/cmd/hugo"
	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/readers/repositoryhosts"
	"github.com/gardener/docforge/pkg/readers/repositoryhosts/repositoryhostsfakes"
	"github.com/gardener/docforge/pkg/workers/document"
	"github.com/gardener/docforge/pkg/workers/downloader/downloaderfakes"
	"github.com/gardener/docforge/pkg/workers/linkresolver/linkresolverfakes"
	"github.com/gardener/docforge/pkg/workers/linkvalidator/linkvalidatorfakes"
	"github.com/gardener/docforge/pkg/writers/writersfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestJobs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Frontmatter Suite")
}

//go:embed tests/*
var manifests embed.FS

var _ = Describe("Document resolving", func() {
	var (
		dw *document.DocumentWorker

		w *writersfakes.FakeWriter
	)
	BeforeEach(func() {
		localHost := repositoryhostsfakes.FakeRepositoryHost{}
		localHost.ManifestFromURLCalls(func(url string) (string, error) {
			content, err := manifests.ReadFile(url)
			return string(content), err
		})
		localHost.ToAbsLinkCalls(func(URL, link string) (string, error) {

			u, _ := url.Parse(URL)
			ulink, _ := url.Parse(link)
			return u.ResolveReference(ulink).String(), nil
		})
		localHost.ReadCalls(func(ctx context.Context, s string) ([]byte, error) {
			if strings.HasPrefix(s, "https://github.com/fake_owner/fake_repo/blob/master/") {
				return manifests.ReadFile("tests/" + strings.TrimPrefix(s, "https://github.com/fake_owner/fake_repo/blob/master/"))
			}
			return nil, nil
		})
		localHost.GetRawFormatLinkReturns("https://github.com/kubernetes/kubernetes/raw/master/logo/logo.png", nil)
		registry := &repositoryhostsfakes.FakeRegistry{}
		registry.GetCalls(func(s string) (repositoryhosts.RepositoryHost, error) {
			if strings.HasPrefix(s, "https://github.com") || s == "tests/baseline.yaml" {
				return &localHost, nil
			}
			return nil, fmt.Errorf("no sutiable repository host for %s", s)
		})
		hugo := hugo.Hugo{
			Enabled:        true,
			BaseURL:        "baseURL",
			IndexFileNames: []string{"readme.md", "readme", "read.me", "index.md", "index"},
		}
		df := &downloaderfakes.FakeInterface{}
		vf := &linkvalidatorfakes.FakeInterface{}
		lrf := &linkresolverfakes.FakeInterface{}
		lrf.ResolveLinkCalls(func(s1 string, n *manifest.Node, s2 string) (string, bool, error) {
			return s1, true, nil
		})
		w = &writersfakes.FakeWriter{}
		dw = document.NewDocumentWorker("__resources", df, vf, lrf, registry, hugo, w)
	})

	Context("#ProcessNode", func() {
		It("", func() {
			node := &manifest.Node{
				FileType: manifest.FileType{
					File:        "node",
					MultiSource: []string{"https://github.com/fake_owner/fake_repo/blob/master/target.md", "https://github.com/fake_owner/fake_repo/blob/master/target2.md"},
				},
				Type: "file",
				Path: "one",
			}
			err := dw.ProcessNode(context.TODO(), node)
			Expect(err).ToNot(HaveOccurred())
			name, path, cnt, nodegot := w.WriteArgsForCall(0)
			Expect(name).To(Equal("node"))
			Expect(path).To(Equal("one"))
			target, err := manifests.ReadFile("tests/expected_target.md")
			Expect(err).NotTo(HaveOccurred())
			target2, err := manifests.ReadFile("tests/expected_target2.md")
			fmt.Println(string(cnt))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(cnt)).To(Equal(string(target) + string(target2) + "\n"))
			Expect(node).To(Equal(nodegot))
		})

		It("", func() {
			node := &manifest.Node{
				FileType: manifest.FileType{
					File:   "node",
					Source: "https://github.com/fake_owner/fake_repo/blob/master/target.md",
				},
				Type: "file",
				Path: "one",
			}
			err := dw.ProcessNode(context.TODO(), node)
			Expect(err).ToNot(HaveOccurred())
			name, path, cnt, nodegot := w.WriteArgsForCall(0)
			Expect(name).To(Equal("node"))
			Expect(path).To(Equal("one"))
			target, err := manifests.ReadFile("tests/expected_target.md")
			Expect(err).NotTo(HaveOccurred())

			Expect(string(cnt)).To(Equal(string(target)))
			Expect(node).To(Equal(nodegot))
		})

	})
})
