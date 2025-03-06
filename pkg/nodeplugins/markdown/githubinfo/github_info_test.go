// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package githubinfo_test

import (
	"context"
	"errors"
	"testing"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/nodeplugins/markdown/githubinfo"
	"github.com/gardener/docforge/pkg/registry/registryfakes"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	"github.com/gardener/docforge/pkg/writers/writersfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestJobs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Validator Suite")
}

var _ = Describe("Executing WriteGithubInfo", func() {
	var (
		err      error
		registry *registryfakes.FakeInterface

		writer *writersfakes.FakeWriter
		worker *githubinfo.Worker

		ctx      context.Context
		taskNode *manifest.Node
	)

	BeforeEach(func() {
		registry = &registryfakes.FakeInterface{}
		writer = &writersfakes.FakeWriter{}
		registry.ReadGitInfoCalls(func(ctx context.Context, s string) ([]byte, error) {
			if s == "https://github.com/gardener/docforge/blob/master/README.md" {
				return []byte("repoHost1 source_content\n"), nil
			}
			if s == "https://github.com/gardener/docforge/blob/feature/A.md" {
				return []byte("repoHost2 multi_source_content\n"), nil
			}
			if s == "https://github.com/gardener/docforge/blob/feature/B.md" {
				return []byte("repoHost2 multi_source_content 2\n"), nil
			}
			if s == "https://github.com/gardener/docforge/blob/feature/C.md" {
				return nil, nil
			}
			return nil, repositoryhost.ErrResourceNotFound(s)
		})
		writer.WriteReturns(nil)
		ctx = context.Background()
		taskNode = &manifest.Node{
			Type: "file",
			FileType: manifest.FileType{
				File:        "README.md",
				Source:      "https://github.com/gardener/docforge/blob/master/README.md",
				MultiSource: []string{"https://github.com/gardener/docforge/blob/feature/A.md", "https://github.com/gardener/docforge/blob/feature/B.md"},
			},
		}
	})

	JustBeforeEach(func() {
		worker, err = githubinfo.NewGithubWorker(registry, writer)
		Expect(worker).NotTo(BeNil())
		Expect(err).NotTo(HaveOccurred())

		err = worker.WriteGithubInfo(ctx, taskNode)
	})

	Context("node without sources", func() {
		BeforeEach(func() {
			taskNode = &manifest.Node{Type: "dir", DirType: manifest.DirType{Dir: "folder"}}
		})
		It("should do nothing", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(writer.WriteCallCount()).To(Equal(0))
		})
	})

	Context("github info read for https://github.com/gardener/docforge/blob/feature/D.md fails with resource not found", func() {
		BeforeEach(func() {
			taskNode.MultiSource[1] = "https://github.com/gardener/docforge/blob/feature/D.md"
		})
		It("fails", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(repositoryhost.ErrResourceNotFound("https://github.com/gardener/docforge/blob/feature/D.md").Error()))
		})
	})

	Context("github info read for repoHost2://fake_multi_source returns nil []byte", func() {
		BeforeEach(func() {
			taskNode.MultiSource[1] = "https://github.com/gardener/docforge/blob/feature/C.md"
		})
		It("succeeded", func() {
			Expect(err).NotTo(HaveOccurred())
			_, _, content, _, _ := writer.WriteArgsForCall(0)
			Expect(string(content)).To(Equal("repoHost1 source_content\nrepoHost2 multi_source_content\n"))
		})
	})

	Context("write fails", func() {
		BeforeEach(func() {
			writer.WriteReturns(errors.New("fake_write_err"))
		})
		It("fails", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake_write_err"))
		})
	})

	It("succeeded", func() {
		Expect(err).NotTo(HaveOccurred())
		name, path, content, node, _ := writer.WriteArgsForCall(0)
		Expect(node).NotTo(BeNil())
		Expect(node.Name()).To(Equal("README.md"))
		Expect(node.Source).To(Equal("https://github.com/gardener/docforge/blob/master/README.md"))
		Expect(path).To(Equal(""))
		Expect(name).To(Equal("README.md"))
		Expect(string(content)).To(Equal("repoHost1 source_content\nrepoHost2 multi_source_content\nrepoHost2 multi_source_content 2\n"))
	})
})
