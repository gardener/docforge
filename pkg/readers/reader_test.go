// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package readers_test

import (
	"context"

	"github.com/gardener/docforge/pkg/readers"
	"github.com/gardener/docforge/pkg/readers/repositoryhosts/repositoryhostsfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Reader", func() {
	var (
		err     error
		content []byte
		ctx     context.Context
		reader  *readers.GenericReader
	)
	BeforeEach(func() {
		ctx = context.Background()
		reader = &readers.GenericReader{}
	})
	When("resource handlers registry not set", func() {
		It("panics invoking Reader.Read", func() {
			Expect(func() { content, err = reader.Read(ctx, "https://fake_source") }).To(Panic())
		})
	})
	When("invokes Reader.Read", func() {
		var (
			resHandlers *repositoryhostsfakes.FakeRegistry
			resHandler  *repositoryhostsfakes.FakeRepositoryHost
		)
		BeforeEach(func() {
			resHandler = &repositoryhostsfakes.FakeRepositoryHost{}
			resHandlers = &repositoryhostsfakes.FakeRegistry{}
			reader.RepositoryHosts = resHandlers
			resHandler.ReadReturns([]byte("content"), nil)
			resHandler.ReadGitInfoReturns([]byte("git_info"), nil)
			resHandlers.GetReturns(resHandler)
		})
		JustBeforeEach(func() {
			content, err = reader.Read(ctx, "https://fake_source")
		})
		It("succeeded", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(Equal("content"))
			Expect(resHandlers.GetCallCount()).To(Equal(1))
			Expect(resHandler.ReadCallCount()).To(Equal(1))
			Expect(resHandler.ReadGitInfoCallCount()).To(Equal(0))
		})
		Context("no suitable handler found", func() {
			BeforeEach(func() {
				resHandlers.GetReturns(nil)
			})
			It("fails", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("https://fake_source"))
				Expect(content).To(BeNil())
			})
		})
		Context("GitHub info flag is set", func() {
			BeforeEach(func() {
				reader.IsGitHubInfo = true
			})
			It("succeeded", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(Equal("git_info"))
				Expect(resHandlers.GetCallCount()).To(Equal(1))
				Expect(resHandler.ReadCallCount()).To(Equal(0))
				Expect(resHandler.ReadGitInfoCallCount()).To(Equal(1))
			})
		})
	})
})
