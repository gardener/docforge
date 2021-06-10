// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package git_test

import (
	"context"
	"fmt"
	"testing"

	mock_git "github.com/gardener/docforge/pkg/mock/git"
	"github.com/gardener/docforge/pkg/resourcehandlers/git"
	gogit "github.com/go-git/go-git/v5"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestRepository_Prepare(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Repository Suite")
}

var branch = "master"

var _ = Describe("Repository", func() {
	var (
		mockCtrl   *gomock.Controller
		repository *git.Repository

		mockGit      *mock_git.MockGit
		mockRepo     *mock_git.MockGitRepository
		mockWorkTree *mock_git.MockGitRepositoryWorktree
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockGit = mock_git.NewMockGit(mockCtrl)
		mockRepo = mock_git.NewMockGitRepository(mockCtrl)
		mockWorkTree = mock_git.NewMockGitRepositoryWorktree(mockCtrl)

		repository = &git.Repository{
			Git: mockGit,
		}
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Describe("Prepare()", func() {
		Context("checks error when repository prepare was called previously", func() {
			BeforeEach(func() {
				repository.State = git.Prepared
			})
			It("returns nil error, when already prepared", func() {
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

		Context("returns the expected error when preparing the repository ", func() {
			var localPath string
			BeforeEach(func() {
				repository.State = 0
				localPath = "localPath"
				repository.LocalPath = localPath
			})

			It("should not return err when PlainOpen is called", func() {
				mockRepo.EXPECT().FetchContext(context.TODO(), gomock.Any()).Times(1).Return(nil)
				mockRepo.EXPECT().Worktree().Times(1).Return(mockWorkTree, nil)
				mockWorkTree.EXPECT().Checkout(gomock.Any()).Return(nil)
				mockGit.EXPECT().PlainOpen(localPath).Times(1).Return(mockRepo, nil)
				err := repository.Prepare(context.TODO(), branch)
				Expect(err).To(BeNil())
			})

			It("should clone the repository ", func() {
				mockRepo.EXPECT().FetchContext(context.TODO(), gomock.Any()).Times(0)
				mockRepo.EXPECT().Worktree().Times(1).Return(mockWorkTree, nil)
				mockWorkTree.EXPECT().Checkout(gomock.Any()).Return(nil)
				mockGit.EXPECT().PlainOpen(localPath).Times(1).Return(nil, gogit.ErrRepositoryNotExists)
				mockGit.EXPECT().PlainCloneContext(gomock.Any(), localPath, false, gomock.Any()).Times(1).Return(mockRepo, nil)

				err := repository.Prepare(context.TODO(), branch)
				Expect(err).To(BeNil())
			})

			It("should return nil err when PlainOpen is called but repo doesn't exist", func() {
				var expectedErr = fmt.Errorf("notgogit.ErrRepositoryNotExists")
				mockGit.EXPECT().PlainOpen(localPath).Times(1).Return(nil, expectedErr)
				err := repository.Prepare(context.TODO(), branch)
				Expect(err).NotTo(BeNil())
				Expect(err).To(BeIdenticalTo(expectedErr))
			})

		})
	})
})
