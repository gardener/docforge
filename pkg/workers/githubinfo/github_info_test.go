// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0
package githubinfo_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/readers/repositoryhosts"
	"github.com/gardener/docforge/pkg/readers/repositoryhosts/repositoryhostsfakes"
	"github.com/gardener/docforge/pkg/workers/githubinfo"
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
		err       error
		registry  *repositoryhostsfakes.FakeRegistry
		repoHost1 *repositoryhostsfakes.FakeRepositoryHost
		repoHost2 *repositoryhostsfakes.FakeRepositoryHost
		writer    *writersfakes.FakeWriter
		worker    *githubinfo.GitHubInfoWorker

		ctx      context.Context
		taskNode *manifest.Node
	)
	BeforeEach(func() {
		registry = &repositoryhostsfakes.FakeRegistry{}
		repoHost1 = &repositoryhostsfakes.FakeRepositoryHost{}
		repoHost2 = &repositoryhostsfakes.FakeRepositoryHost{}
		writer = &writersfakes.FakeWriter{}

		registry.GetCalls(func(s string) (repositoryhosts.RepositoryHost, error) {
			if strings.HasPrefix(s, "repoHost1:") {
				return repoHost1, nil
			}
			if strings.HasPrefix(s, "repoHost2:") {
				return repoHost2, nil
			}
			return nil, fmt.Errorf("no sutiable repository host for %s", s)

		})
		repoHost1.ReadGitInfoReturns([]byte("repoHost1 source_content\n"), nil)
		repoHost2.ReadGitInfoReturnsOnCall(0, []byte("repoHost2 multi_source_content\n"), nil)
		repoHost2.ReadGitInfoReturnsOnCall(1, []byte("repoHost2 multi_source_content 2\n"), nil)
		writer.WriteReturns(nil)
		ctx = context.Background()
		taskNode = &manifest.Node{
			Type: "file",
			FileType: manifest.FileType{
				File:        "fake_name",
				Source:      "repoHost1://fake_source",
				MultiSource: []string{"repoHost2://fake_multi_source", "repoHost2://fake_multi_source2"},
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
			Expect(registry.GetCallCount()).To(Equal(0))
			Expect(writer.WriteCallCount()).To(Equal(0))
		})
	})
	Context("node has a source that no repo host can process", func() {
		BeforeEach(func() {
			taskNode.MultiSource[1] = "repoHost3://fake_multi_source2"
		})
		It("fails", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no sutiable repository host for repoHost3://fake_multi_source2"))
		})
	})
	Context("github info read for repoHost2://fake_multi_source fails with resource not found", func() {
		BeforeEach(func() {
			repoHost2.ReadGitInfoReturnsOnCall(1, nil, repositoryhosts.ErrResourceNotFound("fake_target"))
		})
		It("succeeded", func() {
			Expect(err).NotTo(HaveOccurred())
			_, _, content, _ := writer.WriteArgsForCall(0)
			Expect(string(content)).To(Equal("repoHost1 source_content\nrepoHost2 multi_source_content\n"))
		})
	})
	Context("github info read for repoHost2://fake_multi_source returns nil []byte", func() {
		BeforeEach(func() {
			repoHost2.ReadGitInfoReturnsOnCall(1, nil, nil)
		})
		It("succeeded", func() {
			Expect(err).NotTo(HaveOccurred())
			_, _, content, _ := writer.WriteArgsForCall(0)
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
		name, path, content, node := writer.WriteArgsForCall(0)
		Expect(node).NotTo(BeNil())
		Expect(node.Name()).To(Equal("fake_name"))
		Expect(node.Source).To(Equal("repoHost1://fake_source"))
		Expect(path).To(Equal(""))
		Expect(name).To(Equal("fake_name"))
		Expect(string(content)).To(Equal("repoHost1 source_content\nrepoHost2 multi_source_content\nrepoHost2 multi_source_content 2\n"))
	})
})
