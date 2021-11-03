package reactor_test

import (
	"context"
	"errors"
	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/jobs"
	"github.com/gardener/docforge/pkg/reactor"
	"github.com/gardener/docforge/pkg/reactor/reactorfakes"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/writers/writersfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sync"
)

var _ = Describe("GithubInfo", func() {
	var (
		err    error
		reader *reactorfakes.FakeReader
		writer *writersfakes.FakeWriter
		work   jobs.WorkerFunc
	)
	BeforeEach(func() {
		reader = &reactorfakes.FakeReader{}
		writer = &writersfakes.FakeWriter{}
	})
	JustBeforeEach(func() {
		work, err = reactor.GitHubInfoWorkerFunc(reader, writer)
	})
	When("creating worker function", func() {
		It("creates the work func successfully", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(work).NotTo(BeNil())
		})
		Context("reader is not set", func() {
			BeforeEach(func() {
				reader = nil
			})
			It("should fails", func() {
				Expect(work).To(BeNil())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("reader is nil"))
			})
		})
		Context("writer is not set", func() {
			BeforeEach(func() {
				writer = nil
			})
			It("should fails", func() {
				Expect(work).To(BeNil())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("writer is nil"))
			})
		})
		When("invokes work func", func() {
			var (
				ctx  context.Context
				task interface{}
			)
			BeforeEach(func() {
				reader.ReadReturnsOnCall(0, []byte("source_content\n"), nil)
				reader.ReadReturnsOnCall(1, []byte("selector_content\n"), nil)
				reader.ReadReturnsOnCall(2, []byte("template_content\n"), nil)
				writer.WriteReturns(nil)
				ctx = context.Background()
				task = &reactor.GitHubInfoTask{
					Node: &api.Node{
						Name:             "fake_name",
						Source:           "fake_source",
						ContentSelectors: []api.ContentSelector{{Source: "fake_selector_source"}},
						Template:         &api.Template{Sources: map[string]*api.ContentSelector{"fake_key": {Source: "fake_template_source"}}},
					},
				}
			})
			JustBeforeEach(func() {
				Expect(work).NotTo(BeNil())
				Expect(err).NotTo(HaveOccurred())
				err = work(ctx, task)
			})
			It("succeeded", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(reader.ReadCallCount()).To(Equal(3))
				c, source := reader.ReadArgsForCall(0)
				Expect(c).To(Equal(ctx))
				Expect(source).To(Equal("fake_source"))
				_, source = reader.ReadArgsForCall(1)
				Expect(source).To(Equal("fake_selector_source"))
				_, source = reader.ReadArgsForCall(2)
				Expect(source).To(Equal("fake_template_source"))
				Expect(writer.WriteCallCount()).To(Equal(1))
				name, path, content, node := writer.WriteArgsForCall(0)
				Expect(node).NotTo(BeNil())
				Expect(node.Name).To(Equal("fake_name"))
				Expect(node.Source).To(Equal("fake_source"))
				Expect(path).To(Equal(""))
				Expect(name).To(Equal("fake_name"))
				Expect(string(content)).To(Equal("source_content\nselector_content\ntemplate_content\n"))
			})
			Context("task is invalid", func() {
				BeforeEach(func() {
					task = struct{}{}
				})
				It("fails", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("incorrect github info task"))
				})
			})
			Context("task is nil", func() {
				BeforeEach(func() {
					task = nil
				})
				It("fails", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("incorrect github info task"))
				})
			})
			Context("node without sources", func() {
				BeforeEach(func() {
					task = &reactor.GitHubInfoTask{Node: &api.Node{Name: "folder"}}
				})
				It("succeeded", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(reader.ReadCallCount()).To(Equal(0))
					Expect(writer.WriteCallCount()).To(Equal(0))
				})
			})
			Context("read fails", func() {
				BeforeEach(func() {
					reader.ReadReturnsOnCall(0, nil, errors.New("fake_read_err"))
				})
				It("fails", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake_read_err"))
				})
			})
			Context("read fails with resource not found", func() {
				BeforeEach(func() {
					reader.ReadReturnsOnCall(0, nil, resourcehandlers.ErrResourceNotFound("fake_target"))
				})
				It("succeeded", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(reader.ReadCallCount()).To(Equal(3))
					Expect(writer.WriteCallCount()).To(Equal(1))
					_, _, content, _ := writer.WriteArgsForCall(0)
					Expect(string(content)).To(Equal("selector_content\ntemplate_content\n"))
				})
			})
			Context("read returns nil []byte", func() {
				BeforeEach(func() {
					reader.ReadReturnsOnCall(0, nil, nil)
				})
				It("succeeded", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(reader.ReadCallCount()).To(Equal(3))
					Expect(writer.WriteCallCount()).To(Equal(1))
					_, _, content, _ := writer.WriteArgsForCall(0)
					Expect(string(content)).To(Equal("selector_content\ntemplate_content\n"))
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
		})
		When("creating new GitHub info writer", func() {
			var (
				wg              *sync.WaitGroup
				ctx             context.Context
				gitHubInfoTasks *jobs.JobQueue
				gitHubInfo      reactor.GitHubInfo
			)
			BeforeEach(func() {
				wg = &sync.WaitGroup{}
				ctx = context.Background()
			})
			JustBeforeEach(func() {
				Expect(work).NotTo(BeNil())
				Expect(err).NotTo(HaveOccurred())
				gitHubInfoTasks, err = jobs.NewJobQueue("GitHubInfo", 2, work, false, wg)
				gitHubInfo = reactor.NewGitHubInfo(gitHubInfoTasks)
			})
			It("succeeded", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(gitHubInfoTasks).NotTo(BeNil())
				Expect(gitHubInfo).NotTo(BeNil())
			})
			When("writing GitHub infos", func() {
				JustBeforeEach(func() {
					gitHubInfoTasks.Start(ctx)
					Expect(gitHubInfo.WriteGitHubInfo(&api.Node{Name: "name1", Source: "source1"})).To(BeTrue())
					Expect(gitHubInfo.WriteGitHubInfo(&api.Node{Name: "name2", Source: "source2"})).To(BeTrue())
				})
				It("writes GitHub info successfully", func() {
					wg.Wait()
					Expect(gitHubInfoTasks.GetProcessedTasksCount()).To(Equal(2))
					Expect(gitHubInfoTasks.GetErrorList()).To(BeNil())
					Expect(reader.ReadCallCount()).To(Equal(2))
					Expect(writer.WriteCallCount()).To(Equal(2))
				})
				Context("github tasks queue stopped", func() {
					JustBeforeEach(func() {
						wg.Wait()
						gitHubInfoTasks.Stop()
					})
					It("skips the tasks", func() {
						Expect(gitHubInfo.WriteGitHubInfo(&api.Node{Name: "name3", Source: "source3"})).To(BeFalse())
						Expect(gitHubInfoTasks.GetProcessedTasksCount()).To(Equal(2))
					})
				})
			})
		})
	})
})
