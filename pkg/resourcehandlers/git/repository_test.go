// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package git_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/gardener/docforge/pkg/resourcehandlers/git"
	fakes "github.com/gardener/docforge/pkg/resourcehandlers/git/gitinterface/gitinterfacefakes"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestRepository_Prepare(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Repository Suite")
}

var _ = Describe("Repository", func() {
	var (
		repository *git.Repository
		branch     string

		fakeGit      fakes.FakeGit
		fakeRepo     fakes.FakeRepository
		fakeWorktree fakes.FakeRepositoryWorktree
	)

	BeforeEach(func() {
		branch = "master"

		fakeGit = fakes.FakeGit{}
		fakeRepo = fakes.FakeRepository{}
		fakeWorktree = fakes.FakeRepositoryWorktree{}

		repository = &git.Repository{
			Git: &fakeGit,
		}
	})

	Describe("Prepare()", func() {
		Context("prepare was called previously", func() {
			BeforeEach(func() {
				repository.State = git.Prepared
			})
			It("does not return error when already prepared", func() {
				Expect(repository.Prepare(context.TODO(), branch)).To(BeNil())
			})

			It("returns the expected error when failed from previous calls", func() {
				var expectedErr = fmt.Errorf("some errs")
				repository.State = git.Failed
				repository.PreviousError = expectedErr

				err := repository.Prepare(context.TODO(), branch)
				Expect(err).NotTo(BeNil())
				Expect(err).To(BeIdenticalTo(expectedErr))
			})
		})

		Context("prepare wasn't called previously ", func() {

			BeforeEach(func() {
				repository.State = 0
				repository.LocalPath = "localPath"

				fakeRepo.FetchContextReturns(nil)
				fakeRepo.TagsReturns([]string{}, nil)
				fakeRepo.ReferenceReturnsOnCall(0, plumbing.NewReferenceFromStrings("refs/remotes/origin/"+branch, ""), nil)
				fakeRepo.ReferenceReturnsOnCall(1, nil, fmt.Errorf("some err"))
				fakeRepo.WorktreeReturns(&fakeWorktree, nil)

				fakeWorktree.CheckoutReturns(nil)
			})

			Context("repository does not exist", func() {
				var expectedErr = fmt.Errorf("notgogit.ErrRepositoryNotExists")

				BeforeEach(func() {
					fakeGit.PlainOpenReturns(nil, expectedErr)
				})

				It("should return error", func() {
					err := repository.Prepare(context.TODO(), branch)
					Expect(err).NotTo(BeNil())
					Expect(err).To(BeIdenticalTo(expectedErr))

					Expect(fakeGit.PlainOpenCallCount()).Should(Equal(1))
				})
			})

			Context("repository exists but does not locally", func() {
				BeforeEach(func() {
					fakeGit.PlainOpenReturns(nil, gogit.ErrRepositoryNotExists)
					fakeGit.PlainCloneContextReturns(&fakeRepo, nil)
				})

				It("should clone the repository ", func() {
					err := repository.Prepare(context.TODO(), branch)
					Expect(err).To(BeNil())

					Expect(fakeRepo.FetchContextCallCount()).Should(Equal(0))
					Expect(fakeRepo.WorktreeCallCount()).Should(Equal(1))
					Expect(fakeRepo.TagsCallCount()).Should(Equal(0))
					Expect(fakeRepo.ReferenceCallCount()).Should(Equal(2))

					Expect(fakeGit.PlainOpenCallCount()).Should(Equal(1))
					Expect(fakeGit.PlainCloneContextCallCount()).Should(Equal(1))

				})

			})

			Context("repository does exist", func() {
				BeforeEach(func() {
					fakeGit.PlainOpenReturns(&fakeRepo, nil)
				})

				It("should not return any errors ", func() {
					err := repository.Prepare(context.TODO(), branch)
					Expect(err).To(BeNil())

					Expect(fakeGit.PlainOpenCallCount()).Should(Equal(1))
					Expect(fakeGit.PlainCloneContextCallCount()).Should(Equal(0))
					Expect(fakeRepo.FetchContextCallCount()).Should(Equal(1))
					Expect(fakeRepo.WorktreeCallCount()).Should(Equal(1))
					Expect(fakeRepo.TagsCallCount()).Should(Equal(0))
					Expect(fakeRepo.ReferenceCallCount()).Should(Equal(2))
				})

			})

		})
	})
})
