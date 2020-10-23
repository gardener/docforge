package github

import (
	"encoding/json"
	"sort"
	"strings"

	"k8s.io/klog/v2"

	"github.com/gardener/docforge/pkg/git"
	"github.com/google/go-github/v32/github"
)

func transform(commits []*github.RepositoryCommit) *git.GitInfo {
	gitInfo := &git.GitInfo{}
	nonInternalCommits := []*github.RepositoryCommit{}
	// skip internal commits
	for _, commit := range commits {
		if !isInternalCommit(commit) {
			nonInternalCommits = append(nonInternalCommits, commit)
		}
	}
	if len(commits) == 0 {
		return nil
	}
	sort.Slice(nonInternalCommits, func(i, j int) bool {
		return nonInternalCommits[i].GetCommit().GetCommitter().GetDate().After(nonInternalCommits[j].GetCommit().GetCommitter().GetDate())
	})

	lastModifiedDate := nonInternalCommits[0].GetCommit().GetCommitter().GetDate().Format(git.DateFormat)
	gitInfo.LastModifiedDate = &lastModifiedDate

	publishDate := commits[len(nonInternalCommits)-1].GetCommit().GetCommitter().GetDate().Format(git.DateFormat)
	gitInfo.PublishDate = &publishDate

	if gitInfo.Author = getCommitAuthor(nonInternalCommits[len(nonInternalCommits)-1]); gitInfo.Author == nil {
		klog.Warningf("cannot get commit author")
	}

	if len(nonInternalCommits) > 1 {
		gitInfo.Contributors = []*github.User{}
		registered := []string{}
		for _, commit := range nonInternalCommits {
			var contributor *github.User
			if contributor = getCommitAuthor(commit); contributor == nil {
				continue
			}
			if contributor.GetType() == "User" && contributor.GetEmail() != gitInfo.Author.GetEmail() && !contains(registered, contributor.GetEmail()) {
				gitInfo.Contributors = append(gitInfo.Contributors, contributor)
				registered = append(registered, contributor.GetEmail())
			}
		}
	}

	return gitInfo
}

func contains(slice []string, s string) bool {
	for _, _s := range slice {
		if s == _s {
			return true
		}
	}
	return false
}

func marshallGitInfo(gitInfo *git.GitInfo) ([]byte, error) {
	var (
		blob []byte
		err  error
	)
	if blob, err = json.MarshalIndent(gitInfo, "", "  "); err != nil {
		return nil, err
	}
	return blob, nil
}

func isInternalCommit(commit *github.RepositoryCommit) bool {
	message := commit.GetCommit().GetMessage()
	email := commit.GetCommitter().GetEmail()
	return strings.HasPrefix(message, "[int]") ||
		strings.Contains(message, "[skip ci]") ||
		strings.HasPrefix(email, "gardener.ci") ||
		strings.HasPrefix(email, "gardener.opensource")
}

func mergeAuthors(author *github.User, commitAuthor *github.CommitAuthor) *github.User {
	if author == nil {
		author = &github.User{}
	}
	if commitAuthor != nil {
		author.Name = commitAuthor.Name
		author.Email = commitAuthor.Email
	}
	return author
}

func getCommitAuthor(commit *github.RepositoryCommit) *github.User {
	var contributor *github.User
	if contributor = commit.GetAuthor(); contributor != nil && commit.GetCommit().GetAuthor() != nil {
		contributor = mergeAuthors(contributor, commit.GetCommit().GetAuthor())
	}
	if contributor == nil && commit.GetCommit().GetAuthor() != nil {
		contributor = mergeAuthors(&github.User{}, commit.GetCommit().GetAuthor())
	}
	if contributor == nil && commit.GetCommit().GetCommitter() != nil {
		contributor = mergeAuthors(&github.User{}, commit.GetCommit().GetCommitter())
	}
	return contributor
}
