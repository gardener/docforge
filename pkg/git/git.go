// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package git

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import (
	"context"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// Git interface defines gogit git API
//counterfeiter:generate . Git
type Git interface {
	PlainOpen(path string) (Repository, error)
	PlainCloneContext(ctx context.Context, path string, isBare bool, o *gogit.CloneOptions) (Repository, error)
}

// Repository interface defines gogit repository API
//counterfeiter:generate . Repository
type Repository interface {
	FetchContext(ctx context.Context, o *gogit.FetchOptions) error
	Worktree() (RepositoryWorktree, error)
	Reference(name plumbing.ReferenceName, resolved bool) (*plumbing.Reference, error)
	Tags() ([]string, error)
}

// RepositoryWorktree interface defines gogit worktree API
//counterfeiter:generate . RepositoryWorktree
type RepositoryWorktree interface {
	Checkout(opts *gogit.CheckoutOptions) error
}

type git struct {
	repository *gogit.Repository
}

// NewGit creates new git struct
func NewGit() Git {
	return &git{}
}

// PlainOpen calls git repository API PlainOpen
func (g *git) PlainOpen(path string) (Repository, error) {
	repo, err := gogit.PlainOpen(path)
	return &git{repository: repo}, err
}

// PlainCloneContext calls git repository API PlainCloneContext
func (g *git) PlainCloneContext(ctx context.Context, path string, isBare bool, o *gogit.CloneOptions) (Repository, error) {
	repository, err := gogit.PlainCloneContext(ctx, path, isBare, o)
	return &git{repository: repository}, err
}

// FetchContext calls git repository API FetchContext
func (g *git) FetchContext(ctx context.Context, o *gogit.FetchOptions) error {
	return g.repository.FetchContext(ctx, o)
}

// Worktree calls git repository API Worktree
func (g *git) Worktree() (RepositoryWorktree, error) {
	return g.repository.Worktree()
}

// Reference translates a reference name to a reference structure with a parameter resolved
func (g *git) Reference(name plumbing.ReferenceName, resolved bool) (*plumbing.Reference, error) {
	return g.repository.Reference(name, resolved)
}

// Tags gets the tags from the corresponding repository
func (g *git) Tags() ([]string, error) {
	iter, err := g.repository.Tags()
	if err != nil {
		return nil, err
	}
	var tags []string
	if err = iter.ForEach(func(ref *plumbing.Reference) error {
		tags = append(tags, ref.Name().Short())
		return nil
	}); err != nil {
		return nil, err
	}
	return tags, nil
}
