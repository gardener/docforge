// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0
package github_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/resourcehandlers/github"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	githubApi "github.com/google/go-github/v43/github"
	"k8s.io/utils/pointer"
)

func TestGithub(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Github Suite")
}

const (
	// baseURLPath is a non-empty client.BaseURL path to use during tests,
	// to ensure relative URLs are used for all endpoints. See issue #752.
	baseURLPath = "/api-v3"
)

// setup sets up a test HTTP server along with a github.client that is
// configured to talk to that test server. Tests should register handlers on
// mux which provide mock responses for the API method being tested.
func setup() (client *githubApi.Client, mux *http.ServeMux, serverURL string, teardown func()) {
	// mux is the HTTP request multiplexer used with the test server.
	mux = http.NewServeMux()

	// We want to ensure that tests catch mistakes where the endpoint URL is
	// specified as absolute rather than relative. It only makes a difference
	// when there's a non-empty base URL path. So, use that. See issue #752.
	apiHandler := http.NewServeMux()
	apiHandler.Handle(baseURLPath+"/", http.StripPrefix(baseURLPath, mux))
	apiHandler.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		fmt.Fprintln(os.Stderr, "FAIL: client.BaseURL path prefix is not preserved in the request URL:")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "\t"+req.URL.String())
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "\tDid you accidentally use an absolute endpoint URL rather than relative?")
		fmt.Fprintln(os.Stderr, "\tSee https://github.com/google/go-github/issues/752 for information.")
		http.Error(w, "client.BaseURL path prefix is not preserved in the request URL.", http.StatusInternalServerError)
	})

	// server is a test HTTP server used to provide mock API responses.
	server := httptest.NewServer(apiHandler)

	// client is the GitHub client being tested and is
	// configured to use test server.
	client = githubApi.NewClient(nil)
	url, _ := url.Parse(server.URL + baseURLPath + "/")
	client.BaseURL = url
	client.UploadURL = url

	return client, mux, server.URL, server.Close
}

var _ = Describe("Github", func() {

	var gh resourcehandlers.ResourceHandler

	BeforeEach(func() {
		gh = github.NewResourceHandler(nil, nil, nil, map[string]string{}, map[string]string{})
	})

	Describe("UrlToGitHubLocator", func() {
		ghrl1 := &github.ResourceLocator{
			"https",
			"github.com",
			"gardener",
			"gardener",
			"",
			github.Blob,
			"docs/README.md",
			"master",
			false,
		}
		ghrl2 := &github.ResourceLocator{
			"https",
			"github.com",
			"gardener",
			"gardener",
			"91776959202ec10db883c5cfc05c51e78403f02c",
			github.Blob,
			"docs/README.md",
			"master",
			false,
		}
		ghrl3 := &github.ResourceLocator{
			"https",
			"github.com",
			"gardener",
			"gardener",
			"",
			github.Pull,
			"123",
			"",
			false,
		}
		ghrl4 := &github.ResourceLocator{
			"https",
			"github.com",
			"gardener",
			"gardener",
			"s9n39h1bdc89nbv",
			github.Blob,
			"docs/img/image.png",
			"master",
			false,
		}
		emptyCache := github.NewEmptyCache(nil)
		cache1 := github.NewCache(map[string]*github.ResourceLocator{
			"github.com:gardener:gardener:master:docs/readme.md": ghrl2,
		}, nil)
		cache2 := github.NewCache(map[string]*github.ResourceLocator{
			"github.enterprise:org:repo:master:docs/img/img.png": ghrl4,
		}, nil)
		cache3 := github.NewCache(map[string]*github.ResourceLocator{
			"github.enterprise:org:repo:master:docs/img/image.png": ghrl4,
		}, nil)
		mux := func(mux *http.ServeMux) {
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
		}
		DescribeTable("processing entries",
			func(
				inURL string,
				inResolveAPI bool,
				cache *github.Cache,
				mux func(mux *http.ServeMux),
				want *github.ResourceLocator) {
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer cancel()
				if inResolveAPI {
					client, muxRes, _, teardown := setup()
					defer teardown()
					if mux != nil {
						mux(muxRes)
					}
					cache.SetTreeExtractor(&github.TreeExtractorGithub{Client: client})
				}
				ghrh := github.NewResourceHandlerTest(nil, nil, nil, cache)
				gh := ghrh.(*github.GitHub)
				got, err := gh.URLToGitHubLocator(ctx, inURL, inResolveAPI)

				Expect(err).NotTo(HaveOccurred())
				Expect(got).Should(Equal(want))
			},
			Entry("GitHub url should return valid GitHubResourceLocator",
				"https://github.com/gardener/gardener/blob/master/docs/README.md",
				false,
				emptyCache,
				nil,
				ghrl1),
			Entry("GitHub url should return valid GitHubResourceLocator from cache",
				"https://github.com/gardener/gardener/blob/master/docs/README.md",
				false,
				cache1,
				nil,
				ghrl2),
			Entry("GitHub url should return valid GitHubResourceLocator from cache raw as query parameter",
				"https://github.com/gardener/gardener/blob/master/docs/README.md?raw=true",
				false,
				cache1,
				nil,
				ghrl2),
			Entry("non-cached url should resolve a valid GitHubResourceLocator from API",
				"https://github.com/gardener/gardener/blob/master/docs/README.md",
				true,
				emptyCache,
				mux,
				ghrl2),
			Entry("non-SHAAlias GitHub url should return valid GitHubResourceLocator",
				"https://github.com/gardener/gardener/pull/123",
				false,
				emptyCache,
				nil,
				ghrl3),
			Entry("cached url with raw host should return valid GitHubResourceLocator",
				"https://raw.github.enterprise/org/repo/master/docs/img/img.png",
				false,
				cache2,
				nil,
				ghrl4),
			Entry("cached url with raw api should return valid GitHubResourceLocator",
				"https://github.enterprise/raw/org/repo/master/docs/img/image.png",
				true,
				cache3,
				nil,
				ghrl4),
		)
	})

	Describe("CleanupNodeTree", func() {
		var (
			node     *api.Node
			wantNode *api.Node
		)

		JustBeforeEach(func() {
			node.Cleanup()
		})

		Describe("general use case", func() {
			BeforeEach(func() {
				node = &api.Node{
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
						{
							Name:  "03",
							Nodes: []*api.Node{},
						},
						{
							Name:  "04",
							Nodes: []*api.Node{},
						},
					},
				}
				wantNode = &api.Node{
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
				}
			})

			It("should process it correctly", func() {
				Expect(node).Should(Equal(wantNode))
			})
		})
	})

	Describe("TreeEntryToGitHubLocator", func() {
		var (
			treeEntry *githubApi.TreeEntry
			shaalias  string

			got  *github.ResourceLocator
			want *github.ResourceLocator
		)
		BeforeEach(func() {
			shaalias = "master"
		})

		JustBeforeEach(func() {
			got = github.TreeEntryToGitHubLocator(treeEntry, shaalias)
		})

		Describe("when given an enterprise github entry", func() {
			BeforeEach(func() {
				treeEntry = &githubApi.TreeEntry{
					SHA:  pointer.StringPtr("b578f8f6cce210d44388e7136b9acce055da4d1b"),
					Path: pointer.StringPtr("docs/cluster_resources.md"),
					Mode: pointer.StringPtr("100644"),
					Type: pointer.StringPtr("blob"),
					Size: new(int),
					URL:  pointer.StringPtr("https://github.enterprise/api/v3/repos/test-org/test-repo/git/blobs/b578f8f6cce210d44388e7136b9acce055da4d1b"),
				}
				want = &github.ResourceLocator{
					Host:     "github.enterprise",
					Owner:    "test-org",
					Repo:     "test-repo",
					Path:     "docs/cluster_resources.md",
					SHA:      "b578f8f6cce210d44388e7136b9acce055da4d1b",
					Scheme:   "https",
					SHAAlias: "master",
					Type:     github.Blob,
				}
			})

			It("should return the expected ResourceLocator for a enterprise GitHub entry", func() {
				Expect(got).Should(Equal(want))
			})

		})

		Describe("when given an enterprise github raw entry", func() {
			BeforeEach(func() {
				treeEntry = &githubApi.TreeEntry{
					SHA:  pointer.StringPtr("b578f8f6cce210d44388e7136b9acce055da4d1b"),
					Path: pointer.StringPtr("docs/cluster_resources.md"),
					Mode: pointer.StringPtr("100644"),
					Type: pointer.StringPtr("blob"),
					Size: new(int),
					URL:  pointer.StringPtr("https://github.enterprise/api/v3/repos/test-org/test-repo/git/blobs/b578f8f6cce210d44388e7136b9acce055da4d1b"),
				}
				want = &github.ResourceLocator{
					Host:     "github.enterprise",
					Owner:    "test-org",
					Repo:     "test-repo",
					Path:     "docs/cluster_resources.md",
					SHA:      "b578f8f6cce210d44388e7136b9acce055da4d1b",
					Scheme:   "https",
					SHAAlias: "master",
					Type:     github.Blob,
				}
			})

			It("should return the expected ResourceLocator for a enterprise GitHub raw entry", func() {
				Expect(got).Should(Equal(want))
			})

		})
		Describe("when given an github raw entry", func() {
			BeforeEach(func() {
				treeEntry = &githubApi.TreeEntry{
					SHA:  pointer.StringPtr("b578f8f6cce210d44388e7136b9acce055da4d1b"),
					Path: pointer.StringPtr("docs/cluster_resources.md"),
					Mode: pointer.StringPtr("100644"),
					Type: pointer.StringPtr("blob"),
					Size: new(int),
					URL:  pointer.StringPtr("https://api.github.com/repos/test-org/test-repo/git/blobs/b578f8f6cce210d44388e7136b9acce055da4d1b"),
				}
				want = &github.ResourceLocator{
					Host:     "github.com",
					Owner:    "test-org",
					Repo:     "test-repo",
					Path:     "docs/cluster_resources.md",
					SHA:      "b578f8f6cce210d44388e7136b9acce055da4d1b",
					Scheme:   "https",
					SHAAlias: "master",
					Type:     github.Blob,
				}
			})

			It("should return the expected ResourceLocator for a GitHub raw entry", func() {
				Expect(got).Should(Equal(want))
			})

		})
		Describe("when given an github entry", func() {
			BeforeEach(func() {
				treeEntry = &githubApi.TreeEntry{
					SHA:  pointer.StringPtr("b578f8f6cce210d44388e7136b9acce055da4d1b"),
					Path: pointer.StringPtr("docs/cluster_resources.md"),
					Mode: pointer.StringPtr("100644"),
					Type: pointer.StringPtr("blob"),
					Size: new(int),
					URL:  pointer.StringPtr("https://api.github.com/repos/test-org/test-repo/git/blobs/b578f8f6cce210d44388e7136b9acce055da4d1b"),
				}
				want = &github.ResourceLocator{
					Host:     "github.com",
					Owner:    "test-org",
					Repo:     "test-repo",
					Path:     "docs/cluster_resources.md",
					SHA:      "b578f8f6cce210d44388e7136b9acce055da4d1b",
					Scheme:   "https",
					SHAAlias: "master",
					Type:     github.Blob,
				}
			})

			It("should return the expected ResourceLocator for a GitHub entry", func() {
				Expect(got).Should(Equal(want))
			})

		})
	})

	Describe("ReadGitInfo", func() {
		var (
			url string

			mux func(mux *http.ServeMux)

			got  []byte
			want []byte
			err  error
		)

		JustBeforeEach(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			ghrh := &github.GitHub{}
			client, muxGot, _, teardown := setup()
			defer teardown()
			if mux != nil {
				mux(muxGot)
			}
			ghrh.Client = client
			got, err = ghrh.ReadGitInfo(ctx, url)
		})

		Describe("the usual use case", func() {
			BeforeEach(func() {
				url = "https://github.com/testOrg2/testRepo2/blob/master/testRes"
				want = []byte(fmt.Sprintf(`{
  "lastmod": "2021-12-20 13:11:24",
  "publishdate": "2021-12-20 13:11:24",
  "author": {
    "login": "userx-usery",
    "id": 51451517,
    "node_id": "MDQ6VXNlcjUxNDUxNTE3",
    "avatar_url": "https://avatars.githubusercontent.com/u/51451517?v=4",
    "html_url": "https://github.com/userx-usery",
    "gravatar_id": "",
    "name": "userx usery",
    "email": "userx.usery@gmail.com",
    "type": "User",
    "site_admin": false,
    "url": "https://api.github.com/users/userx-usery",
    "events_url": "https://api.github.com/users/userx-usery/events{/privacy}",
    "following_url": "https://api.github.com/users/userx-usery/following{/other_user}",
    "followers_url": "https://api.github.com/users/userx-usery/followers",
    "gists_url": "https://api.github.com/users/userx-usery/gists{/gist_id}",
    "organizations_url": "https://api.github.com/users/userx-usery/orgs",
    "received_events_url": "https://api.github.com/users/userx-usery/received_events",
    "repos_url": "https://api.github.com/users/userx-usery/repos",
    "starred_url": "https://api.github.com/users/userx-usery/starred{/owner}{/repo}",
    "subscriptions_url": "https://api.github.com/users/userx-usery/subscriptions"
  },
  "weburl": "https://github.com/gardener/docforge",
  "shaalias": "master",
  "path": "testRes"
}`))
				mux = func(mux *http.ServeMux) {
					mux.HandleFunc("/repos/testOrg2/testRepo2/commits", func(w http.ResponseWriter, r *http.Request) {

						w.Write([]byte(fmt.Sprintf(`[
							{
							"sha": "6bea6bee790673592da2d1af784b79339bb8c2c6",
							"node_id": "C_kwDOEI7t4toAKDZiZWE2YmVlNzkwNjczNTkyZGEyZDFhZjc4NGI3OTMzOWJiOGMyYzY",
							"commit": {
							"author": {
								"name": "userx usery",
								"email": "userx.usery@gmail.com",
								"date": "2021-12-08T15:09:39Z"
							},
							"committer": {
								"name": "userx usery",
								"email": "userx.usery@gmail.com",
								"date": "2021-12-20T13:11:24Z"
							},
							"message": "Integrate Goldmark Markdown parser",
							"tree": {
								"sha": "85c0520f6a222319ae5b29e06afff6b2f16beb8f",
								"url": "https://api.github.com/repos/gardener/docforge/git/trees/85c0520f6a222319ae5b29e06afff6b2f16beb8f"
							},
							"url": "https://api.github.com/repos/gardener/docforge/git/commits/6bea6bee790673592da2d1af784b79339bb8c2c6",
							"comment_count": 0,
							"verification": {
								"verified": false,
								"reason": "unsigned",
								"signature": null,
								"payload": null
							}
							},
							"url": "https://api.github.com/repos/gardener/docforge/commits/6bea6bee790673592da2d1af784b79339bb8c2c6",
							"html_url": "https://github.com/gardener/docforge/commit/6bea6bee790673592da2d1af784b79339bb8c2c6",
							"comments_url": "https://api.github.com/repos/gardener/docforge/commits/6bea6bee790673592da2d1af784b79339bb8c2c6/comments",
							"author": {
							"login": "userx-usery",
							"id": 51451517,
							"node_id": "MDQ6VXNlcjUxNDUxNTE3",
							"avatar_url": "https://avatars.githubusercontent.com/u/51451517?v=4",
							"gravatar_id": "",
							"url": "https://api.github.com/users/userx-usery",
							"html_url": "https://github.com/userx-usery",
							"followers_url": "https://api.github.com/users/userx-usery/followers",
							"following_url": "https://api.github.com/users/userx-usery/following{/other_user}",
							"gists_url": "https://api.github.com/users/userx-usery/gists{/gist_id}",
							"starred_url": "https://api.github.com/users/userx-usery/starred{/owner}{/repo}",
							"subscriptions_url": "https://api.github.com/users/userx-usery/subscriptions",
							"organizations_url": "https://api.github.com/users/userx-usery/orgs",
							"repos_url": "https://api.github.com/users/userx-usery/repos",
							"events_url": "https://api.github.com/users/userx-usery/events{/privacy}",
							"received_events_url": "https://api.github.com/users/userx-usery/received_events",
							"type": "User",
							"site_admin": false
							},
							"committer": {
							"login": "userx-usery",
							"id": 51451517,
							"node_id": "MDQ6VXNlcjUxNDUxNTE3",
							"avatar_url": "https://avatars.githubusercontent.com/u/51451517?v=4",
							"gravatar_id": "",
							"url": "https://api.github.com/users/userx-usery",
							"html_url": "https://github.com/userx-usery",
							"followers_url": "https://api.github.com/users/userx-usery/followers",
							"following_url": "https://api.github.com/users/userx-usery/following{/other_user}",
							"gists_url": "https://api.github.com/users/userx-usery/gists{/gist_id}",
							"starred_url": "https://api.github.com/users/userx-usery/starred{/owner}{/repo}",
							"subscriptions_url": "https://api.github.com/users/userx-usery/subscriptions",
							"organizations_url": "https://api.github.com/users/userx-usery/orgs",
							"repos_url": "https://api.github.com/users/userx-usery/repos",
							"events_url": "https://api.github.com/users/userx-usery/events{/privacy}",
							"received_events_url": "https://api.github.com/users/userx-usery/received_events",
							"type": "User",
							"site_admin": false
							},
							"parents": [
							{
								"sha": "c3a04a0827224f4c78f4f07349587c270b5c00d6",
								"url": "https://api.github.com/repos/gardener/docforge/commits/c3a04a0827224f4c78f4f07349587c270b5c00d6",
								"html_url": "https://github.com/gardener/docforge/commit/c3a04a0827224f4c78f4f07349587c270b5c00d6"
							}
							]
							}
							]
							`)))
					})
				}

			})

			It("should process it correctly", func() {

				Expect(err).NotTo(HaveOccurred())
				Expect(got).Should(Equal(want))
			})
		})
	})

	Describe("BuildAbsLink", func() {
		var (
			source string
			link   string

			got  string
			want string
			err  error
		)

		BeforeEach(func() {
			cachedRL := []string{"https://github.com/gardener/gardener/tree/master/jjbj.md", "https://github.com/gardener/gardener/tree/master/images/jjbj.png", "https://github.com/gardener/external-dns-management/blob/master/docs/aws-route53/README.md"}
			cache := github.NewTestCache(cachedRL)

			gh = github.NewResourceHandlerTest(nil, nil, nil, cache)

		})

		JustBeforeEach(func() {

			got, err = gh.BuildAbsLink(source, link)
		})

		Describe("test nested relative link", func() {
			BeforeEach(func() {
				source = "https://github.com/gardener/gardener/tree/master/readme.md"
				link = "jjbj.md"
				want = "https://github.com/gardener/gardener/tree/master/jjbj.md"
			})

			It("should process it correctly", func() {
				Expect(got).Should(Equal(want))
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Describe("test outside link", func() {
			BeforeEach(func() {
				source = "https://github.com/gardener/gardener/tree/master/docs/extensions/readme.md"
				link = "../../images/jjbj.png"
				want = "https://github.com/gardener/gardener/tree/master/images/jjbj.png"
			})

			It("should process it correctly", func() {
				Expect(got).Should(Equal(want))
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Describe("test root link", func() {
			BeforeEach(func() {
				source = "https://github.com/gardener/external-dns-management/blob/master/README.md"
				link = "/docs/aws-route53/README.md"
				want = "https://github.com/gardener/external-dns-management/blob/master/docs/aws-route53/README.md"
			})

			It("should process it correctly", func() {
				Expect(got).Should(Equal(want))
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Describe("test not found", func() {
			BeforeEach(func() {
				source = "https://github.com/gardener-samples/kube-overprovisioning/blob/master/test/README.md"
				link = "images/test.png"
				want = "https://github.com/gardener-samples/kube-overprovisioning/blob/master/test/images/test.png"
			})

			It("should process it correctly", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).Should(Equal(resourcehandlers.ErrResourceNotFound(want)))
			})
		})
	})

	Describe("ResourceName", func() {
		var (
			link      string
			resource  string
			extention string
		)

		JustBeforeEach(func() {
			resource, extention = gh.ResourceName(link)
		})

		When("not a valid url", func() {
			BeforeEach(func() {
				link = "http:/invalid.com//"
			})

			It("should return err", func() {
				Expect(resource).Should(Equal(""))
				Expect(extention).Should(Equal(""))
			})
		})

		When("valid url", func() {
			BeforeEach(func() {
				link = "https://github.com/index.html?page=1"
			})

			It("should return resource and extension", func() {
				Expect(resource).Should(Equal("index"))
				Expect(extention).Should(Equal("html"))
			})
		})
	})

	Describe("GetRawFormatLink", func() {
		var (
			absLink string
			got     string
			err     error
		)

		JustBeforeEach(func() {
			got, err = gh.GetRawFormatLink(absLink)
		})

		When("not a github rl", func() {
			BeforeEach(func() {
				absLink = "http://not.git.com/gardener/docforge/master"
			})

			It("should return err", func() {
				Expect(err).To(HaveOccurred())
				Expect(got).Should(Equal(""))
			})
		})

		When("not a blob or raw github rl", func() {
			BeforeEach(func() {
				absLink = "http://not.git.com/gardener/docforge/tree/master/dir"
			})

			It("should not change absLinks", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(got).Should(Equal(absLink))
			})
		})

		When("a blob or raw github rl", func() {
			BeforeEach(func() {
				absLink = "http://not.git.com/gardener/docforge/blob/master/dir"
			})

			It("should change absLinks to raw", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(got).Should(Equal("http://not.git.com/gardener/docforge/raw/master/dir"))
			})
		})
	})

	Describe("Accept url", func() {
		var (
			acceptedHosts []string
			url           string
			got           bool
		)

		JustBeforeEach(func() {
			gh = github.NewResourceHandler(nil, nil, acceptedHosts, map[string]string{}, map[string]string{})
			got = gh.Accept(url)
		})

		When("accepted hosts is not set (nil)", func() {
			It("should return false by default", func() {
				Expect(got).Should(Equal(false))
			})
		})

		When("accepted hosts is set", func() {
			BeforeEach(func() {
				acceptedHosts = []string{"github.com", "test.com"}
			})

			When("given a relative path", func() {
				BeforeEach(func() {
					url = "/relative/path"
				})

				It("should not accept it", func() {
					Expect(got).Should(Equal(false))
				})
			})

			When("not a valid url", func() {
				BeforeEach(func() {
					url = "http:/invalid.com//"
				})

				It("should not accept it", func() {
					Expect(got).Should(Equal(false))
				})
			})

			When("not a github resource locator", func() {
				BeforeEach(func() {
					url = "http://github.com"
				})

				It("should not accept it", func() {
					Expect(got).Should(Equal(false))
				})
			})

			When("resource locator is not in accepted hosts", func() {
				BeforeEach(func() {
					url = "http://github.com/gardener/docforge"
					acceptedHosts = []string{"test.com"}
				})

				It("should not accept it", func() {
					Expect(got).Should(Equal(false))
				})
			})

			When("resource locator is in accepted hosts", func() {
				BeforeEach(func() {
					url = "http://github.com/gardener/docforge"
				})

				It("should accept it", func() {
					Expect(got).Should(Equal(true))
				})
			})
		})

	})

	Describe("Read", func() {
		var (
			mux func(mux *http.ServeMux)

			uri string
			got []byte
			err error
		)

		BeforeEach(func() {
			mux = func(mux *http.ServeMux) {
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
						  },
						  {
							"path": "dir",
							"mode": "100644",
							"type": "tree",
							"size": 30,
							"sha": "testSha2",
							"url": "https://api.github.com/repos/testOrg/testRepo/git/trees/testMainBranch/dir"
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
- name: docs
source: https://github.com/gardener/docforge/blob/master/integration-test/tested-doc/merge-test/testFile.md
`)))
				})

			}
		})

		JustBeforeEach(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			client, muxFromSetup, _, teardown := setup()
			gh = github.NewResourceHandler(client, nil, nil, map[string]string{}, map[string]string{})
			defer teardown()
			if mux != nil {
				mux(muxFromSetup)
			}

			got, err = gh.Read(ctx, uri)
		})

		Context("repo file does not exist", func() {

			BeforeEach(func() {
				uri = "https://github.com/testOrg/testRepo/blob/testMainBranch/noexist.yaml"

			})

			It("should return error ErrResourceNotFound", func() {
				Expect(err).Should(Equal(resourcehandlers.ErrResourceNotFound(uri)))
				Expect(got).Should(Equal([]byte{}))
			})
		})

		Context("repo file exists and is a directory", func() {
			BeforeEach(func() {
				uri = "https://github.com/testOrg/testRepo/blob/testMainBranch/dir"

			})

			It("should return nil ,nil", func() {
				Expect(err).Should(BeNil())
				Expect(got).Should(BeNil())
			})
		})

		Context("repo file exists", func() {
			var want []byte

			BeforeEach(func() {
				uri = "https://github.com/testOrg/testRepo/blob/testMainBranch/testManifest.yaml"
				want = []byte(fmt.Sprintf(`structure:
- name: community
source: https://github.com/gardener/docforge/edit/master/integration-test/tested-doc/merge-test/testFile.md
- name: docs
source: https://github.com/gardener/docforge/blob/master/integration-test/tested-doc/merge-test/testFile.md
`))
			})

			It("should read it correctly", func() {
				Expect(got).Should(Equal(want))
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("Resolving documentation", func() {
		var (
			uri string
			mux func(mux *http.ServeMux)
			got *api.Documentation
			err error
		)

		JustBeforeEach(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			client, muxFromSetup, _, teardown := setup()
			gh = github.NewResourceHandler(client, nil, nil, map[string]string{}, map[string]string{})
			defer teardown()
			if mux != nil {
				mux(muxFromSetup)
			}

			got, err = gh.ResolveDocumentation(ctx, uri)
		})
		Context("given the general use case", func() {
			BeforeEach(func() {
				uri = "https://github.com/testOrg/testRepo/blob/DEFAULT_BRANCH/testManifest.yaml"

				mux = func(mux *http.ServeMux) {
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
- name: docs
  source: https://github.com/gardener/docforge/blob/master/integration-test/tested-doc/merge-test/testFile.md
`)))
					})

					mux.HandleFunc("/repos/testOrg/testRepo", func(w http.ResponseWriter, r *http.Request) {
						w.Write([]byte(fmt.Sprintf(`
							{	
								"default_branch": "testMainBranch"
							}`)))
					})

				}
			})
			It("should process it correctly", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(got).To(Equal(&api.Documentation{
					Structure: []*api.Node{
						{
							Name:   "community",
							Source: "https://github.com/gardener/docforge/edit/master/integration-test/tested-doc/merge-test/testFile.md",
						},
						{
							Name:   "docs",
							Source: "https://github.com/gardener/docforge/blob/master/integration-test/tested-doc/merge-test/testFile.md",
						},
					},
				}))
			})
		})
	})

	Describe("Resolving node selector", func() {
		var (
			node         *api.Node
			excludePaths []string
			depth        int32
			mux          func(mux *http.ServeMux)
			got          []*api.Node
			expected     []*api.Node
			err          error
		)

		BeforeEach(func() {
			depth = 0
			excludePaths = nil
			node = &api.Node{
				NodeSelector: &api.NodeSelector{
					Path: "https://github.com/testorg/testrepo/tree/testbranch/testdir",
				},
			}

			mux = func(mux *http.ServeMux) {
				mux.HandleFunc("/repos/testorg/testrepo/git/trees/testbranch", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte(fmt.Sprintf(`
						{
							"url": "http://api.github.com/repos/testorg/testrepo/git/trees/",
							"tree": [
								{
									"path": "testdir",
									"mode": "040000",
									"type": "tree",
									"url": "https://api.github.com/repos/testorg/testrepo/git/trees/"
								},
								{
									"path": "testdir/testfile.md",
									"mode": "100644",
									"type": "blob",
									"url": "https://api.github.com/repos/testorg/testrepo/git/blobs/"
								},
								{
									"path": "testdir/testdir_sub",
									"mode": "040000",
									"type": "tree",
									"url": "https://api.github.com/repos/testorg/testrepo/git/trees/"
								},
								{
									"path": "testdir/testdir_sub/testdir_sub2",
									"mode": "040000",
									"type": "tree",
									"url": "https://api.github.com/repos/testorg/testrepo/git/trees/"
								},
								{
									"path": "testdir/testdir_sub/testdir_sub2/testfile3.md",
									"mode": "100644",
									"type": "blob",
									"url": "https://api.github.com/repos/testorg/testrepo/git/blobs/"
								},
								{
									"path": "testfile2.md",
									"mode": "100644",
									"type": "blob",
									"url": "https://api.github.com/repos/testorg/testrepo/git/blobs/"
								}
							]
						}`)))
				})
			}
		})

		JustBeforeEach(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Minute)
			defer cancel()

			client, muxRes, _, teardown := setup()

			gh = github.NewResourceHandlerTest(nil, nil, nil, github.NewEmptyCache(&github.TreeExtractorGithub{Client: client}))
			defer teardown()
			if mux != nil {
				mux(muxRes)
			}
			node.NodeSelector.ExcludePaths = excludePaths
			node.NodeSelector.Depth = depth

			got, err = gh.ResolveNodeSelector(ctx, node)
		})

		Context("given the general use case", func() {
			BeforeEach(func() {

				testFile3 := &api.Node{Name: "testfile3.md", Source: "https://github.com/testorg/testrepo/blob/testbranch/testdir/testdir_sub/testdir_sub2/testfile3.md"}
				testSubDir2 := &api.Node{Name: "testdir_sub2", Nodes: []*api.Node{testFile3}, Properties: map[string]interface{}{api.ContainerNodeSourceLocation: "https://github.com/testorg/testrepo/tree/testbranch/testdir/testdir_sub/testdir_sub2"}}
				testSubDir := &api.Node{Name: "testdir_sub", Nodes: []*api.Node{testSubDir2}, Properties: map[string]interface{}{api.ContainerNodeSourceLocation: "https://github.com/testorg/testrepo/tree/testbranch/testdir/testdir_sub"}}
				testFile := &api.Node{Name: "testfile.md", Source: "https://github.com/testorg/testrepo/blob/testbranch/testdir/testfile.md"}

				testSubDir.SetParentsDownwards()
				expected = []*api.Node{
					testSubDir,
					testFile,
				}
			})

			It("should process it correctly", func() {
				Expect(err).NotTo(HaveOccurred())
				rootGot := api.Node{Nodes: got}
				rootExpected := api.Node{Nodes: expected}
				Expect(rootGot).To(Equal(rootExpected))
			})

		})

		Context("given a depth parameter", func() {
			BeforeEach(func() {
				depth = 1

				testFile := &api.Node{Name: "testfile.md", Source: "https://github.com/testorg/testrepo/blob/testbranch/testdir/testfile.md"}

				expected = []*api.Node{
					testFile,
				}

			})

			It("should process it correctly", func() {
				Expect(err).NotTo(HaveOccurred())
				rootGot := api.Node{Nodes: got}
				rootExpected := api.Node{Nodes: expected}
				Expect(rootGot).To(Equal(rootExpected))
			})

		})

		Context("given a excludePath parameter", func() {
			BeforeEach(func() {
				excludePaths = []string{"testdir_sub2", "testfile.md"}

				expected = []*api.Node{}
			})

			It("should process it correctly", func() {
				Expect(err).NotTo(HaveOccurred())
				rootGot := api.Node{Nodes: got}
				rootExpected := api.Node{Nodes: expected}
				Expect(rootGot).To(Equal(rootExpected))
			})

		})

	})

})
