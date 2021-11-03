// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package git

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/git"
	ghub "github.com/gardener/docforge/pkg/resourcehandlers/github"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/google/go-github/v32/github"
	"github.com/stretchr/testify/assert"
)

const (
	// baseURLPath is a non-empty Client.BaseURL path to use during tests,
	// to ensure relative URLs are used for all endpoints. See issue #752.
	baseURLPath = "/api-v3"
)

// setup sets up a test HTTP server along with a github.Client that is
// configured to talk to that test server. Tests should register handlers on
// mux which provide mock responses for the API method being tested.
func setup() (client *github.Client, mux *http.ServeMux, serverURL string, teardown func()) {
	// mux is the HTTP request multiplexer used with the test server.
	mux = http.NewServeMux()

	// We want to ensure that tests catch mistakes where the endpoint URL is
	// specified as absolute rather than relative. It only makes a difference
	// when there's a non-empty base URL path. So, use that. See issue #752.
	apiHandler := http.NewServeMux()
	apiHandler.Handle(baseURLPath+"/", http.StripPrefix(baseURLPath, mux))
	apiHandler.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		fmt.Fprintln(os.Stderr, "FAIL: Client.BaseURL path prefix is not preserved in the request URL:")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "\t"+req.URL.String())
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "\tDid you accidentally use an absolute endpoint URL rather than relative?")
		fmt.Fprintln(os.Stderr, "\tSee https://github.com/google/go-github/issues/752 for information.")
		http.Error(w, "Client.BaseURL path prefix is not preserved in the request URL.", http.StatusInternalServerError)
	})

	// server is a test HTTP server used to provide mock API responses.
	server := httptest.NewServer(apiHandler)

	// client is the GitHub client being tested and is
	// configured to use test server.
	client = github.NewClient(nil)
	url, _ := url.Parse(server.URL + baseURLPath + "/")
	client.BaseURL = url
	client.UploadURL = url

	return client, mux, server.URL, server.Close
}

type mockFileSystem struct {
	mockFileContent map[string]([]byte)
	mockFileInfo    map[string]os.FileInfo
}

type mockGit struct {
	repoTags map[string]([]string)
}

type mockFileInfo struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
	isDir   bool
	sys     interface{}
}

func (mfi mockFileInfo) Name() string {
	return mfi.name
}
func (mfi mockFileInfo) Size() int64 {
	return mfi.size
}
func (mfi mockFileInfo) Mode() fs.FileMode {
	return mfi.mode
}
func (mfi mockFileInfo) ModTime() time.Time {
	return mfi.modTime
}
func (mfi mockFileInfo) IsDir() bool {
	return mfi.isDir
}
func (mfi mockFileInfo) Sys() interface{} {
	return mfi.sys
}

func (mfs *mockFileSystem) ReadFile(name string) ([]byte, error) {
	data, ok := mfs.mockFileContent[name]
	if !ok {
		return nil, fmt.Errorf("File does not exist")
	}
	return data, nil
}

func (mfs *mockFileSystem) Stat(name string) (os.FileInfo, error) {
	data, ok := mfs.mockFileInfo[name]
	if !ok {
		return nil, fmt.Errorf("File does not exist")
	}
	return data, nil
}

//check after stat
func (mfs *mockFileSystem) IsNotExist(err error) bool {
	return err.Error() == "File does not exist"
}

type mockGitRepositoryWorktree struct{}
type mockGitRepository struct {
	tags []string
}

func (g *mockGitRepositoryWorktree) Checkout(opts *gogit.CheckoutOptions) error {
	return nil
}
func (g *mockGitRepository) FetchContext(ctx context.Context, o *gogit.FetchOptions) error {
	return nil
}

func (g *mockGitRepository) Worktree() (git.RepositoryWorktree, error) {
	return &mockGitRepositoryWorktree{}, nil
}

// Reference translates a reference name to a reference structure with a parameter resolved
func (g *mockGitRepository) Reference(name plumbing.ReferenceName, resolved bool) (*plumbing.Reference, error) {
	return &plumbing.Reference{}, nil
}

// Tags gets the tags from the corresponding repository
func (g *mockGitRepository) Tags() ([]string, error) {
	return g.tags, nil
}

func (g *mockGit) PlainOpen(path string) (git.Repository, error) {
	return &mockGitRepository{g.repoTags[path]}, nil
}

func (g *mockGit) PlainCloneContext(ctx context.Context, path string, isBare bool, o *gogit.CloneOptions) (git.Repository, error) {
	return &mockGitRepository{}, nil
}

func TestResolveDocumentation(t *testing.T) {

	cases := []struct {
		repositoryPath string
		manifestName   string
		tags           []string
		manifestData   []byte
		uri            string
		mux            func(mux *http.ServeMux)
		want           *api.Documentation
		err            error
	}{
		{
			"github.com/testOrg/testRepo/testMainBranch",
			"testManifest.yaml",
			[]string{"v4.9", "v5.7", "v6.1", "v7.7"},
			[]byte(fmt.Sprintf(`structure:
- name: community
  source: https://github.com/gardener/docforge/edit/master/integration-test/tested-doc/merge-test/testFile.md
{{- $vers := Split .versions "," -}}
{{- range $i, $version := $vers -}}
{{- if eq $i 0  }}
- name: docs
{{- else }}
- name: {{$version}}
{{- end }}
  source: https://github.com/gardener/docforge/blob/{{$version}}/integration-test/tested-doc/merge-test/testFile.md
{{- end }}`)),
			"https://github.com/testOrg/testRepo/blob/DEFAULT_BRANCH/testManifest.yaml",
			func(mux *http.ServeMux) {
				mux.HandleFunc("/repos/testOrg/testRepo/git/trees/testMainBranch", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte(fmt.Sprintf(`
					{
						"sha": "9fb037999f264ba9a7fc6274d15fa3ae2ab98312",
						"url": "https://api.github.com/repos/testOrg/testRepo/git/trees/testMainBranch",
						"tree": [
						  {
							"path": "testManifest.yaml",
							"mode": "100644",
							"type": "blob",
							"size": 30,
							"sha": "testSha",
							"url": "https://api.github.com/repos/testOrg/testRepo/git/trees/testMainBranch/testManifest.yaml"
						  }
						],
						"truncated": false
					  }
					`)))
				})
				mux.HandleFunc("/repos/testOrg/testRepo", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte(fmt.Sprintf(`
						{
							"id": 1296269,
							"node_id": "MDEwOlJlcG9zaXRvcnkxMjk2MjY5",
							"name": "Hello-World",
							"full_name": "octocat/Hello-World",
							"owner": {
							  "login": "octocat",
							  "id": 1,
							  "node_id": "MDQ6VXNlcjE=",
							  "avatar_url": "https://github.com/images/error/octocat_happy.gif",
							  "gravatar_id": "",
							  "url": "https://api.github.com/users/octocat",
							  "html_url": "https://github.com/octocat",
							  "followers_url": "https://api.github.com/users/octocat/followers",
							  "following_url": "https://api.github.com/users/octocat/following{/other_user}",
							  "gists_url": "https://api.github.com/users/octocat/gists{/gist_id}",
							  "starred_url": "https://api.github.com/users/octocat/starred{/owner}{/repo}",
							  "subscriptions_url": "https://api.github.com/users/octocat/subscriptions",
							  "organizations_url": "https://api.github.com/users/octocat/orgs",
							  "repos_url": "https://api.github.com/users/octocat/repos",
							  "events_url": "https://api.github.com/users/octocat/events{/privacy}",
							  "received_events_url": "https://api.github.com/users/octocat/received_events",
							  "type": "User",
							  "site_admin": false
							},
							"private": false,
							"html_url": "https://github.com/octocat/Hello-World",
							"description": "This your first repo!",
							"fork": false,
							"url": "https://api.github.com/repos/octocat/Hello-World",
							"archive_url": "https://api.github.com/repos/octocat/Hello-World/{archive_format}{/ref}",
							"assignees_url": "https://api.github.com/repos/octocat/Hello-World/assignees{/user}",
							"blobs_url": "https://api.github.com/repos/octocat/Hello-World/git/blobs{/sha}",
							"branches_url": "https://api.github.com/repos/octocat/Hello-World/branches{/branch}",
							"collaborators_url": "https://api.github.com/repos/octocat/Hello-World/collaborators{/collaborator}",
							"comments_url": "https://api.github.com/repos/octocat/Hello-World/comments{/number}",
							"commits_url": "https://api.github.com/repos/octocat/Hello-World/commits{/sha}",
							"compare_url": "https://api.github.com/repos/octocat/Hello-World/compare/{base}...{head}",
							"contents_url": "https://api.github.com/repos/octocat/Hello-World/contents/{+path}",
							"contributors_url": "https://api.github.com/repos/octocat/Hello-World/contributors",
							"deployments_url": "https://api.github.com/repos/octocat/Hello-World/deployments",
							"downloads_url": "https://api.github.com/repos/octocat/Hello-World/downloads",
							"events_url": "https://api.github.com/repos/octocat/Hello-World/events",
							"forks_url": "https://api.github.com/repos/octocat/Hello-World/forks",
							"git_commits_url": "https://api.github.com/repos/octocat/Hello-World/git/commits{/sha}",
							"git_refs_url": "https://api.github.com/repos/octocat/Hello-World/git/refs{/sha}",
							"git_tags_url": "https://api.github.com/repos/octocat/Hello-World/git/tags{/sha}",
							"git_url": "git:github.com/octocat/Hello-World.git",
							"issue_comment_url": "https://api.github.com/repos/octocat/Hello-World/issues/comments{/number}",
							"issue_events_url": "https://api.github.com/repos/octocat/Hello-World/issues/events{/number}",
							"issues_url": "https://api.github.com/repos/octocat/Hello-World/issues{/number}",
							"keys_url": "https://api.github.com/repos/octocat/Hello-World/keys{/key_id}",
							"labels_url": "https://api.github.com/repos/octocat/Hello-World/labels{/name}",
							"languages_url": "https://api.github.com/repos/octocat/Hello-World/languages",
							"merges_url": "https://api.github.com/repos/octocat/Hello-World/merges",
							"milestones_url": "https://api.github.com/repos/octocat/Hello-World/milestones{/number}",
							"notifications_url": "https://api.github.com/repos/octocat/Hello-World/notifications{?since,all,participating}",
							"pulls_url": "https://api.github.com/repos/octocat/Hello-World/pulls{/number}",
							"releases_url": "https://api.github.com/repos/octocat/Hello-World/releases{/id}",
							"ssh_url": "git@github.com:octocat/Hello-World.git",
							"stargazers_url": "https://api.github.com/repos/octocat/Hello-World/stargazers",
							"statuses_url": "https://api.github.com/repos/octocat/Hello-World/statuses/{sha}",
							"subscribers_url": "https://api.github.com/repos/octocat/Hello-World/subscribers",
							"subscription_url": "https://api.github.com/repos/octocat/Hello-World/subscription",
							"tags_url": "https://api.github.com/repos/octocat/Hello-World/tags",
							"teams_url": "https://api.github.com/repos/octocat/Hello-World/teams",
							"trees_url": "https://api.github.com/repos/octocat/Hello-World/git/trees{/sha}",
							"clone_url": "https://github.com/octocat/Hello-World.git",
							"mirror_url": "git:git.example.com/octocat/Hello-World",
							"hooks_url": "https://api.github.com/repos/octocat/Hello-World/hooks",
							"svn_url": "https://svn.github.com/octocat/Hello-World",
							"homepage": "https://github.com",
							"language": null,
							"forks_count": 9,
							"forks": 9,
							"stargazers_count": 80,
							"watchers_count": 80,
							"watchers": 80,
							"size": 108,
							"default_branch": "testMainBranch",
							"open_issues_count": 0,
							"open_issues": 0,
							"is_template": false,
							"topics": [
							  "octocat",
							  "atom",
							  "electron",
							  "api"
							],
							"has_issues": true,
							"has_projects": true,
							"has_wiki": true,
							"has_pages": false,
							"has_downloads": true,
							"archived": false,
							"disabled": false,
							"visibility": "public",
							"pushed_at": "2011-01-26T19:06:43Z",
							"created_at": "2011-01-26T19:01:12Z",
							"updated_at": "2011-01-26T19:14:43Z",
							"permissions": {
							  "pull": true,
							  "push": false,
							  "admin": false
							},
							"allow_rebase_merge": true,
							"template_repository": {
							  "id": 1296269,
							  "node_id": "MDEwOlJlcG9zaXRvcnkxMjk2MjY5",
							  "name": "Hello-World-Template",
							  "full_name": "octocat/Hello-World-Template",
							  "owner": {
								"login": "octocat",
								"id": 1,
								"node_id": "MDQ6VXNlcjE=",
								"avatar_url": "https://github.com/images/error/octocat_happy.gif",
								"gravatar_id": "",
								"url": "https://api.github.com/users/octocat",
								"html_url": "https://github.com/octocat",
								"followers_url": "https://api.github.com/users/octocat/followers",
								"following_url": "https://api.github.com/users/octocat/following{/other_user}",
								"gists_url": "https://api.github.com/users/octocat/gists{/gist_id}",
								"starred_url": "https://api.github.com/users/octocat/starred{/owner}{/repo}",
								"subscriptions_url": "https://api.github.com/users/octocat/subscriptions",
								"organizations_url": "https://api.github.com/users/octocat/orgs",
								"repos_url": "https://api.github.com/users/octocat/repos",
								"events_url": "https://api.github.com/users/octocat/events{/privacy}",
								"received_events_url": "https://api.github.com/users/octocat/received_events",
								"type": "User",
								"site_admin": false
							  },
							  "private": false,
							  "html_url": "https://github.com/octocat/Hello-World-Template",
							  "description": "This your first repo!",
							  "fork": false,
							  "url": "https://api.github.com/repos/octocat/Hello-World-Template",
							  "archive_url": "https://api.github.com/repos/octocat/Hello-World-Template/{archive_format}{/ref}",
							  "assignees_url": "https://api.github.com/repos/octocat/Hello-World-Template/assignees{/user}",
							  "blobs_url": "https://api.github.com/repos/octocat/Hello-World-Template/git/blobs{/sha}",
							  "branches_url": "https://api.github.com/repos/octocat/Hello-World-Template/branches{/branch}",
							  "collaborators_url": "https://api.github.com/repos/octocat/Hello-World-Template/collaborators{/collaborator}",
							  "comments_url": "https://api.github.com/repos/octocat/Hello-World-Template/comments{/number}",
							  "commits_url": "https://api.github.com/repos/octocat/Hello-World-Template/commits{/sha}",
							  "compare_url": "https://api.github.com/repos/octocat/Hello-World-Template/compare/{base}...{head}",
							  "contents_url": "https://api.github.com/repos/octocat/Hello-World-Template/contents/{+path}",
							  "contributors_url": "https://api.github.com/repos/octocat/Hello-World-Template/contributors",
							  "deployments_url": "https://api.github.com/repos/octocat/Hello-World-Template/deployments",
							  "downloads_url": "https://api.github.com/repos/octocat/Hello-World-Template/downloads",
							  "events_url": "https://api.github.com/repos/octocat/Hello-World-Template/events",
							  "forks_url": "https://api.github.com/repos/octocat/Hello-World-Template/forks",
							  "git_commits_url": "https://api.github.com/repos/octocat/Hello-World-Template/git/commits{/sha}",
							  "git_refs_url": "https://api.github.com/repos/octocat/Hello-World-Template/git/refs{/sha}",
							  "git_tags_url": "https://api.github.com/repos/octocat/Hello-World-Template/git/tags{/sha}",
							  "git_url": "git:github.com/octocat/Hello-World-Template.git",
							  "issue_comment_url": "https://api.github.com/repos/octocat/Hello-World-Template/issues/comments{/number}",
							  "issue_events_url": "https://api.github.com/repos/octocat/Hello-World-Template/issues/events{/number}",
							  "issues_url": "https://api.github.com/repos/octocat/Hello-World-Template/issues{/number}",
							  "keys_url": "https://api.github.com/repos/octocat/Hello-World-Template/keys{/key_id}",
							  "labels_url": "https://api.github.com/repos/octocat/Hello-World-Template/labels{/name}",
							  "languages_url": "https://api.github.com/repos/octocat/Hello-World-Template/languages",
							  "merges_url": "https://api.github.com/repos/octocat/Hello-World-Template/merges",
							  "milestones_url": "https://api.github.com/repos/octocat/Hello-World-Template/milestones{/number}",
							  "notifications_url": "https://api.github.com/repos/octocat/Hello-World-Template/notifications{?since,all,participating}",
							  "pulls_url": "https://api.github.com/repos/octocat/Hello-World-Template/pulls{/number}",
							  "releases_url": "https://api.github.com/repos/octocat/Hello-World-Template/releases{/id}",
							  "ssh_url": "git@github.com:octocat/Hello-World-Template.git",
							  "stargazers_url": "https://api.github.com/repos/octocat/Hello-World-Template/stargazers",
							  "statuses_url": "https://api.github.com/repos/octocat/Hello-World-Template/statuses/{sha}",
							  "subscribers_url": "https://api.github.com/repos/octocat/Hello-World-Template/subscribers",
							  "subscription_url": "https://api.github.com/repos/octocat/Hello-World-Template/subscription",
							  "tags_url": "https://api.github.com/repos/octocat/Hello-World-Template/tags",
							  "teams_url": "https://api.github.com/repos/octocat/Hello-World-Template/teams",
							  "trees_url": "https://api.github.com/repos/octocat/Hello-World-Template/git/trees{/sha}",
							  "clone_url": "https://github.com/octocat/Hello-World-Template.git",
							  "mirror_url": "git:git.example.com/octocat/Hello-World-Template",
							  "hooks_url": "https://api.github.com/repos/octocat/Hello-World-Template/hooks",
							  "svn_url": "https://svn.github.com/octocat/Hello-World-Template",
							  "homepage": "https://github.com",
							  "language": null,
							  "forks": 9,
							  "forks_count": 9,
							  "stargazers_count": 80,
							  "watchers_count": 80,
							  "watchers": 80,
							  "size": 108,
							  "default_branch": "master",
							  "open_issues": 0,
							  "open_issues_count": 0,
							  "is_template": true,
							  "license": {
								"key": "mit",
								"name": "MIT License",
								"url": "https://api.github.com/licenses/mit",
								"spdx_id": "MIT",
								"node_id": "MDc6TGljZW5zZW1pdA==",
								"html_url": "https://api.github.com/licenses/mit"
							  },
							  "topics": [
								"octocat",
								"atom",
								"electron",
								"api"
							  ],
							  "has_issues": true,
							  "has_projects": true,
							  "has_wiki": true,
							  "has_pages": false,
							  "has_downloads": true,
							  "archived": false,
							  "disabled": false,
							  "visibility": "public",
							  "pushed_at": "2011-01-26T19:06:43Z",
							  "created_at": "2011-01-26T19:01:12Z",
							  "updated_at": "2011-01-26T19:14:43Z",
							  "permissions": {
								"admin": false,
								"push": false,
								"pull": true
							  },
							  "allow_rebase_merge": true,
							  "temp_clone_token": "dummy",
							  "allow_squash_merge": true,
							  "allow_auto_merge": false,
							  "delete_branch_on_merge": true,
							  "allow_merge_commit": true,
							  "subscribers_count": 42,
							  "network_count": 0
							},
							"temp_clone_token": "dummy",
							"allow_squash_merge": true,
							"allow_auto_merge": false,
							"delete_branch_on_merge": true,
							"allow_merge_commit": true,
							"subscribers_count": 42,
							"network_count": 0,
							"license": {
							  "key": "mit",
							  "name": "MIT License",
							  "spdx_id": "MIT",
							  "url": "https://api.github.com/licenses/mit",
							  "node_id": "MDc6TGljZW5zZW1pdA=="
							},
							"organization": {
							  "login": "octocat",
							  "id": 1,
							  "node_id": "MDQ6VXNlcjE=",
							  "avatar_url": "https://github.com/images/error/octocat_happy.gif",
							  "gravatar_id": "",
							  "url": "https://api.github.com/users/octocat",
							  "html_url": "https://github.com/octocat",
							  "followers_url": "https://api.github.com/users/octocat/followers",
							  "following_url": "https://api.github.com/users/octocat/following{/other_user}",
							  "gists_url": "https://api.github.com/users/octocat/gists{/gist_id}",
							  "starred_url": "https://api.github.com/users/octocat/starred{/owner}{/repo}",
							  "subscriptions_url": "https://api.github.com/users/octocat/subscriptions",
							  "organizations_url": "https://api.github.com/users/octocat/orgs",
							  "repos_url": "https://api.github.com/users/octocat/repos",
							  "events_url": "https://api.github.com/users/octocat/events{/privacy}",
							  "received_events_url": "https://api.github.com/users/octocat/received_events",
							  "type": "Organization",
							  "site_admin": false
							},
							"parent": {
							  "id": 1296269,
							  "node_id": "MDEwOlJlcG9zaXRvcnkxMjk2MjY5",
							  "name": "Hello-World",
							  "full_name": "octocat/Hello-World",
							  "owner": {
								"login": "octocat",
								"id": 1,
								"node_id": "MDQ6VXNlcjE=",
								"avatar_url": "https://github.com/images/error/octocat_happy.gif",
								"gravatar_id": "",
								"url": "https://api.github.com/users/octocat",
								"html_url": "https://github.com/octocat",
								"followers_url": "https://api.github.com/users/octocat/followers",
								"following_url": "https://api.github.com/users/octocat/following{/other_user}",
								"gists_url": "https://api.github.com/users/octocat/gists{/gist_id}",
								"starred_url": "https://api.github.com/users/octocat/starred{/owner}{/repo}",
								"subscriptions_url": "https://api.github.com/users/octocat/subscriptions",
								"organizations_url": "https://api.github.com/users/octocat/orgs",
								"repos_url": "https://api.github.com/users/octocat/repos",
								"events_url": "https://api.github.com/users/octocat/events{/privacy}",
								"received_events_url": "https://api.github.com/users/octocat/received_events",
								"type": "User",
								"site_admin": false
							  },
							  "private": false,
							  "html_url": "https://github.com/octocat/Hello-World",
							  "description": "This your first repo!",
							  "fork": false,
							  "url": "https://api.github.com/repos/octocat/Hello-World",
							  "archive_url": "https://api.github.com/repos/octocat/Hello-World/{archive_format}{/ref}",
							  "assignees_url": "https://api.github.com/repos/octocat/Hello-World/assignees{/user}",
							  "blobs_url": "https://api.github.com/repos/octocat/Hello-World/git/blobs{/sha}",
							  "branches_url": "https://api.github.com/repos/octocat/Hello-World/branches{/branch}",
							  "collaborators_url": "https://api.github.com/repos/octocat/Hello-World/collaborators{/collaborator}",
							  "comments_url": "https://api.github.com/repos/octocat/Hello-World/comments{/number}",
							  "commits_url": "https://api.github.com/repos/octocat/Hello-World/commits{/sha}",
							  "compare_url": "https://api.github.com/repos/octocat/Hello-World/compare/{base}...{head}",
							  "contents_url": "https://api.github.com/repos/octocat/Hello-World/contents/{+path}",
							  "contributors_url": "https://api.github.com/repos/octocat/Hello-World/contributors",
							  "deployments_url": "https://api.github.com/repos/octocat/Hello-World/deployments",
							  "downloads_url": "https://api.github.com/repos/octocat/Hello-World/downloads",
							  "events_url": "https://api.github.com/repos/octocat/Hello-World/events",
							  "forks_url": "https://api.github.com/repos/octocat/Hello-World/forks",
							  "git_commits_url": "https://api.github.com/repos/octocat/Hello-World/git/commits{/sha}",
							  "git_refs_url": "https://api.github.com/repos/octocat/Hello-World/git/refs{/sha}",
							  "git_tags_url": "https://api.github.com/repos/octocat/Hello-World/git/tags{/sha}",
							  "git_url": "git:github.com/octocat/Hello-World.git",
							  "issue_comment_url": "https://api.github.com/repos/octocat/Hello-World/issues/comments{/number}",
							  "issue_events_url": "https://api.github.com/repos/octocat/Hello-World/issues/events{/number}",
							  "issues_url": "https://api.github.com/repos/octocat/Hello-World/issues{/number}",
							  "keys_url": "https://api.github.com/repos/octocat/Hello-World/keys{/key_id}",
							  "labels_url": "https://api.github.com/repos/octocat/Hello-World/labels{/name}",
							  "languages_url": "https://api.github.com/repos/octocat/Hello-World/languages",
							  "merges_url": "https://api.github.com/repos/octocat/Hello-World/merges",
							  "milestones_url": "https://api.github.com/repos/octocat/Hello-World/milestones{/number}",
							  "notifications_url": "https://api.github.com/repos/octocat/Hello-World/notifications{?since,all,participating}",
							  "pulls_url": "https://api.github.com/repos/octocat/Hello-World/pulls{/number}",
							  "releases_url": "https://api.github.com/repos/octocat/Hello-World/releases{/id}",
							  "ssh_url": "git@github.com:octocat/Hello-World.git",
							  "stargazers_url": "https://api.github.com/repos/octocat/Hello-World/stargazers",
							  "statuses_url": "https://api.github.com/repos/octocat/Hello-World/statuses/{sha}",
							  "subscribers_url": "https://api.github.com/repos/octocat/Hello-World/subscribers",
							  "subscription_url": "https://api.github.com/repos/octocat/Hello-World/subscription",
							  "tags_url": "https://api.github.com/repos/octocat/Hello-World/tags",
							  "teams_url": "https://api.github.com/repos/octocat/Hello-World/teams",
							  "trees_url": "https://api.github.com/repos/octocat/Hello-World/git/trees{/sha}",
							  "clone_url": "https://github.com/octocat/Hello-World.git",
							  "mirror_url": "git:git.example.com/octocat/Hello-World",
							  "hooks_url": "https://api.github.com/repos/octocat/Hello-World/hooks",
							  "svn_url": "https://svn.github.com/octocat/Hello-World",
							  "homepage": "https://github.com",
							  "language": null,
							  "forks_count": 9,
							  "stargazers_count": 80,
							  "watchers_count": 80,
							  "size": 108,
							  "default_branch": "master",
							  "open_issues_count": 0,
							  "is_template": true,
							  "topics": [
								"octocat",
								"atom",
								"electron",
								"api"
							  ],
							  "has_issues": true,
							  "has_projects": true,
							  "has_wiki": true,
							  "has_pages": false,
							  "has_downloads": true,
							  "archived": false,
							  "disabled": false,
							  "visibility": "public",
							  "pushed_at": "2011-01-26T19:06:43Z",
							  "created_at": "2011-01-26T19:01:12Z",
							  "updated_at": "2011-01-26T19:14:43Z",
							  "permissions": {
								"admin": false,
								"push": false,
								"pull": true
							  },
							  "allow_rebase_merge": true,
							  "temp_clone_token": "dummy",
							  "allow_squash_merge": true,
							  "allow_auto_merge": false,
							  "delete_branch_on_merge": true,
							  "allow_merge_commit": true,
							  "subscribers_count": 42,
							  "network_count": 0,
							  "license": {
								"key": "mit",
								"name": "MIT License",
								"url": "https://api.github.com/licenses/mit",
								"spdx_id": "MIT",
								"node_id": "MDc6TGljZW5zZW1pdA==",
								"html_url": "https://api.github.com/licenses/mit"
							  },
							  "forks": 1,
							  "open_issues": 1,
							  "watchers": 1
							},
							"source": {
							  "id": 1296269,
							  "node_id": "MDEwOlJlcG9zaXRvcnkxMjk2MjY5",
							  "name": "Hello-World",
							  "full_name": "octocat/Hello-World",
							  "owner": {
								"login": "octocat",
								"id": 1,
								"node_id": "MDQ6VXNlcjE=",
								"avatar_url": "https://github.com/images/error/octocat_happy.gif",
								"gravatar_id": "",
								"url": "https://api.github.com/users/octocat",
								"html_url": "https://github.com/octocat",
								"followers_url": "https://api.github.com/users/octocat/followers",
								"following_url": "https://api.github.com/users/octocat/following{/other_user}",
								"gists_url": "https://api.github.com/users/octocat/gists{/gist_id}",
								"starred_url": "https://api.github.com/users/octocat/starred{/owner}{/repo}",
								"subscriptions_url": "https://api.github.com/users/octocat/subscriptions",
								"organizations_url": "https://api.github.com/users/octocat/orgs",
								"repos_url": "https://api.github.com/users/octocat/repos",
								"events_url": "https://api.github.com/users/octocat/events{/privacy}",
								"received_events_url": "https://api.github.com/users/octocat/received_events",
								"type": "User",
								"site_admin": false
							  },
							  "private": false,
							  "html_url": "https://github.com/octocat/Hello-World",
							  "description": "This your first repo!",
							  "fork": false,
							  "url": "https://api.github.com/repos/octocat/Hello-World",
							  "archive_url": "https://api.github.com/repos/octocat/Hello-World/{archive_format}{/ref}",
							  "assignees_url": "https://api.github.com/repos/octocat/Hello-World/assignees{/user}",
							  "blobs_url": "https://api.github.com/repos/octocat/Hello-World/git/blobs{/sha}",
							  "branches_url": "https://api.github.com/repos/octocat/Hello-World/branches{/branch}",
							  "collaborators_url": "https://api.github.com/repos/octocat/Hello-World/collaborators{/collaborator}",
							  "comments_url": "https://api.github.com/repos/octocat/Hello-World/comments{/number}",
							  "commits_url": "https://api.github.com/repos/octocat/Hello-World/commits{/sha}",
							  "compare_url": "https://api.github.com/repos/octocat/Hello-World/compare/{base}...{head}",
							  "contents_url": "https://api.github.com/repos/octocat/Hello-World/contents/{+path}",
							  "contributors_url": "https://api.github.com/repos/octocat/Hello-World/contributors",
							  "deployments_url": "https://api.github.com/repos/octocat/Hello-World/deployments",
							  "downloads_url": "https://api.github.com/repos/octocat/Hello-World/downloads",
							  "events_url": "https://api.github.com/repos/octocat/Hello-World/events",
							  "forks_url": "https://api.github.com/repos/octocat/Hello-World/forks",
							  "git_commits_url": "https://api.github.com/repos/octocat/Hello-World/git/commits{/sha}",
							  "git_refs_url": "https://api.github.com/repos/octocat/Hello-World/git/refs{/sha}",
							  "git_tags_url": "https://api.github.com/repos/octocat/Hello-World/git/tags{/sha}",
							  "git_url": "git:github.com/octocat/Hello-World.git",
							  "issue_comment_url": "https://api.github.com/repos/octocat/Hello-World/issues/comments{/number}",
							  "issue_events_url": "https://api.github.com/repos/octocat/Hello-World/issues/events{/number}",
							  "issues_url": "https://api.github.com/repos/octocat/Hello-World/issues{/number}",
							  "keys_url": "https://api.github.com/repos/octocat/Hello-World/keys{/key_id}",
							  "labels_url": "https://api.github.com/repos/octocat/Hello-World/labels{/name}",
							  "languages_url": "https://api.github.com/repos/octocat/Hello-World/languages",
							  "merges_url": "https://api.github.com/repos/octocat/Hello-World/merges",
							  "milestones_url": "https://api.github.com/repos/octocat/Hello-World/milestones{/number}",
							  "notifications_url": "https://api.github.com/repos/octocat/Hello-World/notifications{?since,all,participating}",
							  "pulls_url": "https://api.github.com/repos/octocat/Hello-World/pulls{/number}",
							  "releases_url": "https://api.github.com/repos/octocat/Hello-World/releases{/id}",
							  "ssh_url": "git@github.com:octocat/Hello-World.git",
							  "stargazers_url": "https://api.github.com/repos/octocat/Hello-World/stargazers",
							  "statuses_url": "https://api.github.com/repos/octocat/Hello-World/statuses/{sha}",
							  "subscribers_url": "https://api.github.com/repos/octocat/Hello-World/subscribers",
							  "subscription_url": "https://api.github.com/repos/octocat/Hello-World/subscription",
							  "tags_url": "https://api.github.com/repos/octocat/Hello-World/tags",
							  "teams_url": "https://api.github.com/repos/octocat/Hello-World/teams",
							  "trees_url": "https://api.github.com/repos/octocat/Hello-World/git/trees{/sha}",
							  "clone_url": "https://github.com/octocat/Hello-World.git",
							  "mirror_url": "git:git.example.com/octocat/Hello-World",
							  "hooks_url": "https://api.github.com/repos/octocat/Hello-World/hooks",
							  "svn_url": "https://svn.github.com/octocat/Hello-World",
							  "homepage": "https://github.com",
							  "language": null,
							  "forks_count": 9,
							  "stargazers_count": 80,
							  "watchers_count": 80,
							  "size": 108,
							  "default_branch": "master",
							  "open_issues_count": 0,
							  "is_template": true,
							  "topics": [
								"octocat",
								"atom",
								"electron",
								"api"
							  ],
							  "has_issues": true,
							  "has_projects": true,
							  "has_wiki": true,
							  "has_pages": false,
							  "has_downloads": true,
							  "archived": false,
							  "disabled": false,
							  "visibility": "public",
							  "pushed_at": "2011-01-26T19:06:43Z",
							  "created_at": "2011-01-26T19:01:12Z",
							  "updated_at": "2011-01-26T19:14:43Z",
							  "permissions": {
								"admin": false,
								"push": false,
								"pull": true
							  },
							  "allow_rebase_merge": true,
							  "temp_clone_token": "dummy",
							  "allow_squash_merge": true,
							  "allow_auto_merge": false,
							  "delete_branch_on_merge": true,
							  "allow_merge_commit": true,
							  "subscribers_count": 42,
							  "network_count": 0,
							  "license": {
								"key": "mit",
								"name": "MIT License",
								"url": "https://api.github.com/licenses/mit",
								"spdx_id": "MIT",
								"node_id": "MDc6TGljZW5zZW1pdA==",
								"html_url": "https://api.github.com/licenses/mit"
							  },
							  "forks": 1,
							  "open_issues": 1,
							  "watchers": 1
							}
						  }`)))
				})

			},
			&api.Documentation{
				Structure: []*api.Node{
					&api.Node{
						Name:   "community",
						Source: "https://github.com/gardener/docforge/edit/master/integration-test/tested-doc/merge-test/testFile.md",
					},
					&api.Node{
						Name:   "docs",
						Source: "https://github.com/gardener/docforge/blob/testMainBranch/integration-test/tested-doc/merge-test/testFile.md",
					},
					&api.Node{
						Name:   "v7.7",
						Source: "https://github.com/gardener/docforge/blob/v7.7/integration-test/tested-doc/merge-test/testFile.md",
					},
					&api.Node{
						Name:   "v6.1",
						Source: "https://github.com/gardener/docforge/blob/v6.1/integration-test/tested-doc/merge-test/testFile.md",
					},
					&api.Node{
						Name:   "v5.7",
						Source: "https://github.com/gardener/docforge/blob/v5.7/integration-test/tested-doc/merge-test/testFile.md",
					},
					&api.Node{
						Name:   "v4.9",
						Source: "https://github.com/gardener/docforge/blob/v4.9/integration-test/tested-doc/merge-test/testFile.md",
					},
				},
			},
			nil,
		},
		{
			"github.com/testOrg/testRepo/testMainBranch2",
			"testManifest.yaml",
			nil,
			[]byte(fmt.Sprintf(`structure:
- name: community
  source: https://github.com/gardener/docforge/edit/master/integration-test/tested-doc/merge-test/testFile.md
{{- $vers := Split .versions "," -}}
{{- range $i, $version := $vers -}}
{{- if eq $i 0  }}
- name: docs
{{- else }}
- name: {{$version}}
{{- end }}
  source: https://github.com/gardener/docforge/blob/{{$version}}/integration-test/tested-doc/merge-test/testFile.md
{{- end }}`)),
			"https://github.com/testOrg/testRepo/blob/DEFAULT_BRANCH/testManifest.yaml",
			func(mux *http.ServeMux) {
				mux.HandleFunc("/repos/testOrg/testRepo/git/trees/testMainBranch2", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte(fmt.Sprintf(`
					{
						"sha": "9fb037999f264ba9a7fc6274d15fa3ae2ab98312",
						"url": "https://api.github.com/repos/testOrg/testRepo/git/trees/testMainBranch2",
						"tree": [
						  {
							"path": "testManifest.yaml",
							"mode": "100644",
							"type": "blob",
							"size": 30,
							"sha": "testSha",
							"url": "https://api.github.com/repos/testOrg/testRepo/git/trees/testMainBranch2/testManifest.yaml"
						  }
						],
						"truncated": false
					  }
					`)))
				})
				mux.HandleFunc("/repos/testOrg/testRepo", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte(fmt.Sprintf(`
						{
							"id": 1296269,
							"node_id": "MDEwOlJlcG9zaXRvcnkxMjk2MjY5",
							"name": "Hello-World",
							"full_name": "octocat/Hello-World",
							"owner": {
							  "login": "octocat",
							  "id": 1,
							  "node_id": "MDQ6VXNlcjE=",
							  "avatar_url": "https://github.com/images/error/octocat_happy.gif",
							  "gravatar_id": "",
							  "url": "https://api.github.com/users/octocat",
							  "html_url": "https://github.com/octocat",
							  "followers_url": "https://api.github.com/users/octocat/followers",
							  "following_url": "https://api.github.com/users/octocat/following{/other_user}",
							  "gists_url": "https://api.github.com/users/octocat/gists{/gist_id}",
							  "starred_url": "https://api.github.com/users/octocat/starred{/owner}{/repo}",
							  "subscriptions_url": "https://api.github.com/users/octocat/subscriptions",
							  "organizations_url": "https://api.github.com/users/octocat/orgs",
							  "repos_url": "https://api.github.com/users/octocat/repos",
							  "events_url": "https://api.github.com/users/octocat/events{/privacy}",
							  "received_events_url": "https://api.github.com/users/octocat/received_events",
							  "type": "User",
							  "site_admin": false
							},
							"private": false,
							"html_url": "https://github.com/octocat/Hello-World",
							"description": "This your first repo!",
							"fork": false,
							"url": "https://api.github.com/repos/octocat/Hello-World",
							"archive_url": "https://api.github.com/repos/octocat/Hello-World/{archive_format}{/ref}",
							"assignees_url": "https://api.github.com/repos/octocat/Hello-World/assignees{/user}",
							"blobs_url": "https://api.github.com/repos/octocat/Hello-World/git/blobs{/sha}",
							"branches_url": "https://api.github.com/repos/octocat/Hello-World/branches{/branch}",
							"collaborators_url": "https://api.github.com/repos/octocat/Hello-World/collaborators{/collaborator}",
							"comments_url": "https://api.github.com/repos/octocat/Hello-World/comments{/number}",
							"commits_url": "https://api.github.com/repos/octocat/Hello-World/commits{/sha}",
							"compare_url": "https://api.github.com/repos/octocat/Hello-World/compare/{base}...{head}",
							"contents_url": "https://api.github.com/repos/octocat/Hello-World/contents/{+path}",
							"contributors_url": "https://api.github.com/repos/octocat/Hello-World/contributors",
							"deployments_url": "https://api.github.com/repos/octocat/Hello-World/deployments",
							"downloads_url": "https://api.github.com/repos/octocat/Hello-World/downloads",
							"events_url": "https://api.github.com/repos/octocat/Hello-World/events",
							"forks_url": "https://api.github.com/repos/octocat/Hello-World/forks",
							"git_commits_url": "https://api.github.com/repos/octocat/Hello-World/git/commits{/sha}",
							"git_refs_url": "https://api.github.com/repos/octocat/Hello-World/git/refs{/sha}",
							"git_tags_url": "https://api.github.com/repos/octocat/Hello-World/git/tags{/sha}",
							"git_url": "git:github.com/octocat/Hello-World.git",
							"issue_comment_url": "https://api.github.com/repos/octocat/Hello-World/issues/comments{/number}",
							"issue_events_url": "https://api.github.com/repos/octocat/Hello-World/issues/events{/number}",
							"issues_url": "https://api.github.com/repos/octocat/Hello-World/issues{/number}",
							"keys_url": "https://api.github.com/repos/octocat/Hello-World/keys{/key_id}",
							"labels_url": "https://api.github.com/repos/octocat/Hello-World/labels{/name}",
							"languages_url": "https://api.github.com/repos/octocat/Hello-World/languages",
							"merges_url": "https://api.github.com/repos/octocat/Hello-World/merges",
							"milestones_url": "https://api.github.com/repos/octocat/Hello-World/milestones{/number}",
							"notifications_url": "https://api.github.com/repos/octocat/Hello-World/notifications{?since,all,participating}",
							"pulls_url": "https://api.github.com/repos/octocat/Hello-World/pulls{/number}",
							"releases_url": "https://api.github.com/repos/octocat/Hello-World/releases{/id}",
							"ssh_url": "git@github.com:octocat/Hello-World.git",
							"stargazers_url": "https://api.github.com/repos/octocat/Hello-World/stargazers",
							"statuses_url": "https://api.github.com/repos/octocat/Hello-World/statuses/{sha}",
							"subscribers_url": "https://api.github.com/repos/octocat/Hello-World/subscribers",
							"subscription_url": "https://api.github.com/repos/octocat/Hello-World/subscription",
							"tags_url": "https://api.github.com/repos/octocat/Hello-World/tags",
							"teams_url": "https://api.github.com/repos/octocat/Hello-World/teams",
							"trees_url": "https://api.github.com/repos/octocat/Hello-World/git/trees{/sha}",
							"clone_url": "https://github.com/octocat/Hello-World.git",
							"mirror_url": "git:git.example.com/octocat/Hello-World",
							"hooks_url": "https://api.github.com/repos/octocat/Hello-World/hooks",
							"svn_url": "https://svn.github.com/octocat/Hello-World",
							"homepage": "https://github.com",
							"language": null,
							"forks_count": 9,
							"forks": 9,
							"stargazers_count": 80,
							"watchers_count": 80,
							"watchers": 80,
							"size": 108,
							"default_branch": "testMainBranch2",
							"open_issues_count": 0,
							"open_issues": 0,
							"is_template": false,
							"topics": [
							  "octocat",
							  "atom",
							  "electron",
							  "api"
							],
							"has_issues": true,
							"has_projects": true,
							"has_wiki": true,
							"has_pages": false,
							"has_downloads": true,
							"archived": false,
							"disabled": false,
							"visibility": "public",
							"pushed_at": "2011-01-26T19:06:43Z",
							"created_at": "2011-01-26T19:01:12Z",
							"updated_at": "2011-01-26T19:14:43Z",
							"permissions": {
							  "pull": true,
							  "push": false,
							  "admin": false
							},
							"allow_rebase_merge": true,
							"template_repository": {
							  "id": 1296269,
							  "node_id": "MDEwOlJlcG9zaXRvcnkxMjk2MjY5",
							  "name": "Hello-World-Template",
							  "full_name": "octocat/Hello-World-Template",
							  "owner": {
								"login": "octocat",
								"id": 1,
								"node_id": "MDQ6VXNlcjE=",
								"avatar_url": "https://github.com/images/error/octocat_happy.gif",
								"gravatar_id": "",
								"url": "https://api.github.com/users/octocat",
								"html_url": "https://github.com/octocat",
								"followers_url": "https://api.github.com/users/octocat/followers",
								"following_url": "https://api.github.com/users/octocat/following{/other_user}",
								"gists_url": "https://api.github.com/users/octocat/gists{/gist_id}",
								"starred_url": "https://api.github.com/users/octocat/starred{/owner}{/repo}",
								"subscriptions_url": "https://api.github.com/users/octocat/subscriptions",
								"organizations_url": "https://api.github.com/users/octocat/orgs",
								"repos_url": "https://api.github.com/users/octocat/repos",
								"events_url": "https://api.github.com/users/octocat/events{/privacy}",
								"received_events_url": "https://api.github.com/users/octocat/received_events",
								"type": "User",
								"site_admin": false
							  },
							  "private": false,
							  "html_url": "https://github.com/octocat/Hello-World-Template",
							  "description": "This your first repo!",
							  "fork": false,
							  "url": "https://api.github.com/repos/octocat/Hello-World-Template",
							  "archive_url": "https://api.github.com/repos/octocat/Hello-World-Template/{archive_format}{/ref}",
							  "assignees_url": "https://api.github.com/repos/octocat/Hello-World-Template/assignees{/user}",
							  "blobs_url": "https://api.github.com/repos/octocat/Hello-World-Template/git/blobs{/sha}",
							  "branches_url": "https://api.github.com/repos/octocat/Hello-World-Template/branches{/branch}",
							  "collaborators_url": "https://api.github.com/repos/octocat/Hello-World-Template/collaborators{/collaborator}",
							  "comments_url": "https://api.github.com/repos/octocat/Hello-World-Template/comments{/number}",
							  "commits_url": "https://api.github.com/repos/octocat/Hello-World-Template/commits{/sha}",
							  "compare_url": "https://api.github.com/repos/octocat/Hello-World-Template/compare/{base}...{head}",
							  "contents_url": "https://api.github.com/repos/octocat/Hello-World-Template/contents/{+path}",
							  "contributors_url": "https://api.github.com/repos/octocat/Hello-World-Template/contributors",
							  "deployments_url": "https://api.github.com/repos/octocat/Hello-World-Template/deployments",
							  "downloads_url": "https://api.github.com/repos/octocat/Hello-World-Template/downloads",
							  "events_url": "https://api.github.com/repos/octocat/Hello-World-Template/events",
							  "forks_url": "https://api.github.com/repos/octocat/Hello-World-Template/forks",
							  "git_commits_url": "https://api.github.com/repos/octocat/Hello-World-Template/git/commits{/sha}",
							  "git_refs_url": "https://api.github.com/repos/octocat/Hello-World-Template/git/refs{/sha}",
							  "git_tags_url": "https://api.github.com/repos/octocat/Hello-World-Template/git/tags{/sha}",
							  "git_url": "git:github.com/octocat/Hello-World-Template.git",
							  "issue_comment_url": "https://api.github.com/repos/octocat/Hello-World-Template/issues/comments{/number}",
							  "issue_events_url": "https://api.github.com/repos/octocat/Hello-World-Template/issues/events{/number}",
							  "issues_url": "https://api.github.com/repos/octocat/Hello-World-Template/issues{/number}",
							  "keys_url": "https://api.github.com/repos/octocat/Hello-World-Template/keys{/key_id}",
							  "labels_url": "https://api.github.com/repos/octocat/Hello-World-Template/labels{/name}",
							  "languages_url": "https://api.github.com/repos/octocat/Hello-World-Template/languages",
							  "merges_url": "https://api.github.com/repos/octocat/Hello-World-Template/merges",
							  "milestones_url": "https://api.github.com/repos/octocat/Hello-World-Template/milestones{/number}",
							  "notifications_url": "https://api.github.com/repos/octocat/Hello-World-Template/notifications{?since,all,participating}",
							  "pulls_url": "https://api.github.com/repos/octocat/Hello-World-Template/pulls{/number}",
							  "releases_url": "https://api.github.com/repos/octocat/Hello-World-Template/releases{/id}",
							  "ssh_url": "git@github.com:octocat/Hello-World-Template.git",
							  "stargazers_url": "https://api.github.com/repos/octocat/Hello-World-Template/stargazers",
							  "statuses_url": "https://api.github.com/repos/octocat/Hello-World-Template/statuses/{sha}",
							  "subscribers_url": "https://api.github.com/repos/octocat/Hello-World-Template/subscribers",
							  "subscription_url": "https://api.github.com/repos/octocat/Hello-World-Template/subscription",
							  "tags_url": "https://api.github.com/repos/octocat/Hello-World-Template/tags",
							  "teams_url": "https://api.github.com/repos/octocat/Hello-World-Template/teams",
							  "trees_url": "https://api.github.com/repos/octocat/Hello-World-Template/git/trees{/sha}",
							  "clone_url": "https://github.com/octocat/Hello-World-Template.git",
							  "mirror_url": "git:git.example.com/octocat/Hello-World-Template",
							  "hooks_url": "https://api.github.com/repos/octocat/Hello-World-Template/hooks",
							  "svn_url": "https://svn.github.com/octocat/Hello-World-Template",
							  "homepage": "https://github.com",
							  "language": null,
							  "forks": 9,
							  "forks_count": 9,
							  "stargazers_count": 80,
							  "watchers_count": 80,
							  "watchers": 80,
							  "size": 108,
							  "default_branch": "master",
							  "open_issues": 0,
							  "open_issues_count": 0,
							  "is_template": true,
							  "license": {
								"key": "mit",
								"name": "MIT License",
								"url": "https://api.github.com/licenses/mit",
								"spdx_id": "MIT",
								"node_id": "MDc6TGljZW5zZW1pdA==",
								"html_url": "https://api.github.com/licenses/mit"
							  },
							  "topics": [
								"octocat",
								"atom",
								"electron",
								"api"
							  ],
							  "has_issues": true,
							  "has_projects": true,
							  "has_wiki": true,
							  "has_pages": false,
							  "has_downloads": true,
							  "archived": false,
							  "disabled": false,
							  "visibility": "public",
							  "pushed_at": "2011-01-26T19:06:43Z",
							  "created_at": "2011-01-26T19:01:12Z",
							  "updated_at": "2011-01-26T19:14:43Z",
							  "permissions": {
								"admin": false,
								"push": false,
								"pull": true
							  },
							  "allow_rebase_merge": true,
							  "temp_clone_token": "dummy",
							  "allow_squash_merge": true,
							  "allow_auto_merge": false,
							  "delete_branch_on_merge": true,
							  "allow_merge_commit": true,
							  "subscribers_count": 42,
							  "network_count": 0
							},
							"temp_clone_token": "dummy",
							"allow_squash_merge": true,
							"allow_auto_merge": false,
							"delete_branch_on_merge": true,
							"allow_merge_commit": true,
							"subscribers_count": 42,
							"network_count": 0,
							"license": {
							  "key": "mit",
							  "name": "MIT License",
							  "spdx_id": "MIT",
							  "url": "https://api.github.com/licenses/mit",
							  "node_id": "MDc6TGljZW5zZW1pdA=="
							},
							"organization": {
							  "login": "octocat",
							  "id": 1,
							  "node_id": "MDQ6VXNlcjE=",
							  "avatar_url": "https://github.com/images/error/octocat_happy.gif",
							  "gravatar_id": "",
							  "url": "https://api.github.com/users/octocat",
							  "html_url": "https://github.com/octocat",
							  "followers_url": "https://api.github.com/users/octocat/followers",
							  "following_url": "https://api.github.com/users/octocat/following{/other_user}",
							  "gists_url": "https://api.github.com/users/octocat/gists{/gist_id}",
							  "starred_url": "https://api.github.com/users/octocat/starred{/owner}{/repo}",
							  "subscriptions_url": "https://api.github.com/users/octocat/subscriptions",
							  "organizations_url": "https://api.github.com/users/octocat/orgs",
							  "repos_url": "https://api.github.com/users/octocat/repos",
							  "events_url": "https://api.github.com/users/octocat/events{/privacy}",
							  "received_events_url": "https://api.github.com/users/octocat/received_events",
							  "type": "Organization",
							  "site_admin": false
							},
							"parent": {
							  "id": 1296269,
							  "node_id": "MDEwOlJlcG9zaXRvcnkxMjk2MjY5",
							  "name": "Hello-World",
							  "full_name": "octocat/Hello-World",
							  "owner": {
								"login": "octocat",
								"id": 1,
								"node_id": "MDQ6VXNlcjE=",
								"avatar_url": "https://github.com/images/error/octocat_happy.gif",
								"gravatar_id": "",
								"url": "https://api.github.com/users/octocat",
								"html_url": "https://github.com/octocat",
								"followers_url": "https://api.github.com/users/octocat/followers",
								"following_url": "https://api.github.com/users/octocat/following{/other_user}",
								"gists_url": "https://api.github.com/users/octocat/gists{/gist_id}",
								"starred_url": "https://api.github.com/users/octocat/starred{/owner}{/repo}",
								"subscriptions_url": "https://api.github.com/users/octocat/subscriptions",
								"organizations_url": "https://api.github.com/users/octocat/orgs",
								"repos_url": "https://api.github.com/users/octocat/repos",
								"events_url": "https://api.github.com/users/octocat/events{/privacy}",
								"received_events_url": "https://api.github.com/users/octocat/received_events",
								"type": "User",
								"site_admin": false
							  },
							  "private": false,
							  "html_url": "https://github.com/octocat/Hello-World",
							  "description": "This your first repo!",
							  "fork": false,
							  "url": "https://api.github.com/repos/octocat/Hello-World",
							  "archive_url": "https://api.github.com/repos/octocat/Hello-World/{archive_format}{/ref}",
							  "assignees_url": "https://api.github.com/repos/octocat/Hello-World/assignees{/user}",
							  "blobs_url": "https://api.github.com/repos/octocat/Hello-World/git/blobs{/sha}",
							  "branches_url": "https://api.github.com/repos/octocat/Hello-World/branches{/branch}",
							  "collaborators_url": "https://api.github.com/repos/octocat/Hello-World/collaborators{/collaborator}",
							  "comments_url": "https://api.github.com/repos/octocat/Hello-World/comments{/number}",
							  "commits_url": "https://api.github.com/repos/octocat/Hello-World/commits{/sha}",
							  "compare_url": "https://api.github.com/repos/octocat/Hello-World/compare/{base}...{head}",
							  "contents_url": "https://api.github.com/repos/octocat/Hello-World/contents/{+path}",
							  "contributors_url": "https://api.github.com/repos/octocat/Hello-World/contributors",
							  "deployments_url": "https://api.github.com/repos/octocat/Hello-World/deployments",
							  "downloads_url": "https://api.github.com/repos/octocat/Hello-World/downloads",
							  "events_url": "https://api.github.com/repos/octocat/Hello-World/events",
							  "forks_url": "https://api.github.com/repos/octocat/Hello-World/forks",
							  "git_commits_url": "https://api.github.com/repos/octocat/Hello-World/git/commits{/sha}",
							  "git_refs_url": "https://api.github.com/repos/octocat/Hello-World/git/refs{/sha}",
							  "git_tags_url": "https://api.github.com/repos/octocat/Hello-World/git/tags{/sha}",
							  "git_url": "git:github.com/octocat/Hello-World.git",
							  "issue_comment_url": "https://api.github.com/repos/octocat/Hello-World/issues/comments{/number}",
							  "issue_events_url": "https://api.github.com/repos/octocat/Hello-World/issues/events{/number}",
							  "issues_url": "https://api.github.com/repos/octocat/Hello-World/issues{/number}",
							  "keys_url": "https://api.github.com/repos/octocat/Hello-World/keys{/key_id}",
							  "labels_url": "https://api.github.com/repos/octocat/Hello-World/labels{/name}",
							  "languages_url": "https://api.github.com/repos/octocat/Hello-World/languages",
							  "merges_url": "https://api.github.com/repos/octocat/Hello-World/merges",
							  "milestones_url": "https://api.github.com/repos/octocat/Hello-World/milestones{/number}",
							  "notifications_url": "https://api.github.com/repos/octocat/Hello-World/notifications{?since,all,participating}",
							  "pulls_url": "https://api.github.com/repos/octocat/Hello-World/pulls{/number}",
							  "releases_url": "https://api.github.com/repos/octocat/Hello-World/releases{/id}",
							  "ssh_url": "git@github.com:octocat/Hello-World.git",
							  "stargazers_url": "https://api.github.com/repos/octocat/Hello-World/stargazers",
							  "statuses_url": "https://api.github.com/repos/octocat/Hello-World/statuses/{sha}",
							  "subscribers_url": "https://api.github.com/repos/octocat/Hello-World/subscribers",
							  "subscription_url": "https://api.github.com/repos/octocat/Hello-World/subscription",
							  "tags_url": "https://api.github.com/repos/octocat/Hello-World/tags",
							  "teams_url": "https://api.github.com/repos/octocat/Hello-World/teams",
							  "trees_url": "https://api.github.com/repos/octocat/Hello-World/git/trees{/sha}",
							  "clone_url": "https://github.com/octocat/Hello-World.git",
							  "mirror_url": "git:git.example.com/octocat/Hello-World",
							  "hooks_url": "https://api.github.com/repos/octocat/Hello-World/hooks",
							  "svn_url": "https://svn.github.com/octocat/Hello-World",
							  "homepage": "https://github.com",
							  "language": null,
							  "forks_count": 9,
							  "stargazers_count": 80,
							  "watchers_count": 80,
							  "size": 108,
							  "default_branch": "master",
							  "open_issues_count": 0,
							  "is_template": true,
							  "topics": [
								"octocat",
								"atom",
								"electron",
								"api"
							  ],
							  "has_issues": true,
							  "has_projects": true,
							  "has_wiki": true,
							  "has_pages": false,
							  "has_downloads": true,
							  "archived": false,
							  "disabled": false,
							  "visibility": "public",
							  "pushed_at": "2011-01-26T19:06:43Z",
							  "created_at": "2011-01-26T19:01:12Z",
							  "updated_at": "2011-01-26T19:14:43Z",
							  "permissions": {
								"admin": false,
								"push": false,
								"pull": true
							  },
							  "allow_rebase_merge": true,
							  "temp_clone_token": "dummy",
							  "allow_squash_merge": true,
							  "allow_auto_merge": false,
							  "delete_branch_on_merge": true,
							  "allow_merge_commit": true,
							  "subscribers_count": 42,
							  "network_count": 0,
							  "license": {
								"key": "mit",
								"name": "MIT License",
								"url": "https://api.github.com/licenses/mit",
								"spdx_id": "MIT",
								"node_id": "MDc6TGljZW5zZW1pdA==",
								"html_url": "https://api.github.com/licenses/mit"
							  },
							  "forks": 1,
							  "open_issues": 1,
							  "watchers": 1
							},
							"source": {
							  "id": 1296269,
							  "node_id": "MDEwOlJlcG9zaXRvcnkxMjk2MjY5",
							  "name": "Hello-World",
							  "full_name": "octocat/Hello-World",
							  "owner": {
								"login": "octocat",
								"id": 1,
								"node_id": "MDQ6VXNlcjE=",
								"avatar_url": "https://github.com/images/error/octocat_happy.gif",
								"gravatar_id": "",
								"url": "https://api.github.com/users/octocat",
								"html_url": "https://github.com/octocat",
								"followers_url": "https://api.github.com/users/octocat/followers",
								"following_url": "https://api.github.com/users/octocat/following{/other_user}",
								"gists_url": "https://api.github.com/users/octocat/gists{/gist_id}",
								"starred_url": "https://api.github.com/users/octocat/starred{/owner}{/repo}",
								"subscriptions_url": "https://api.github.com/users/octocat/subscriptions",
								"organizations_url": "https://api.github.com/users/octocat/orgs",
								"repos_url": "https://api.github.com/users/octocat/repos",
								"events_url": "https://api.github.com/users/octocat/events{/privacy}",
								"received_events_url": "https://api.github.com/users/octocat/received_events",
								"type": "User",
								"site_admin": false
							  },
							  "private": false,
							  "html_url": "https://github.com/octocat/Hello-World",
							  "description": "This your first repo!",
							  "fork": false,
							  "url": "https://api.github.com/repos/octocat/Hello-World",
							  "archive_url": "https://api.github.com/repos/octocat/Hello-World/{archive_format}{/ref}",
							  "assignees_url": "https://api.github.com/repos/octocat/Hello-World/assignees{/user}",
							  "blobs_url": "https://api.github.com/repos/octocat/Hello-World/git/blobs{/sha}",
							  "branches_url": "https://api.github.com/repos/octocat/Hello-World/branches{/branch}",
							  "collaborators_url": "https://api.github.com/repos/octocat/Hello-World/collaborators{/collaborator}",
							  "comments_url": "https://api.github.com/repos/octocat/Hello-World/comments{/number}",
							  "commits_url": "https://api.github.com/repos/octocat/Hello-World/commits{/sha}",
							  "compare_url": "https://api.github.com/repos/octocat/Hello-World/compare/{base}...{head}",
							  "contents_url": "https://api.github.com/repos/octocat/Hello-World/contents/{+path}",
							  "contributors_url": "https://api.github.com/repos/octocat/Hello-World/contributors",
							  "deployments_url": "https://api.github.com/repos/octocat/Hello-World/deployments",
							  "downloads_url": "https://api.github.com/repos/octocat/Hello-World/downloads",
							  "events_url": "https://api.github.com/repos/octocat/Hello-World/events",
							  "forks_url": "https://api.github.com/repos/octocat/Hello-World/forks",
							  "git_commits_url": "https://api.github.com/repos/octocat/Hello-World/git/commits{/sha}",
							  "git_refs_url": "https://api.github.com/repos/octocat/Hello-World/git/refs{/sha}",
							  "git_tags_url": "https://api.github.com/repos/octocat/Hello-World/git/tags{/sha}",
							  "git_url": "git:github.com/octocat/Hello-World.git",
							  "issue_comment_url": "https://api.github.com/repos/octocat/Hello-World/issues/comments{/number}",
							  "issue_events_url": "https://api.github.com/repos/octocat/Hello-World/issues/events{/number}",
							  "issues_url": "https://api.github.com/repos/octocat/Hello-World/issues{/number}",
							  "keys_url": "https://api.github.com/repos/octocat/Hello-World/keys{/key_id}",
							  "labels_url": "https://api.github.com/repos/octocat/Hello-World/labels{/name}",
							  "languages_url": "https://api.github.com/repos/octocat/Hello-World/languages",
							  "merges_url": "https://api.github.com/repos/octocat/Hello-World/merges",
							  "milestones_url": "https://api.github.com/repos/octocat/Hello-World/milestones{/number}",
							  "notifications_url": "https://api.github.com/repos/octocat/Hello-World/notifications{?since,all,participating}",
							  "pulls_url": "https://api.github.com/repos/octocat/Hello-World/pulls{/number}",
							  "releases_url": "https://api.github.com/repos/octocat/Hello-World/releases{/id}",
							  "ssh_url": "git@github.com:octocat/Hello-World.git",
							  "stargazers_url": "https://api.github.com/repos/octocat/Hello-World/stargazers",
							  "statuses_url": "https://api.github.com/repos/octocat/Hello-World/statuses/{sha}",
							  "subscribers_url": "https://api.github.com/repos/octocat/Hello-World/subscribers",
							  "subscription_url": "https://api.github.com/repos/octocat/Hello-World/subscription",
							  "tags_url": "https://api.github.com/repos/octocat/Hello-World/tags",
							  "teams_url": "https://api.github.com/repos/octocat/Hello-World/teams",
							  "trees_url": "https://api.github.com/repos/octocat/Hello-World/git/trees{/sha}",
							  "clone_url": "https://github.com/octocat/Hello-World.git",
							  "mirror_url": "git:git.example.com/octocat/Hello-World",
							  "hooks_url": "https://api.github.com/repos/octocat/Hello-World/hooks",
							  "svn_url": "https://svn.github.com/octocat/Hello-World",
							  "homepage": "https://github.com",
							  "language": null,
							  "forks_count": 9,
							  "stargazers_count": 80,
							  "watchers_count": 80,
							  "size": 108,
							  "default_branch": "master",
							  "open_issues_count": 0,
							  "is_template": true,
							  "topics": [
								"octocat",
								"atom",
								"electron",
								"api"
							  ],
							  "has_issues": true,
							  "has_projects": true,
							  "has_wiki": true,
							  "has_pages": false,
							  "has_downloads": true,
							  "archived": false,
							  "disabled": false,
							  "visibility": "public",
							  "pushed_at": "2011-01-26T19:06:43Z",
							  "created_at": "2011-01-26T19:01:12Z",
							  "updated_at": "2011-01-26T19:14:43Z",
							  "permissions": {
								"admin": false,
								"push": false,
								"pull": true
							  },
							  "allow_rebase_merge": true,
							  "temp_clone_token": "dummy",
							  "allow_squash_merge": true,
							  "allow_auto_merge": false,
							  "delete_branch_on_merge": true,
							  "allow_merge_commit": true,
							  "subscribers_count": 42,
							  "network_count": 0,
							  "license": {
								"key": "mit",
								"name": "MIT License",
								"url": "https://api.github.com/licenses/mit",
								"spdx_id": "MIT",
								"node_id": "MDc6TGljZW5zZW1pdA==",
								"html_url": "https://api.github.com/licenses/mit"
							  },
							  "forks": 1,
							  "open_issues": 1,
							  "watchers": 1
							}
						  }`)))
				})

			},
			&api.Documentation{
				Structure: []*api.Node{
					&api.Node{
						Name:   "community",
						Source: "https://github.com/gardener/docforge/edit/master/integration-test/tested-doc/merge-test/testFile.md",
					},
					&api.Node{
						Name:   "docs",
						Source: "https://github.com/gardener/docforge/blob/testMainBranch2/integration-test/tested-doc/merge-test/testFile.md",
					},
				},
			},
			nil,
		},
	}
	for _, c := range cases {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		client, mux, _, teardown := setup()
		repo := make(map[string]*Repository)
		gitTags := make(map[string]([]string))
		mockGit := &mockGit{repoTags: gitTags}
		gitTags[filepath.FromSlash(c.repositoryPath)] = c.tags
		repo[c.repositoryPath] = &Repository{
			State:     Prepared,
			Git:       mockGit,
			LocalPath: c.repositoryPath,
		}
		filedata := make(map[string]([]byte))
		fileInfo := make(map[string]os.FileInfo)
		fileInfo[filepath.Join(c.repositoryPath, c.manifestName)] = mockFileInfo{isDir: false}
		filedata[filepath.Join(c.repositoryPath, c.manifestName)] = c.manifestData

		gh := &Git{
			client:        client,
			git:           mockGit,
			preparedRepos: repo,
			fileReader:    &mockFileSystem{mockFileContent: filedata, mockFileInfo: fileInfo},
		}
		defer teardown()
		if c.mux != nil {
			c.mux(mux)
		}
		var s map[string]int = make(map[string]int)
		s[c.uri] = len(c.tags)
		api.SetNVersions(s, s)
		api.SetFlagsVariables(make(map[string]string))
		//clear default branch cache
		ghub.ClearDefaultBranchesCache()
		got, gotErr := gh.ResolveDocumentation(ctx, c.uri)
		fmt.Println(gotErr)
		assert.Equal(t, c.err, gotErr)
		assert.Equal(t, c.want, got)
	}

}
