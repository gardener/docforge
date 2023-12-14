// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package linkvalidator_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sync"
	"testing"

	"github.com/gardener/docforge/pkg/document"
	"github.com/gardener/docforge/pkg/httpclient/httpclientfakes"
	"github.com/gardener/docforge/pkg/reactor/jobs"
	"github.com/gardener/docforge/pkg/reactor/linkvalidator"
	"github.com/gardener/docforge/pkg/readers/repositoryhosts/repositoryhostsfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestJobs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Validator Suite")
}

var _ = Describe("Validator", func() {
	var (
		err         error
		httpClient  *httpclientfakes.FakeClient
		resHandlers *repositoryhostsfakes.FakeRegistry

		worker            *linkvalidator.ValidatorWorker
		linkDestination   string
		contentSourcePath string
	)
	BeforeEach(func() {
		httpClient = &httpclientfakes.FakeClient{}
		resHandlers = &repositoryhostsfakes.FakeRegistry{}
	})
	JustBeforeEach(func() {
		worker, err = linkvalidator.NewValidatorWorker(httpClient, resHandlers)
	})
	When("creating worker function", func() {
		It("creates the work func successfully", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(worker).NotTo(BeNil())
		})
		Context("http client is nil", func() {
			BeforeEach(func() {
				httpClient = nil
			})
			It("should fails", func() {
				Expect(worker).To(BeNil())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("httpClient is nil"))
			})
		})
		Context("resource registry is nil", func() {
			BeforeEach(func() {
				resHandlers = nil
			})
			It("should fails", func() {
				Expect(worker).To(BeNil())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("repositoryhosts is nil"))
			})
		})
		When("invokes work func", func() {
			var (
				ctx     context.Context
				linkURL *url.URL
			)
			BeforeEach(func() {
				ctx = context.Background()
				httpClient.DoReturns(&http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader([]byte(""))),
				}, nil)
				linkURL, err = url.Parse("https://fake_host/fake_link")
				Expect(err).NotTo(HaveOccurred())
				linkDestination = "fake_destination"
				contentSourcePath = "fake_path"

			})
			JustBeforeEach(func() {
				Expect(worker).NotTo(BeNil())
				Expect(err).NotTo(HaveOccurred())
				err = worker.Validate(ctx, linkURL, linkDestination, contentSourcePath)
			})
			It("succeeded", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(httpClient.DoCallCount()).To(Equal(1))
				req := httpClient.DoArgsForCall(0)
				Expect(req).NotTo(BeNil())
				Expect(req.Host).To(Equal("fake_host"))
				Expect(resHandlers.GetCallCount()).To(Equal(1))
				link := resHandlers.GetArgsForCall(0)
				Expect(link).To(Equal("https://fake_host/fake_link"))
			})
			Context("localhost", func() {
				BeforeEach(func() {
					linkURL, err = url.Parse("https://127.0.0.1/fake_link")
					Expect(err).NotTo(HaveOccurred())

				})
				It("skips link validation", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(httpClient.DoCallCount()).To(Equal(0))
				})
			})
			Context("sample host", func() {
				BeforeEach(func() {
					linkURL, err = url.Parse("https://foo.bar/fake_link")
					Expect(err).NotTo(HaveOccurred())
				})
				It("skips link validation", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(httpClient.DoCallCount()).To(Equal(0))
				})
			})
			Context("url is not valid", func() {
				BeforeEach(func() {
					Expect(err).NotTo(HaveOccurred())
					linkURL = &url.URL{
						Scheme: "https",
						Host:   "invalid host",
					}
				})
				It("fails", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("invalid URL"))
					Expect(httpClient.DoCallCount()).To(Equal(0))
				})
			})
			Context("http client returns errors", func() {
				BeforeEach(func() {
					httpClient.DoReturnsOnCall(0, nil, errors.New("fake_error"))
				})
				It("succeeded", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(httpClient.DoCallCount()).To(Equal(1))
				})
			})
			Context("http client returns StatusTooManyRequests", func() {
				BeforeEach(func() {
					httpClient.DoReturnsOnCall(0, &http.Response{
						StatusCode: http.StatusTooManyRequests,
						Body:       io.NopCloser(bytes.NewReader([]byte(""))),
					}, nil)
				})
				It("retries on StatusTooManyRequests", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(httpClient.DoCallCount()).To(Equal(2))
				})
			})
			Context("http client returns StatusUnauthorized", func() {
				BeforeEach(func() {
					httpClient.DoReturnsOnCall(0, &http.Response{
						StatusCode: http.StatusUnauthorized,
						Body:       io.NopCloser(bytes.NewReader([]byte(""))),
					}, nil)
				})
				It("returns on StatusUnauthorized", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(httpClient.DoCallCount()).To(Equal(1))
				})
			})
			Context("http client returns error status code", func() {
				BeforeEach(func() {
					httpClient.DoReturnsOnCall(0, &http.Response{
						StatusCode: http.StatusInternalServerError,
						Body:       io.NopCloser(bytes.NewReader([]byte(""))),
					}, nil)
				})
				It("retries on error status code", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(httpClient.DoCallCount()).To(Equal(2))
				})
			})
			Context("http client returns error on retry", func() {
				BeforeEach(func() {
					httpClient.DoReturns(nil, errors.New("fake_error"))
					httpClient.DoReturnsOnCall(0, &http.Response{
						StatusCode: http.StatusInternalServerError,
						Body:       io.NopCloser(bytes.NewReader([]byte(""))),
					}, nil)
				})
				It("succeeded", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(httpClient.DoCallCount()).To(Equal(2))
				})
			})
			Context("http client returns error code on retry", func() {
				BeforeEach(func() {
					httpClient.DoReturns(&http.Response{
						StatusCode: http.StatusInternalServerError,
						Body:       io.NopCloser(bytes.NewReader([]byte(""))),
					}, nil)
				})
				It("succeeded", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(httpClient.DoCallCount()).To(Equal(2))
				})
			})
			When("resource handlers for the link is found", func() {
				var (
					resourceHandler   *repositoryhostsfakes.FakeRepositoryHost
					handlerHttpClient *httpclientfakes.FakeClient
				)
				BeforeEach(func() {
					resourceHandler = &repositoryhostsfakes.FakeRepositoryHost{}
					resHandlers.GetReturns(resourceHandler)
					handlerHttpClient = &httpclientfakes.FakeClient{}
					handlerHttpClient.DoReturns(&http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewReader([]byte(""))),
					}, nil)
					resourceHandler.GetClientReturns(handlerHttpClient)
				})
				It("uses handler's client", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(httpClient.DoCallCount()).To(Equal(0))
					Expect(handlerHttpClient.DoCallCount()).To(Equal(1))
				})
			})
		})
		When("creating Validator", func() {
			var (
				wg             *sync.WaitGroup
				ctx            context.Context
				validatorTasks jobs.QueueController
				validator      document.Validator
			)
			BeforeEach(func() {
				wg = &sync.WaitGroup{}
				ctx = context.Background()
				httpClient.DoReturns(&http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader([]byte(""))),
				}, nil)
			})
			JustBeforeEach(func() {
				Expect(worker).NotTo(BeNil())
				Expect(err).NotTo(HaveOccurred())
				validator, validatorTasks, err = linkvalidator.New(2, false, wg, httpClient, resHandlers)
			})
			It("succeeded", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(validatorTasks).NotTo(BeNil())
				Expect(validator).NotTo(BeNil())
			})
			When("validate links", func() {
				JustBeforeEach(func() {
					validatorTasks.Start(ctx)
					Expect(validator.ValidateLink(&url.URL{Scheme: "https", Host: "host1", Path: "link1"}, "dest1", "path1")).To(BeTrue())
					Expect(validator.ValidateLink(&url.URL{Scheme: "https", Host: "host2", Path: "link2"}, "dest2", "path2")).To(BeTrue())
				})
				It("validates link successfully", func() {
					wg.Wait()
					Expect(validatorTasks.GetProcessedTasksCount()).To(Equal(2))
					Expect(validatorTasks.GetErrorList()).To(BeNil())
					Expect(httpClient.DoCallCount()).To(Equal(2))
				})
				Context("validator tasks queue stopped", func() {
					JustBeforeEach(func() {
						wg.Wait()
						validatorTasks.Stop()
					})
					It("skips the tasks", func() {
						Expect(validator.ValidateLink(&url.URL{Scheme: "https", Host: "host3", Path: "link3"}, "dest3", "path3")).To(BeFalse())
						Expect(validatorTasks.GetProcessedTasksCount()).To(Equal(2))
					})
				})
			})
		})
	})
})
