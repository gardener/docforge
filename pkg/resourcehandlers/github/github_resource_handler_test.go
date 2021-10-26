// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package github

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gardener/docforge/pkg/resourcehandlers"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/util/tests"
	"github.com/google/go-github/v32/github"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/pointer"
)

const (
	// baseURLPath is a non-empty Client.BaseURL path to use during tests,
	// to ensure relative URLs are used for all endpoints. See issue #752.
	baseURLPath = "/api-v3"
)

func init() {
	tests.SetKlogV(6)
}

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

func TestUrlToGitHubLocator(t *testing.T) {
	ghrl1 := &ResourceLocator{
		"https",
		"github.com",
		"gardener",
		"gardener",
		"",
		Blob,
		"docs/README.md",
		"master",
		false,
	}
	ghrl2 := &ResourceLocator{
		"https",
		"github.com",
		"gardener",
		"gardener",
		"91776959202ec10db883c5cfc05c51e78403f02c",
		Blob,
		"docs/README.md",
		"master",
		false,
	}
	ghrl3 := &ResourceLocator{
		"https",
		"github.com",
		"gardener",
		"gardener",
		"",
		Pull,
		"123",
		"",
		false,
	}
	ghrl4 := &ResourceLocator{
		"https",
		"github.com",
		"gardener",
		"gardener",
		"s9n39h1bdc89nbv",
		Blob,
		"docs/img/image.png",
		"master",
		false,
	}
	cases := []struct {
		description  string
		inURL        string
		inResolveAPI bool
		cache        *Cache
		mux          func(mux *http.ServeMux)
		want         *ResourceLocator
	}{
		{
			"GitHub url should return valid GitHubResourceLocator",
			"https://github.com/gardener/gardener/blob/master/docs/README.md",
			false,
			&Cache{
				cache: map[string]*ResourceLocator{},
			},
			nil,
			ghrl1,
		},
		{
			"GitHub url should return valid GitHubResourceLocator from cache",
			"https://github.com/gardener/gardener/blob/master/docs/README.md",
			false,
			&Cache{
				cache: map[string]*ResourceLocator{
					"github.com:gardener:gardener:master:docs/readme.md": ghrl2,
				},
			},
			nil,
			ghrl2,
		},
		{
			"GitHub url should return valid GitHubResourceLocator from cache raw as query parameter",
			"https://github.com/gardener/gardener/blob/master/docs/README.md?raw=true",
			false,
			&Cache{
				cache: map[string]*ResourceLocator{
					"github.com:gardener:gardener:master:docs/readme.md": ghrl2,
				},
			},
			nil,
			ghrl2,
		},
		{
			"non-cached url should resolve a valid GitHubResourceLocator from API",
			"https://github.com/gardener/gardener/blob/master/docs/README.md",
			true,
			&Cache{
				cache: map[string]*ResourceLocator{},
			},
			func(mux *http.ServeMux) {
				mux.HandleFunc("/repos/gardener/gardener/git/trees/master", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte(fmt.Sprintf(`
						{
							"sha": "0255b12f5b51f821e59cf5cf343cb0c36f1cb1f9",
							"url": "http://api.github.com/repos/gardener/gardener/git/trees/0255b12f5b51f821e59cf5cf343cb0c36f1cb1f9",
							"tree": [
								{
									"path": "docs/README.md",
									"mode": "100644",
									"type": "blob",
									"sha": "91776959202ec10db883c5cfc05c51e78403f02c",
									"size": 6260,
									"url": "https://api.github.com/repos/gardener/gardener/git/blobs/91776959202ec10db883c5cfc05c51e78403f02c"
								}
							]
						}`)))
				})
			},
			ghrl2,
		},
		{
			"non-SHAAlias GitHub url should return valid GitHubResourceLocator",
			"https://github.com/gardener/gardener/pull/123",
			false,
			&Cache{
				cache: map[string]*ResourceLocator{},
			},
			nil,
			ghrl3,
		},
		{
			"cached url with raw host should return valid GitHubResourceLocator",
			"https://raw.github.enterprise/org/repo/master/docs/img/img.png",
			false,
			&Cache{
				cache: map[string]*ResourceLocator{
					"github.enterprise:org:repo:master:docs/img/img.png": ghrl4,
				},
			},
			nil,
			ghrl4,
		},
		{
			"cached url with raw api should return valid GitHubResourceLocator",
			"https://github.enterprise/raw/org/repo/master/docs/img/image.png",
			true,
			&Cache{
				cache: map[string]*ResourceLocator{
					"github.enterprise:org:repo:master:docs/img/image.png": ghrl4,
				},
			},
			nil,
			ghrl4,
		},
	}
	for _, c := range cases {
		t.Run("", func(t *testing.T) {
			fmt.Println(c.description)
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			gh := &GitHub{
				cache: c.cache,
			}
			if c.inResolveAPI {
				client, mux, _, teardown := setup()
				defer teardown()
				if c.mux != nil {
					c.mux(mux)
				}
				gh.cache.ghClient = client
			}
			got, err := gh.URLToGitHubLocator(ctx, c.inURL, c.inResolveAPI)
			if err != nil {
				t.Errorf("Test failed %s", err.Error())
			}
			assert.Equal(t, c.want, got)
		})
	}
}

func TestResolveDocumentation(t *testing.T) {
	cases := []struct {
		uri  string
		mux  func(mux *http.ServeMux)
		want *api.Documentation
		err  error
	}{
		{
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
				mux.HandleFunc("/repos/testOrg/testRepo/git/blobs/testSha", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte(fmt.Sprintf(`structure:
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
{{- end }}`)))
				})
				mux.HandleFunc("/repos/testOrg/testRepo/git/matching-refs/tags", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte(fmt.Sprintf(`
					[
						{
						  "ref": "refs/tags/v4.9",
						  "node_id": "MDM6UmVmMjc3ODAyNDY2OnJlZnMvdGFncy92MC4wLjE=",
						  "url": "https://api.github.com/repos/gardener/docforge/git/refs/tags/v4.9",
						  "object": {
							"sha": "c5391f5187af434160c8056f47fbeeaed3670a9d",
							"type": "commit",
							"url": "https://api.github.com/repos/gardener/docforge/git/commits/c5391f5187af434160c8056f47fbeeaed3670a9d"
						  }
						},
						{
						  "ref": "refs/tags/v5.7",
						  "node_id": "MDM6UmVmMjc3ODAyNDY2OnJlZnMvdGFncy92MC4xLjA=",
						  "url": "https://api.github.com/repos/gardener/docforge/git/refs/tags/v5.7",
						  "object": {
							"sha": "6bd668f2353f7ae6cddab09ef1434defe6431b89",
							"type": "commit",
							"url": "https://api.github.com/repos/gardener/docforge/git/commits/6bd668f2353f7ae6cddab09ef1434defe6431b89"
						  }
						},
						{
						  "ref": "refs/tags/v6.1",
						  "node_id": "MDM6UmVmMjc3ODAyNDY2OnJlZnMvdGFncy92MC4yLjA=",
						  "url": "https://api.github.com/repos/gardener/docforge/git/refs/tags/v6.1",
						  "object": {
							"sha": "183554163eb56886860ba40af0c4b121379d4459",
							"type": "commit",
							"url": "https://api.github.com/repos/gardener/docforge/git/commits/183554163eb56886860ba40af0c4b121379d4459"
						  }
						},
						{
						  "ref": "refs/tags/v7.7",
						  "node_id": "MDM6UmVmMjc3ODAyNDY2OnJlZnMvdGFncy92MC4yLjA=",
						  "url": "https://api.github.com/repos/gardener/docforge/git/refs/tags/v7.7",
						  "object": {
							"sha": "183554163eb56886860ba40af0c4b121379d4459",
							"type": "commit",
							"url": "https://api.github.com/repos/gardener/docforge/git/commits/183554163eb56886860ba40af0c4b121379d4459"
						  }
						}
					]`)))
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
						Name:   "v4.9",
						Source: "https://github.com/gardener/docforge/blob/v4.9/integration-test/tested-doc/merge-test/testFile.md",
					},
					&api.Node{
						Name:   "v5.7",
						Source: "https://github.com/gardener/docforge/blob/v5.7/integration-test/tested-doc/merge-test/testFile.md",
					},
					&api.Node{
						Name:   "v6.1",
						Source: "https://github.com/gardener/docforge/blob/v6.1/integration-test/tested-doc/merge-test/testFile.md",
					},
					&api.Node{
						Name:   "v7.7",
						Source: "https://github.com/gardener/docforge/blob/v7.7/integration-test/tested-doc/merge-test/testFile.md",
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
		gh := &GitHub{
			cache: &Cache{
				cache:    map[string]*ResourceLocator{},
				ghClient: client,
			}}
		defer teardown()
		if c.mux != nil {
			c.mux(mux)
		}
		gh.Client = client
		var s map[string]int = make(map[string]int)
		s["default"] = 4
		api.SetVersions(s)
		api.SetFlagsVariables(make(map[string]string))
		got, gotErr := gh.ResolveDocumentation(ctx, c.uri)
		fmt.Println(gotErr)
		assert.Equal(t, c.err, gotErr)
		assert.Equal(t, c.want, got)
	}
}
func TestGetAllTags(t *testing.T) {
	cases := []struct {
		rl   *ResourceLocator
		mux  func(mux *http.ServeMux)
		want []string
		err  error
	}{
		{
			&ResourceLocator{
				"https",
				"github.com",
				"gardener",
				"gardener",
				"",
				Blob,
				"",
				"master",
				false,
			},
			func(mux *http.ServeMux) {
				mux.HandleFunc("/repos/gardener/gardener/git/matching-refs/tags", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte(fmt.Sprintf(`
					[
						{
						  "ref": "refs/tags/v0.0.1",
						  "node_id": "MDM6UmVmMjc3ODAyNDY2OnJlZnMvdGFncy92MC4wLjE=",
						  "url": "https://api.github.com/repos/gardener/docforge/git/refs/tags/v0.0.1",
						  "object": {
							"sha": "c5391f5187af434160c8056f47fbeeaed3670a9d",
							"type": "commit",
							"url": "https://api.github.com/repos/gardener/docforge/git/commits/c5391f5187af434160c8056f47fbeeaed3670a9d"
						  }
						},
						{
						  "ref": "refs/tags/v0.1.0",
						  "node_id": "MDM6UmVmMjc3ODAyNDY2OnJlZnMvdGFncy92MC4xLjA=",
						  "url": "https://api.github.com/repos/gardener/docforge/git/refs/tags/v0.1.0",
						  "object": {
							"sha": "6bd668f2353f7ae6cddab09ef1434defe6431b89",
							"type": "commit",
							"url": "https://api.github.com/repos/gardener/docforge/git/commits/6bd668f2353f7ae6cddab09ef1434defe6431b89"
						  }
						},
						{
						  "ref": "refs/tags/v0.2.0",
						  "node_id": "MDM6UmVmMjc3ODAyNDY2OnJlZnMvdGFncy92MC4yLjA=",
						  "url": "https://api.github.com/repos/gardener/docforge/git/refs/tags/v0.2.0",
						  "object": {
							"sha": "183554163eb56886860ba40af0c4b121379d4459",
							"type": "commit",
							"url": "https://api.github.com/repos/gardener/docforge/git/commits/183554163eb56886860ba40af0c4b121379d4459"
						  }
						}
					]`)))
				})
			},
			[]string{"v0.0.1", "v0.1.0", "v0.2.0"},
			nil,
		},
		{
			&ResourceLocator{
				"https",
				"github.com",
				"gardener",
				"emptyTest",
				"",
				Blob,
				"",
				"master",
				false,
			},
			func(mux *http.ServeMux) {
				mux.HandleFunc("/repos/gardener/emptyTest/git/matching-refs/tags", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte(fmt.Sprintf(`
					[]`)))
				})
			},
			[]string{},
			nil,
		},
	}
	for _, c := range cases {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		gh := &GitHub{}
		client, mux, _, teardown := setup()
		defer teardown()
		if c.mux != nil {
			c.mux(mux)
		}
		gh.Client = client
		got, gotErr := gh.getAllTags(ctx, c.rl)

		assert.Equal(t, c.err, gotErr)
		assert.Equal(t, c.want, got)
	}
}

func TestResolveNodeSelector(t *testing.T) {
	n1 := &api.Node{
		NodeSelector: &api.NodeSelector{
			Path: "https://github.com/gardener/gardener/tree/master/docs",
		},
	}
	cases := []struct {
		description        string
		inNode             *api.Node
		excludePaths       []string
		frontMatter        map[string]interface{}
		excludeFrontMatter map[string]interface{}
		depth              int32
		mux                func(mux *http.ServeMux)
		want               *api.Node
		wantError          error
	}{
		{
			"resolve node selector",
			n1,
			nil,
			nil,
			nil,
			0,
			func(mux *http.ServeMux) {
				mux.HandleFunc("/repos/gardener/gardener/git/trees/master", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte(fmt.Sprintf(`
						{
							"sha": "0255b12f5b51f821e59cf5cf343cb0c36f1cb1f9",
							"url": "http://api.github.com/repos/gardener/gardener/git/trees/0255b12f5b51f821e59cf5cf343cb0c36f1cb1f9",
							"tree": [
								{
									"path": "docs",
									"mode": "040000",
									"type": "tree",
									"sha": "5e11bda664b234920d85db5ca10055916c11e35d",
									"url": "https://api.github.com/repos/gardener/gardener/git/trees/5e11bda664b234920d85db5ca10055916c11e35d"
								},
								{
									"path": "docs/README.md",
									"mode": "100644",
									"type": "blob",
									"sha": "91776959202ec10db883c5cfc05c51e78403f02c",
									"size": 6260,
									"url": "https://api.github.com/repos/gardener/gardener/git/blobs/91776959202ec10db883c5cfc05c51e78403f02c"
								},
								{
									"path": "docs/concepts",
									"mode": "040000",
									"type": "tree",
									"sha": "e3ac8f22d00ab4423b184687d3ecc7e03e7393eb",
									"url": "https://api.github.com/repos/gardener/gardener/git/trees/e3ac8f22d00ab4423b184687d3ecc7e03e7393eb"
								},
								{
									"path": "docs/concepts/apiserver.md",
									"mode": "100644",
									"type": "blob",
									"sha": "30c4e21a53be25f9300f9cca8bd73309b1257d1f",
									"size": 5209,
									"url": "https://api.github.com/repos/gardener/gardener/git/blobs/30c4e21a53be25f9300f9cca8bd73309b1257d1f"
								}
							]
						}`)))
				})
			},
			&api.Node{
				NodeSelector: &api.NodeSelector{
					Path: "https://github.com/gardener/gardener/tree/master/docs",
				},
				Nodes: []*api.Node{
					{
						Name:   "README.md",
						Source: "https://github.com/gardener/gardener/blob/master/docs/README.md",
					},
					{
						Name: "concepts",
						Nodes: []*api.Node{
							{
								Name:   "apiserver.md",
								Source: "https://github.com/gardener/gardener/blob/master/docs/concepts/apiserver.md",
							},
						},
					},
				},
			},
			nil,
		},
	}
	// set source location for container nodes
	cases[0].want.Nodes[1].SetSourceLocation("https://github.com/gardener/gardener/tree/master/docs/concepts")
	for _, c := range cases {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		gh := &GitHub{
			cache: &Cache{
				cache: map[string]*ResourceLocator{},
			},
		}
		client, mux, _, teardown := setup()
		defer teardown()
		if c.mux != nil {
			c.mux(mux)
		}
		gh.cache.ghClient = client
		nodes, gotError := gh.ResolveNodeSelector(ctx, c.inNode, c.excludePaths, c.frontMatter, c.excludeFrontMatter, c.depth)
		if gotError != nil {
			t.Errorf("error == %q, want %q", gotError, c.wantError)
		}
		c.inNode.Nodes = append(c.inNode.Nodes, nodes...)
		c.want.SetParentsDownwards()
		api.SortNodesByName(c.inNode)
		api.SortNodesByName(c.want)
		assert.Equal(t, c.want, c.inNode)
	}
}

func TestName(t *testing.T) {
	ghrl1 := &ResourceLocator{
		"https",
		"github.com",
		"gardener",
		"gardener",
		"master",
		Blob,
		"docs/README.md",
		"",
		false,
	}
	ghrl2 := &ResourceLocator{
		"https",
		"github.com",
		"gardener",
		"gardener",
		"master",
		Tree,
		"docs",
		"",
		false,
	}
	testCases := []struct {
		description string
		inURL       string
		cache       *Cache
		wantName    string
		wantExt     string
	}{
		{
			"return file name for url",
			"https://github.com/gardener/gardener/blob/master/docs/README.md",
			&Cache{
				cache: map[string]*ResourceLocator{
					"github.com:gardener:gardener:master:docs/readme.md": ghrl1,
				},
			},
			"README",
			"md",
		},
		{
			"return folder name for url",
			"https://github.com/gardener/gardener/tree/master/docs",
			&Cache{
				cache: map[string]*ResourceLocator{
					"github.com:gardener:gardener:master:docs": ghrl2,
				},
			},
			"docs",
			"",
		},
	}
	for _, tc := range testCases {
		gh := &GitHub{
			cache: tc.cache,
		}
		gotName, gotExt := gh.ResourceName(tc.inURL)
		assert.Equal(t, tc.wantName, gotName)
		assert.Equal(t, tc.wantExt, gotExt)
	}
}

func TestRead(t *testing.T) {
	var sampleContent = []byte("Sample content")
	cases := []struct {
		description string
		inURI       string
		mux         func(mux *http.ServeMux)
		cache       *Cache
		want        []byte
		wantError   error
	}{
		{
			"read node source",
			"https://github.com/gardener/gardener/blob/master/docs/README.md",
			func(mux *http.ServeMux) {
				mux.HandleFunc("/repos/gardener/gardener/git/blobs/master", func(w http.ResponseWriter, r *http.Request) {
					w.Write(sampleContent)
				})
			},
			&Cache{
				cache: map[string]*ResourceLocator{
					"github.com:gardener:gardener:master:docs/readme.md": {
						"https",
						"github.com",
						"gardener",
						"gardener",
						"master",
						Blob,
						"docs/README.md",
						"",
						false,
					},
				},
			},
			sampleContent,
			nil,
		},
	}
	for _, c := range cases {
		fmt.Println(c.description)
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		client, mux, serverURL, teardown := setup()
		defer teardown()
		// rewrite cached url keys host to match the mock server

		gh := &GitHub{
			cache: c.cache,
		}
		if c.mux != nil {
			c.mux(mux)
		}
		gh.Client = client
		inURI := strings.Replace(c.inURI, "https://github.com", serverURL, 1)
		got, gotError := gh.Read(ctx, c.inURI)
		if gotError != nil {
			t.Errorf("error == %q, want %q", gotError, c.wantError)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("Read(ctx,%v) == %v, want %v", inURI, string(got), string(c.want))
		}
	}
}

func TestGitHub_ResolveRelLink(t *testing.T) {

	type args struct {
		source string
		link   string
	}
	tests := []struct {
		name        string
		args        args
		wantRelLink string
		notFound    bool
	}{
		{
			name: "test nested relative link",
			args: args{
				source: "https://github.com/gardener/gardener/tree/master/readme.md",
				link:   "jjbj.md",
			},
			wantRelLink: "https://github.com/gardener/gardener/tree/master/jjbj.md",
		},
		{
			name: "test outside link",
			args: args{
				source: "https://github.com/gardener/gardener/tree/master/docs/extensions/readme.md",
				link:   "../../images/jjbj.png",
			},
			wantRelLink: "https://github.com/gardener/gardener/tree/master/images/jjbj.png",
		},
		{
			name: "test root link",
			args: args{
				source: "https://github.com/gardener/external-dns-management/blob/master/README.md",
				link:   "/docs/aws-route53/README.md",
			},
			wantRelLink: "https://github.com/gardener/external-dns-management/blob/master/docs/aws-route53/README.md",
		},
		{
			name: "test not found",
			args: args{
				source: "https://github.com/gardener-samples/kube-overprovisioning/blob/master/test/README.md",
				link:   "images/test.png",
			},
			wantRelLink: "https://github.com/gardener-samples/kube-overprovisioning/blob/master/test/images/test.png",
			notFound:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl, _ := Parse(tt.wantRelLink)
			ghCache := &Cache{cache: map[string]*ResourceLocator{}}
			if !tt.notFound {
				rlKey, _ := ghCache.key(rl, false)
				ghCache.cache[rlKey] = rl
			}
			gh := &GitHub{cache: ghCache}
			gotRelLink, err := gh.BuildAbsLink(tt.args.source, tt.args.link)
			assert.Equal(t, tt.wantRelLink, gotRelLink)
			if tt.notFound {
				assert.Equal(t, resourcehandlers.ErrResourceNotFound(tt.wantRelLink), err)
			}
		})
	}
}

func TestCleanupNodeTree(t *testing.T) {
	tests := []struct {
		name     string
		node     *api.Node
		wantNode *api.Node
	}{
		{
			name: "",
			node: &api.Node{
				Name: "00",
				Nodes: []*api.Node{
					{
						Name:   "01.md",
						Source: "https://github.com/gardener/gardener/blob/master/docs/01.md",
					},
					{
						Name:   "02",
						Source: "https://github.com/gardener/gardener/tree/master/docs/02",
						Nodes: []*api.Node{
							{
								Name:   "021.md",
								Source: "https://github.com/gardener/gardener/blob/master/docs/021.md",
							},
						},
					},
					{
						Name:   "03",
						Source: "https://github.com/gardener/gardener/tree/master/docs/03",
						Nodes:  []*api.Node{},
					},
					{
						Name:   "04",
						Source: "https://github.com/gardener/gardener/tree/master/docs/04",
						Nodes:  []*api.Node{},
					},
				},
			},
			wantNode: &api.Node{
				Name: "00",
				Nodes: []*api.Node{
					{
						Name:   "01.md",
						Source: "https://github.com/gardener/gardener/blob/master/docs/01.md",
					},
					{
						Name: "02",
						Nodes: []*api.Node{
							{
								Name:   "021.md",
								Source: "https://github.com/gardener/gardener/blob/master/docs/021.md",
							},
						},
					},
				},
			},
		},
	}
	// set source location for container nodes
	tests[0].wantNode.Nodes[1].SetSourceLocation("https://github.com/gardener/gardener/tree/master/docs/02")
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			CleanupNodeTree(tc.node)
			assert.Equal(t, tc.wantNode, tc.node)
		})
	}
}

func TestTreeEntryToGitHubLocator(t *testing.T) {
	type args struct {
		treeEntry *github.TreeEntry
		shaAlias  string
	}
	tests := []struct {
		name       string
		args       args
		expectedRL *ResourceLocator
	}{
		{
			name: "should return the expected ResourceLocator for a enterprise GitHub entry",
			args: args{
				treeEntry: &github.TreeEntry{
					SHA:  pointer.StringPtr("b578f8f6cce210d44388e7136b9acce055da4d1b"),
					Path: pointer.StringPtr("docs/cluster_resources.md"),
					Mode: pointer.StringPtr("100644"),
					Type: pointer.StringPtr("blob"),
					Size: new(int),
					URL:  pointer.StringPtr("https://github.enterprise/api/v3/repos/test-org/test-repo/git/blobs/b578f8f6cce210d44388e7136b9acce055da4d1b"),
				},
				shaAlias: "master",
			},
			expectedRL: &ResourceLocator{
				Host:     "github.enterprise",
				Owner:    "test-org",
				Repo:     "test-repo",
				Path:     "docs/cluster_resources.md",
				SHA:      "b578f8f6cce210d44388e7136b9acce055da4d1b",
				Scheme:   "https",
				SHAAlias: "master",
				Type:     Blob,
			},
		},
		{
			name: "should return the expected ResourceLocator for a enterprise GitHub raw entry",
			args: args{
				treeEntry: &github.TreeEntry{
					SHA:  pointer.StringPtr("b578f8f6cce210d44388e7136b9acce055da4d1b"),
					Path: pointer.StringPtr("docs/cluster_resources.md"),
					Mode: pointer.StringPtr("100644"),
					Type: pointer.StringPtr("blob"),
					Size: new(int),
					URL:  pointer.StringPtr("https://github.enterprise/api/v3/repos/test-org/test-repo/git/blobs/b578f8f6cce210d44388e7136b9acce055da4d1b"),
				},
				shaAlias: "master",
			},
			expectedRL: &ResourceLocator{
				Host:     "github.enterprise",
				Owner:    "test-org",
				Repo:     "test-repo",
				Path:     "docs/cluster_resources.md",
				SHA:      "b578f8f6cce210d44388e7136b9acce055da4d1b",
				Scheme:   "https",
				SHAAlias: "master",
				Type:     Blob,
			},
		},
		{
			name: "should return the expected ResourceLocator for a GitHub raw entry",
			args: args{
				treeEntry: &github.TreeEntry{
					SHA:  pointer.StringPtr("b578f8f6cce210d44388e7136b9acce055da4d1b"),
					Path: pointer.StringPtr("docs/cluster_resources.md"),
					Mode: pointer.StringPtr("100644"),
					Type: pointer.StringPtr("blob"),
					Size: new(int),
					URL:  pointer.StringPtr("https://api.github.com/repos/test-org/test-repo/git/blobs/b578f8f6cce210d44388e7136b9acce055da4d1b"),
				},
				shaAlias: "master",
			},
			expectedRL: &ResourceLocator{
				Host:     "github.com",
				Owner:    "test-org",
				Repo:     "test-repo",
				Path:     "docs/cluster_resources.md",
				SHA:      "b578f8f6cce210d44388e7136b9acce055da4d1b",
				Scheme:   "https",
				SHAAlias: "master",
				Type:     Blob,
			},
		},
		{
			name: "should return the expected ResourceLocator for a GitHub entry",
			args: args{
				treeEntry: &github.TreeEntry{
					SHA:  pointer.StringPtr("b578f8f6cce210d44388e7136b9acce055da4d1b"),
					Path: pointer.StringPtr("docs/cluster_resources.md"),
					Mode: pointer.StringPtr("100644"),
					Type: pointer.StringPtr("blob"),
					Size: new(int),
					URL:  pointer.StringPtr("https://api.github.com/repos/test-org/test-repo/git/blobs/b578f8f6cce210d44388e7136b9acce055da4d1b"),
				},
				shaAlias: "master",
			},
			expectedRL: &ResourceLocator{
				Host:     "github.com",
				Owner:    "test-org",
				Repo:     "test-repo",
				Path:     "docs/cluster_resources.md",
				SHA:      "b578f8f6cce210d44388e7136b9acce055da4d1b",
				Scheme:   "https",
				SHAAlias: "master",
				Type:     Blob,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TreeEntryToGitHubLocator(tt.args.treeEntry, tt.args.shaAlias); !reflect.DeepEqual(got, tt.expectedRL) {
				t.Errorf("TreeEntryToGitHubLocator() = %v, want %v", got, tt.expectedRL)
			}
		})
	}
}

func TestSetVersion(t *testing.T) {
	tests := []struct {
		url         string
		version     string
		expectedURL string
		expectedErr bool
	}{
		{
			"https://github.com/gardener/gardener/blob/master/docs/README.md",
			"v1.12.0",
			"https://github.com/gardener/gardener/blob/v1.12.0/docs/README.md",
			false,
		},
		{
			"https://github.com/gardener/gardener/pull/1234",
			"v1.12.0",
			"https://github.com/gardener/gardener/pull/1234",
			false,
		},
		{
			"https://kubernetes.io",
			"v1.12.0",
			"",
			true,
		},
	}
	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			gh := GitHub{}
			gotURL, gotErr := gh.SetVersion(tc.url, tc.version)
			if tc.expectedErr {
				assert.Error(t, gotErr)
			} else {
				assert.Nil(t, gotErr)
			}
			assert.Equal(t, tc.expectedURL, gotURL)
		})
	}
}
