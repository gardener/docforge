package git

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

const depth = 1

type State int

const (
	_ State = iota
	Prepared
	Failed
)

type Repository struct {
	Auth          http.AuthMethod
	LocalPath     string
	RemoteURL     string
	State         State
	PreviousError error

	mutex sync.RWMutex
}

func (r *Repository) Prepare(ctx context.Context, branch string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	switch r.State {
	case Failed:
		return r.PreviousError
	case Prepared:
		return nil
	}

	if err := r.prepare(ctx, branch); err != nil {
		r.State = Failed
		r.PreviousError = err
		return err
	}
	r.State = Prepared
	return nil
}

func (r *Repository) prepare(ctx context.Context, branch string) error {
	var fetch = true
	gitRepo, err := git.PlainOpen(r.LocalPath)
	if err != nil {
		if err != git.ErrRepositoryNotExists {
			return err
		}
		if gitRepo, err = git.PlainCloneContext(ctx, r.LocalPath, false, &git.CloneOptions{
			URL:        r.RemoteURL,
			RemoteName: git.DefaultRemoteName,
			Depth:      depth,
			Auth:       r.Auth,
		}); err != nil {
			return fmt.Errorf("failed to prepare repo: %s, %v", r.LocalPath, err)
		}
		fetch = false
	}

	if fetch {
		if err := gitRepo.FetchContext(ctx, &git.FetchOptions{
			Auth:       r.Auth,
			Depth:      depth,
			RemoteName: git.DefaultRemoteName,
		}); err != nil && err != git.NoErrAlreadyUpToDate {
			return fmt.Errorf("failed to fetch repository %s: %v", r.LocalPath, err)
		}
	}

	w, err := gitRepo.Worktree()
	if err != nil {
		return err
	}
	if err := w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewRemoteReferenceName(git.DefaultRemoteName, branch),
	}); err != nil {
		return fmt.Errorf("couldn't checkout branch %s for repository %s: %v", branch, r.LocalPath, err)
	}
	return nil
}
