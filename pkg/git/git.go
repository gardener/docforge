// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0
package git

import (
	"context"

	gogit "github.com/go-git/go-git/v5"
)

type Git interface {
	PlainOpen(path string) (GitRepository, error)
	PlainCloneContext(ctx context.Context, path string, isBare bool, o *gogit.CloneOptions) (GitRepository, error)
}

type GitRepository interface {
	FetchContext(ctx context.Context, o *gogit.FetchOptions) error
	Worktree() (GitRepositoryWorktree, error)
}

type GitRepositoryWorktree interface {
	Checkout(opts *gogit.CheckoutOptions) error
}

type git struct {
	repository *gogit.Repository
}

func NewGit() Git {
	return &git{}
}

func (g *git) PlainOpen(path string) (GitRepository, error) {
	repo, err := gogit.PlainOpen(path)
	return &git{repository: repo}, err
}

func (g *git) PlainCloneContext(ctx context.Context, path string, isBare bool, o *gogit.CloneOptions) (GitRepository, error) {
	repository, err := gogit.PlainCloneContext(ctx, path, isBare, o)
	return &git{repository: repository}, err
}

func (g *git) FetchContext(ctx context.Context, o *gogit.FetchOptions) error {
	return g.repository.FetchContext(ctx, o)
}

func (g *git) Worktree() (GitRepositoryWorktree, error) {
	return g.repository.Worktree()
}
