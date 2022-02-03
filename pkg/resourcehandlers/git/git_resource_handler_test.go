// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0
package git_test

import (
	"context"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/resourcehandlers/git"
	"github.com/gardener/docforge/pkg/resourcehandlers/git/gitfakes"
	"github.com/gardener/docforge/pkg/resourcehandlers/git/gitinterface/gitinterfacefakes"
	ghub "github.com/gardener/docforge/pkg/resourcehandlers/github"

	"github.com/google/go-github/v32/github"
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

var (
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
		mux.HandleFunc("/repos/testOrg/testRepo", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(fmt.Sprintf(`
			{	
				"default_branch": "testMainBranch"
			}
		`)))
		})
		mux.HandleFunc("/repos/gardener/docforge/commits?path=testRes&sha=master", func(w http.ResponseWriter, r *http.Request) {
			fmt.Print("yay")

			w.Write([]byte(fmt.Sprintf(`
			{	
				"default_branch": "testMainBranch"
			}
		`)))
		})

		mux.HandleFunc("/repos/testOrg2/testRepo2/commits", func(w http.ResponseWriter, r *http.Request) {

			w.Write([]byte(fmt.Sprintf(`
			[
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
)

var _ = Describe("Git", func() {

	var (
		repositoryPath string
		repo           map[string]*git.Repository

		fakeGit        gitinterfacefakes.FakeGit
		fakeFileSystem gitfakes.FakeFileReader

		ctx      context.Context
		client   *github.Client
		teardown func()
		cancel   context.CancelFunc

		gh resourcehandlers.ResourceHandler

		localMappings map[string]string

		err error
	)

	BeforeEach(func() {
		localMappings = make(map[string]string)
	})

	JustBeforeEach(func() {
		//creating git resource handler
		ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)

		var (
			muxRes *http.ServeMux
		)
		client, muxRes, _, teardown = setup()
		if mux != nil {
			mux(muxRes)
		}

		repo = make(map[string]*git.Repository)
		repo[repositoryPath] = &git.Repository{
			State:     git.Prepared,
			Git:       &fakeGit,
			LocalPath: repositoryPath,
		}
		gh = git.NewResourceHandlerTest("", nil, "", client, nil, nil, localMappings, &fakeGit, repo, &fakeFileSystem, nil)

	})
	JustAfterEach(func() {
		cancel()
		teardown()
	})
	Describe("BuildAbsLink", func() {
		var (
			source string
			link   string

			got  string
			want string
			err  error
		)

		JustBeforeEach(func() {

			got, err = gh.BuildAbsLink(source, link)
		})

		Describe("test nested relative link", func() {
			BeforeEach(func() {
				source = "https://github.com/gardener/gardener/tree/master/readme.md"
				link = "jjbj.md"
				want = "https://github.com/gardener/gardener/tree/master/jjbj.md"

				var info gitfakes.FakeFileInfo
				info.IsDirReturns(true)
				fakeFileSystem.StatReturns(&info, nil)
			})

			It("should process it correctly", func() {
				Expect(got).Should(Equal(want))
				Expect(err).NotTo(HaveOccurred())
			})

			Describe("using local mappings", func() {
				BeforeEach(func() {
					localMappings["https://github.com/gardener"] = "/remapped"

				})

				It("should process it correctly", func() {
					Expect(got).Should(Equal(want))
					Expect(err).NotTo(HaveOccurred())
				})
			})
		})

		Describe("test outside link", func() {
			BeforeEach(func() {
				source = "https://github.com/gardener/gardener/tree/master/docs/extensions/readme.md"
				link = "../../images/jjbj.png"
				want = "https://github.com/gardener/gardener/tree/master/images/jjbj.png"

				var info gitfakes.FakeFileInfo
				info.IsDirReturns(true)
				fakeFileSystem.StatReturns(&info, nil)
			})

			It("should process it correctly", func() {
				Expect(got).Should(Equal(want))
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Describe("test outside link with wrong source type", func() {
			When("used blob instead of tree", func() {
				BeforeEach(func() {
					source = "https://github.com/gardener/gardener/blob/master/docs/extensions/"
					link = "../../images"
					want = "https://github.com/gardener/gardener/tree/master/images"

					var info gitfakes.FakeFileInfo
					info.IsDirReturns(true)
					fakeFileSystem.StatReturns(&info, nil)
				})

				It("should process it correctly and return the right type ", func() {
					Expect(got).Should(Equal(want))
					Expect(err).NotTo(HaveOccurred())
				})
			})
			When("used tree instead of blob", func() {
				BeforeEach(func() {
					source = "https://github.com/gardener/gardener/tree/master/docs/extensions/readme.md"
					link = "../../images/jjbj.png"
					want = "https://github.com/gardener/gardener/blob/master/images/jjbj.png"

					var info gitfakes.FakeFileInfo
					info.IsDirReturns(false)
					fakeFileSystem.StatReturns(&info, nil)
				})

				It("should process it correctly and return the right type ", func() {
					Expect(got).Should(Equal(want))
					Expect(err).NotTo(HaveOccurred())
				})
			})
		})

		Describe("test root link", func() {
			BeforeEach(func() {
				source = "https://github.com/gardener/external-dns-management/blob/master/README.md"
				link = "/docs/aws-route53/README.md"
				want = "https://github.com/gardener/external-dns-management/blob/master/docs/aws-route53/README.md"

				var info gitfakes.FakeFileInfo
				info.IsDirReturns(false)
				fakeFileSystem.StatReturns(&info, nil)
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

				var info gitfakes.FakeFileInfo
				info.IsDirReturns(true)
				fakeFileSystem.IsNotExistReturns(true)
				fakeFileSystem.StatReturns(nil, resourcehandlers.ErrResourceNotFound(want))
			})

			It("should process it correctly", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).Should(Equal(resourcehandlers.ErrResourceNotFound(want)))
			})
		})
	})
	Describe("ReadGitInfo", func() {
		var (
			url string

			got  []byte
			want []byte
			err  error
		)

		JustBeforeEach(func() {
			got, err = gh.ReadGitInfo(ctx, url)
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
			})

			It("should process it correctly", func() {

				Expect(err).NotTo(HaveOccurred())
				Expect(got).Should(Equal(want))
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
				link = "http://github.com/index.html?page=1"
			})

			It("should return resource and extention", func() {
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

	Describe("SetVersion", func() {
		var (
			absLink string
			version string
			got     string
			err     error
		)

		JustBeforeEach(func() {
			got, err = gh.SetVersion(absLink, version)
		})

		BeforeEach(func() {
			version = "v1"
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

		When("version not set", func() {
			BeforeEach(func() {
				absLink = "http://github.com/gardener/docforge"
			})
			It("should not change absLink", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(got).Should(Equal(absLink))
			})
		})

		When("version is set", func() {
			BeforeEach(func() {
				absLink = "http://github.com/gardener/docforge/blob/master/res"
			})
			It("should return the proper link", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(got).Should(Equal("http://github.com/gardener/docforge/blob/v1/res"))
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
			gh = git.NewResourceHandler("", nil, "", nil, nil, acceptedHosts, nil)
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

	Describe("Resolving node selector", func() {
		var (
			node         *api.Node
			excludePaths []string
			depth        int32

			got      []*api.Node
			expected []*api.Node
		)

		BeforeEach(func() {
			var (
				fakeRepository gitinterfacefakes.FakeRepository
				fakeWorktree   gitinterfacefakes.FakeRepositoryWorktree
				fakeFileInfo   gitfakes.FakeFileInfo
			)

			fakeGit.PlainOpenReturns(&fakeRepository, nil)
			fakeRepository.WorktreeReturns(&fakeWorktree, nil)

			fakeFileSystem.IsNotExistReturns(false)
			fakeFileInfo.IsDirReturns(false)
			fakeFileSystem.StatReturns(&fakeFileInfo, nil)

			fileData := []byte(fmt.Sprintf(`
---
title: Test
---
This is test file`))

			node = &api.Node{
				NodeSelector: &api.NodeSelector{
					Path: "https://github.com/testorg/testrepo/tree/testbranch/testdir",
				},
			}

			fakeFileSystem.ReadFileReturns(fileData, nil)

			depth = 0
			excludePaths = nil

		})

		JustBeforeEach(func() {
			node.NodeSelector.ExcludePaths = excludePaths
			node.NodeSelector.Depth = depth

			var (
				//fakeFile  gitfakes.FakeFileInfo
				fakeFile1 gitfakes.FakeFileInfo
				fakeFile2 gitfakes.FakeFileInfo
				fakeFile3 gitfakes.FakeFileInfo
				fakeDir   gitfakes.FakeFileInfo
				fakeDir1  gitfakes.FakeFileInfo
				fakeDir2  gitfakes.FakeFileInfo
			)

			fakeFile1.IsDirReturns(false)
			fakeFile1.NameReturns(".config")

			fakeFile1.IsDirReturns(false)
			fakeFile1.NameReturns("testfile.md")

			fakeFile2.IsDirReturns(false)
			fakeFile2.NameReturns("testfile2.md")

			fakeFile3.IsDirReturns(false)
			fakeFile3.NameReturns("testfile3.md")

			fakeDir.IsDirReturns(true)
			fakeDir.NameReturns("")

			fakeDir1.IsDirReturns(true)
			fakeDir1.NameReturns("testdir_sub")

			fakeDir2.IsDirReturns(true)
			fakeDir2.NameReturns("testdir_sub2")
			gh = git.NewResourceHandlerTest("", nil, "", client, nil, nil, localMappings, &fakeGit, repo, &fakeFileSystem, func(root string, walkerFunc filepath.WalkFunc) error {
				walkerFunc(root, &fakeDir, nil)
				walkerFunc(filepath.Join(root, "testfile.md"), &fakeFile1, nil)
				walkerFunc(filepath.Join(root, "testdir_sub"), &fakeDir1, nil)
				walkerFunc(filepath.Join(root, "testdir_sub", "testdir_sub2"), &fakeDir2, nil)
				walkerFunc(filepath.Join(root, "testdir_sub", "testdir_sub2", "testfile3.md"), &fakeFile3, nil)
				return nil
			})

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

	Describe("Read", func() {
		var (
			uri string
			got []byte

			fakeFileInfo gitfakes.FakeFileInfo
		)

		BeforeEach(func() {

			fakeFileInfo.IsDirReturns(false)

			fakeFileSystem.IsNotExistReturns(false)
			fakeFileSystem.StatReturns(&fakeFileInfo, nil)

			readData := []byte(fmt.Sprintf(`
structure:
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
{{- end }}`))
			fakeFileSystem.ReadFileReturns(readData, nil)

		})

		JustBeforeEach(func() {
			got, err = gh.Read(ctx, uri)
		})

		Context("repo file does not exist", func() {

			BeforeEach(func() {
				uri = "https://github.com/gardener/docforge/blob/master/nores"

				fakeFileSystem.StatReturns(nil, fmt.Errorf("some err"))
				fakeFileSystem.IsNotExistReturns(true)
			})

			It("should return error ErrResourceNotFound", func() {
				Expect(err).Should(Equal(resourcehandlers.ErrResourceNotFound(uri)))
				Expect(got).Should(Equal([]byte{}))
			})
		})

		Context("repo file exists and is a directory", func() {
			BeforeEach(func() {
				uri = "https://github.com/gardener/docforge/tree/master/res"

				fakeFileInfo.IsDirReturns(true)
			})

			It("should return nil ,nil", func() {
				Expect(err).Should(BeNil())
				Expect(got).Should(BeNil())
			})
		})

		Context("repo file exists", func() {
			var want []byte

			BeforeEach(func() {
				uri = "https://github.com/gardener/docforge/blob/master/res"
				want = []byte(fmt.Sprintf(`
structure:
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
{{- end }}`))
			})

			It("should read it correctly", func() {
				Expect(got).Should(Equal(want))
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("Resolving documentation", func() {
		var (
			tags []string
			uri  string

			expected *api.Documentation
			got      *api.Documentation
		)

		BeforeEach(func() {
			var (
				fakeFileInfo gitfakes.FakeFileInfo
			)

			fakeFileSystem.IsNotExistReturns(false)
			fakeFileInfo.IsDirReturns(false)
			fakeFileSystem.StatReturns(&fakeFileInfo, nil)

			manifestData := []byte(fmt.Sprintf(`
structure:
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
{{- end }}`))
			fakeFileSystem.ReadFileReturns(manifestData, nil)

			api.SetFlagsVariables(make(map[string]string))
		})

		JustBeforeEach(func() {

			var (
				fakeRepository gitinterfacefakes.FakeRepository
				fakeWorktree   gitinterfacefakes.FakeRepositoryWorktree
			)

			fakeGit.PlainOpenReturns(&fakeRepository, nil)
			fakeRepository.TagsReturns(tags, nil)
			fakeRepository.WorktreeReturns(&fakeWorktree, nil)

			s := make(map[string]int)
			s[uri] = len(tags)
			api.SetNVersions(s, s)

			got, err = gh.ResolveDocumentation(ctx, uri)
		})

		JustAfterEach(func() {
			//clear default branch cache
			ghub.ClearDefaultBranchesCache()
		})

		Context("given the general use case", func() {
			BeforeEach(func() {
				repositoryPath = "github.com/testOrg/testRepo/testMainBranch"
				uri = "https://github.com/testOrg/testRepo/blob/DEFAULT_BRANCH/testManifest.yaml"

				tags = []string{"v4.9", "v5.7", "v6.1", "v7.7"}

				expected = &api.Documentation{
					Structure: []*api.Node{
						{
							Name:   "community",
							Source: "https://github.com/gardener/docforge/edit/master/integration-test/tested-doc/merge-test/testFile.md",
						},
						{
							Name:   "docs",
							Source: "https://github.com/gardener/docforge/blob/testMainBranch/integration-test/tested-doc/merge-test/testFile.md",
						},
						{
							Name:   "v7.7",
							Source: "https://github.com/gardener/docforge/blob/v7.7/integration-test/tested-doc/merge-test/testFile.md",
						},
						{
							Name:   "v6.1",
							Source: "https://github.com/gardener/docforge/blob/v6.1/integration-test/tested-doc/merge-test/testFile.md",
						},
						{
							Name:   "v5.7",
							Source: "https://github.com/gardener/docforge/blob/v5.7/integration-test/tested-doc/merge-test/testFile.md",
						},
						{
							Name:   "v4.9",
							Source: "https://github.com/gardener/docforge/blob/v4.9/integration-test/tested-doc/merge-test/testFile.md",
						},
					},
				}
			})

			It("should process it correctly", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(got).To(Equal(expected))
			})

			Context("and no versions", func() {
				BeforeEach(func() {
					tags = []string{}

					expected = &api.Documentation{
						Structure: []*api.Node{
							{
								Name:   "community",
								Source: "https://github.com/gardener/docforge/edit/master/integration-test/tested-doc/merge-test/testFile.md",
							},
							{
								Name:   "docs",
								Source: "https://github.com/gardener/docforge/blob/testMainBranch/integration-test/tested-doc/merge-test/testFile.md",
							},
						},
					}
				})

				It("should apply only the main branch", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(got).To(Equal(expected))
				})
			})
		})
	})
})
