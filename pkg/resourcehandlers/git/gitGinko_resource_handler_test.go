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
	manifestData = []byte(fmt.Sprintf(`structure:
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

	Describe("resolving documentation", func() {
		var (
			repositoryPath string
			tags           []string

			uri            string
			fakeGit        gitinterfacefakes.FakeGit
			fakeFileSystem gitfakes.FakeFileReader
			fakeFileInfo   gitfakes.FakeFileInfo
			got            *api.Documentation
			err            error
		)

		JustBeforeEach(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			client, muxRes, _, teardown := setup()
			repo := make(map[string]*git.Repository)

			repo[repositoryPath] = &git.Repository{
				State:     git.Prepared,
				Git:       &fakeGit,
				LocalPath: repositoryPath,
			}

			gh := git.NewResourceHandlerExtended("", nil, "", client, nil, nil, nil, &fakeGit, repo, &fakeFileSystem)

			defer teardown()
			if mux != nil {
				mux(muxRes)
			}
			var s map[string]int = make(map[string]int)
			s[uri] = len(tags)
			api.SetNVersions(s, s)
			api.SetFlagsVariables(make(map[string]string))
			//clear default branch cache
			ghub.ClearDefaultBranchesCache()
			got, err = gh.ResolveDocumentation(ctx, uri)
		})
		Context("given the general use case", func() {
			BeforeEach(func() {
				repositoryPath = "github.com/testOrg/testRepo/testMainBranch"
				//	manifestName = "testManifest.yaml"
				tags = []string{"v4.9", "v5.7", "v6.1", "v7.7"}
				uri = "https://github.com/testOrg/testRepo/blob/DEFAULT_BRANCH/testManifest.yaml"
				var (
					fakeRepository = &gitinterfacefakes.FakeRepository{}
					fakeWorktree   = &gitinterfacefakes.FakeRepositoryWorktree{}
				)

				//fakeGit.PlainCloneContextReturns(&fakeRepository, nil)
				fakeGit.PlainOpenReturns(fakeRepository, nil)
				fakeRepository.TagsReturns(tags, nil)
				fakeRepository.WorktreeReturns(fakeWorktree, nil)

				fakeFileSystem.IsNotExistReturns(false)
				fakeFileSystem.ReadFileReturns(manifestData, nil)
				fakeFileInfo.IsDirReturns(false)
				fakeFileSystem.StatReturns(&fakeFileInfo, nil)
			})

			It("should process it correctly", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(got).To(Equal(&api.Documentation{
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
				}))
			})
			Context("and no versions", func() {
				BeforeEach(func() {
					tags = []string{}
				})
				It("should apply only the main branch", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(got).To(Equal(&api.Documentation{
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
					}))
				})
			})
		})
	})
})
