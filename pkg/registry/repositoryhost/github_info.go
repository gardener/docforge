package repositoryhost

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/google/go-github/v43/github"
	"k8s.io/klog/v2"
)

const (
	// DateFormat defines format for LastModifiedDate & PublishDate
	DateFormat = "2006-01-02 15:04:05"
)

// GitInfo defines git resource attributes
type GitInfo struct {
	LastModifiedDate *string        `json:"lastmod,omitempty"`
	PublishDate      *string        `json:"publishdate,omitempty"`
	Author           *github.User   `json:"author,omitempty"`
	Contributors     []*github.User `json:"contributors,omitempty"`
	WebURL           *string        `json:"weburl,omitempty"`
	SHA              *string        `json:"sha,omitempty"`
	SHAAlias         *string        `json:"shaalias,omitempty"`
	Path             *string        `json:"path,omitempty"`
}

// ReadGitInfo reads the git info for a given resource URL
func ReadGitInfo(ctx context.Context, repositories Repositories, r URL) ([]byte, error) {
	opts := &github.CommitsListOptions{
		Path: r.GetResourcePath(),
		SHA:  r.GetRef(),
	}
	commits, resp, err := repositories.ListCommits(ctx, r.GetOwner(), r.GetRepo(), opts)
	if err != nil {
		return nil, err
	}
	if resp != nil && resp.StatusCode >= 400 {
		return nil, fmt.Errorf("list commits for %s fails with HTTP status: %d", r.String(), resp.StatusCode)
	}
	gitInfo := transform(commits)
	if gitInfo == nil {
		return nil, nil
	}
	ref := r.GetRef()
	if len(ref) > 0 {
		gitInfo.SHAAlias = &ref
	}
	resourcePath := r.GetResourcePath()
	if len(resourcePath) > 0 {
		gitInfo.Path = &resourcePath
	}
	return json.MarshalIndent(gitInfo, "", "  ")
}

// transform builds git.Info from a commits list
func transform(commits []*github.RepositoryCommit) *GitInfo {
	if commits == nil {
		return nil
	}
	gitInfo := &GitInfo{}
	// skip internal commits
	nonInternalCommits := slices.DeleteFunc(commits, isInternalCommit)
	if len(nonInternalCommits) == 0 {
		return nil
	}
	sort.Slice(nonInternalCommits, func(i, j int) bool {
		return nonInternalCommits[i].GetCommit().GetCommitter().GetDate().After(nonInternalCommits[j].GetCommit().GetCommitter().GetDate())
	})
	lastModifiedDate := nonInternalCommits[0].GetCommit().GetCommitter().GetDate().Format(DateFormat)
	gitInfo.LastModifiedDate = &lastModifiedDate

	webURL := nonInternalCommits[0].GetHTMLURL()
	gitInfo.WebURL = github.String(strings.Split(webURL, "/commit/")[0])

	gitInfo.PublishDate = github.String(nonInternalCommits[len(nonInternalCommits)-1].GetCommit().GetCommitter().GetDate().Format(DateFormat))

	if gitInfo.Author = getCommitAuthor(nonInternalCommits[len(nonInternalCommits)-1]); gitInfo.Author == nil {
		klog.Warningf("cannot get commit author")
	}
	if len(nonInternalCommits) < 2 {
		return gitInfo
	}
	gitInfo.Contributors = []*github.User{}
	var registered []string
	for _, commit := range nonInternalCommits {
		var contributor *github.User
		if contributor = getCommitAuthor(commit); contributor == nil {
			continue
		}
		if contributor.GetType() == "User" && contributor.GetEmail() != gitInfo.Author.GetEmail() && slices.Index(registered, contributor.GetEmail()) < 0 {
			gitInfo.Contributors = append(gitInfo.Contributors, contributor)
			registered = append(registered, contributor.GetEmail())
		}
	}
	return gitInfo
}

func isInternalCommit(commit *github.RepositoryCommit) bool {
	message := commit.GetCommit().GetMessage()
	email := commit.GetCommitter().GetEmail()
	return strings.HasPrefix(message, "[int]") ||
		strings.Contains(message, "[skip ci]") ||
		strings.HasPrefix(email, "gardener.ci") ||
		strings.HasPrefix(email, "gardener.opensource")
}

func getCommitAuthor(commit *github.RepositoryCommit) *github.User {
	getCommitAuthor := commit.GetCommit().GetAuthor()
	getCommitCommiter := commit.GetCommit().GetCommitter()
	contributor := commit.GetAuthor()
	if contributor != nil && getCommitAuthor != nil {
		contributor.Name = getCommitAuthor.Name
		contributor.Email = getCommitAuthor.Email
		return contributor
	}
	if getCommitAuthor != nil {
		return &github.User{Name: getCommitAuthor.Name, Email: getCommitAuthor.Email}
	}
	if getCommitCommiter != nil {
		return &github.User{Name: getCommitCommiter.Name, Email: getCommitCommiter.Email}
	}
	return nil
}
