// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0
package git_test

import (
	"context"
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
	"github.com/gardener/docforge/pkg/resourcehandlers/github/githubfakes"

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
	}
)

var _ = Describe("Git", func() {

	var (
		repositoryPath string

		fakeGit        gitinterfacefakes.FakeGit
		fakeFileSystem gitfakes.FakeFileReader

		ctx      context.Context
		teardown func()
		cancel   context.CancelFunc

		cache *ghub.Cache
		gh    resourcehandlers.ResourceHandler

		err error
	)

	BeforeEach(func() {
		cache = ghub.NewCache(nil)
	})

	JustBeforeEach(func() {
		//creating git resource handler
		ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)

		var (
			muxRes *http.ServeMux
			client *github.Client
		)
		client, muxRes, _, teardown = setup()
		if mux != nil {
			mux(muxRes)
		}

		repo := make(map[string]*git.Repository)
		repo[repositoryPath] = &git.Repository{
			State:     git.Prepared,
			Git:       &fakeGit,
			LocalPath: repositoryPath,
		}

		gh = git.NewResourceHandlerExtended("", nil, "", client, nil, nil, nil, &fakeGit, repo, &fakeFileSystem, cache)

	})

	JustAfterEach(func() {
		cancel()
		teardown()
	})

	Describe("resolving node selector", func() {
		var (
			node               *api.Node
			excludePaths       []string
			frontMatter        map[string]interface{}
			excludeFrontMatter map[string]interface{}
			depth              int32

			fakeTreeExtractor githubfakes.FakeTreeExtractor

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

			fakeTreeExtractorRes := []*ghub.ResourceLocator{
				&ghub.ResourceLocator{
					Scheme:   "https",
					Host:     "github.com",
					Owner:    "testorg",
					Repo:     "testrepo",
					Type:     ghub.Tree,
					SHAAlias: "testbranch",
					Path:     "testdir",
				},
				&ghub.ResourceLocator{
					Scheme:   "https",
					Host:     "github.com",
					Owner:    "testorg",
					Repo:     "testrepo",
					Type:     ghub.Blob,
					SHAAlias: "testbranch",
					Path:     "testdir/testfile.md",
				},
				&ghub.ResourceLocator{
					Scheme:   "https",
					Host:     "github.com",
					Owner:    "testorg",
					Repo:     "testrepo",
					Type:     ghub.Tree,
					SHAAlias: "testbranch",
					Path:     "testdir/testdir_sub",
				},
				&ghub.ResourceLocator{
					Scheme:   "https",
					Host:     "github.com",
					Owner:    "testorg",
					Repo:     "testrepo",
					Type:     ghub.Tree,
					SHAAlias: "testbranch",
					Path:     "testdir/testdir_sub/testdir_sub2",
				},
				&ghub.ResourceLocator{
					Scheme:   "https",
					Host:     "github.com",
					Owner:    "testorg",
					Repo:     "testrepo",
					Type:     ghub.Blob,
					SHAAlias: "testbranch",
					Path:     "testdir/testdir_sub/testdir_sub2/testfile3.md",
				},
				&ghub.ResourceLocator{
					Scheme:   "https",
					Host:     "github.com",
					Owner:    "testorg",
					Repo:     "testrepo",
					Type:     ghub.Blob,
					SHAAlias: "testbranch",
					Path:     "testfile2.md",
				},
			}
			fakeTreeExtractor.ExtractTreeReturns(fakeTreeExtractorRes, nil)

			cache = ghub.NewCache(&fakeTreeExtractor)
		})

		JustBeforeEach(func() {
			got, err = gh.ResolveNodeSelector(ctx, node, excludePaths, frontMatter, excludeFrontMatter, depth)
		})

		Context("given the general use case", func() {
			BeforeEach(func() {

				root := api.Node{
					NodeSelector: &api.NodeSelector{Path: "https://github.com/testorg/testrepo/tree/testbranch/testdir"},
				}
				testfile3 := api.NewNodeForTesting("testfile3.md", "https://github.com/testorg/testrepo/blob/testbranch/testdir/testdir_sub/testdir_sub2/testfile3.md", nil, "")
				testdir_sub2 := api.NewNodeForTesting("testdir_sub2", "", []*api.Node{&testfile3}, "https://github.com/testorg/testrepo/tree/testbranch/testdir/testdir_sub/testdir_sub2")
				testdir_sub := api.NewNodeForTesting("testdir_sub", "", []*api.Node{&testdir_sub2}, "https://github.com/testorg/testrepo/tree/testbranch/testdir/testdir_sub")
				testdir_sub.SetParent(&root)
				testfile := api.NewNodeForTesting("testfile.md", "https://github.com/testorg/testrepo/blob/testbranch/testdir/testfile.md", nil, "")
				testfile.SetParent(&root)
				testdir_sub.SetParentsDownwards()

				expected = []*api.Node{
					&testfile,
					&testdir_sub,
				}
			})

			It("should process it correctly", func() {
				Expect(err).NotTo(HaveOccurred())
				rootGot := api.Node{Nodes: got}
				rootExpected := api.Node{Nodes: expected}
				api.SortNodesByName(&rootGot)
				api.SortNodesByName(&rootExpected)
				Expect(rootGot).To(Equal(rootExpected))
			})

		})

		Context("given a depth parameter", func() {
			BeforeEach(func() {
				depth = 1

				root := api.Node{
					NodeSelector: &api.NodeSelector{Path: "https://github.com/testorg/testrepo/tree/testbranch/testdir"},
				}
				testdir_sub := api.NewNodeForTesting("testdir_sub", "", []*api.Node{}, "https://github.com/testorg/testrepo/tree/testbranch/testdir/testdir_sub")
				testdir_sub.SetParent(&root)
				testfile := api.NewNodeForTesting("testfile.md", "https://github.com/testorg/testrepo/blob/testbranch/testdir/testfile.md", nil, "")
				testfile.SetParent(&root)
				testdir_sub.SetParentsDownwards()

				expected = []*api.Node{
					&testfile,
					&testdir_sub,
				}

			})

			It("should process it correctly", func() {
				Expect(err).NotTo(HaveOccurred())
				rootGot := api.Node{Nodes: got}
				rootExpected := api.Node{Nodes: expected}
				api.SortNodesByName(&rootGot)
				api.SortNodesByName(&rootExpected)
				Expect(rootGot).To(Equal(rootExpected))
			})

		})

		Context("given a excludePath parameter", func() {
			BeforeEach(func() {
				excludePaths = []string{"testdir_sub2", "testfile.md"}

				root := api.Node{
					NodeSelector: &api.NodeSelector{Path: "https://github.com/testorg/testrepo/tree/testbranch/testdir"},
				}
				testdir_sub := api.NewNodeForTesting("testdir_sub", "", []*api.Node{}, "https://github.com/testorg/testrepo/tree/testbranch/testdir/testdir_sub")
				testdir_sub.SetParent(&root)
				testdir_sub.SetParentsDownwards()

				expected = []*api.Node{
					&testdir_sub,
				}
			})

			It("should process it correctly", func() {
				Expect(err).NotTo(HaveOccurred())
				rootGot := api.Node{Nodes: got}
				rootExpected := api.Node{Nodes: expected}
				api.SortNodesByName(&rootGot)
				api.SortNodesByName(&rootExpected)
				Expect(rootGot).To(Equal(rootExpected))
			})

		})

		Context("given a excludeFrontMatter parameter", func() {
			BeforeEach(func() {
				excludeFrontMatter = make(map[string]interface{})
				excludeFrontMatter[".title"] = "Test"
				testdir_sub := api.NewNodeForTesting("testdir_sub", "", []*api.Node{}, "https://github.com/testorg/testrepo/tree/testbranch/testdir/testdir_sub")
				root := api.Node{
					NodeSelector: &api.NodeSelector{Path: "https://github.com/testorg/testrepo/tree/testbranch/testdir"},
				}
				testdir_sub.SetParent(&root)
				testdir_sub.SetParentsDownwards()

				expected = []*api.Node{
					&testdir_sub,
				}
			})

			It("should process it correctly", func() {
				Expect(err).NotTo(HaveOccurred())
				rootGot := api.Node{Nodes: got}
				rootExpected := api.Node{Nodes: expected}
				api.SortNodesByName(&rootGot)
				api.SortNodesByName(&rootExpected)
				Expect(rootGot).To(Equal(rootExpected))
			})

		})

		Context("given a frontMatter parameter", func() {
			BeforeEach(func() {
				frontMatter = make(map[string]interface{})
				frontMatter[".title"] = "broken Test"

				testdir_sub := api.NewNodeForTesting("testdir_sub", "", []*api.Node{}, "https://github.com/testorg/testrepo/tree/testbranch/testdir/testdir_sub")
				root := api.Node{
					NodeSelector: &api.NodeSelector{Path: "https://github.com/testorg/testrepo/tree/testbranch/testdir"},
				}
				testdir_sub.SetParent(&root)
				testdir_sub.SetParentsDownwards()

				expected = []*api.Node{
					&testdir_sub,
				}
			})

			It("should process it correctly", func() {
				Expect(err).NotTo(HaveOccurred())
				rootGot := api.Node{Nodes: got}
				rootExpected := api.Node{Nodes: expected}
				api.SortNodesByName(&rootGot)
				api.SortNodesByName(&rootExpected)
				Expect(rootGot).To(Equal(rootExpected))
			})

		})

	})

	Describe("resolving documentation", func() {
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
							&api.Node{
								Name:   "community",
								Source: "https://github.com/gardener/docforge/edit/master/integration-test/tested-doc/merge-test/testFile.md",
							},
							&api.Node{
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
