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
	"github.com/gardener/docforge/pkg/resourcehandlers/github"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	githubApi "github.com/google/go-github/v32/github"
)

func TestGithub(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Github Suite")
}

const (
	// baseURLPath is a non-empty Client.BaseURL path to use during tests,
	// to ensure relative URLs are used for all endpoints. See issue #752.
	baseURLPath = "/api-v3"
)

// setup sets up a test HTTP server along with a github.Client that is
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
	client = githubApi.NewClient(nil)
	url, _ := url.Parse(server.URL + baseURLPath + "/")
	client.BaseURL = url
	client.UploadURL = url

	return client, mux, server.URL, server.Close
}

var _ = Describe("Github", func() {
	Describe("getting all tags", func() {
		var (
			rl  *github.ResourceLocator
			mux func(mux *http.ServeMux)
			got []string
			err error
		)
		JustBeforeEach(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			gh := &github.GitHub{}
			client, muxGot, _, teardown := setup()
			defer teardown()
			if mux != nil {
				mux(muxGot)
			}
			gh.Client = client
			got, err = gh.GetAllTags(ctx, rl)
		})
		Context("given the general use case", func() {
			BeforeEach(func() {
				rl = &github.ResourceLocator{
					"https",
					"github.com",
					"gardener",
					"gardener",
					"",
					github.Blob,
					"",
					"master",
					false,
				}
				mux = func(mux *http.ServeMux) {
					mux.HandleFunc("/repos/gardener/gardener/git/matching-refs/tags", func(w http.ResponseWriter, r *http.Request) {
						w.Write([]byte(fmt.Sprintf(`
						[
							{
							  "ref": "refs/tags/v0.0.1"
							},
							{
							  "ref": "refs/tags/v0.1.0"
							},
							{
							  "ref": "refs/tags/v0.2.0"
							}
						]`)))
					})
				}

			})

			It("should work as expected", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(got).To(Equal([]string{"v0.0.1", "v0.1.0", "v0.2.0"}))
			})
		})
	})

	Describe("resolving documentation", func() {
		var (
			uri              string
			nDefaultVersions int
			mux              func(mux *http.ServeMux)
			got              *api.Documentation
			err              error
		)

		JustBeforeEach(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			client, muxFromSetup, _, teardown := setup()
			gh := github.NewResourceHandler(client, nil, nil)
			defer teardown()
			if mux != nil {
				mux(muxFromSetup)
			}
			var s map[string]int = make(map[string]int)
			s["default"] = nDefaultVersions
			api.SetNVersions(s, s)
			api.SetFlagsVariables(make(map[string]string))
			got, err = gh.ResolveDocumentation(ctx, uri)
		})
		Context("given the general use case", func() {
			BeforeEach(func() {
				nDefaultVersions = 4
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
							  "ref": "refs/tags/v4.9"
							},
							{
							  "ref": "refs/tags/v5.7"
							},
							{
							  "ref": "refs/tags/v6.1"
							},
							{
							  "ref": "refs/tags/v7.7"
							}
						]`)))
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
					nDefaultVersions = 0
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
