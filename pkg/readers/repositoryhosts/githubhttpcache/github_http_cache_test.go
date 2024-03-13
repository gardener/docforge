package githubhttpcache_test

// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/osfakes/httpclient"
	"github.com/gardener/docforge/pkg/osfakes/osshim"
	"github.com/gardener/docforge/pkg/readers/repositoryhosts"
	"github.com/gardener/docforge/pkg/readers/repositoryhosts/githubhttpcache"
	"github.com/gardener/docforge/pkg/readers/repositoryhosts/githubhttpcache/githubhttpcachefakes"
	"github.com/google/go-github/v43/github"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestGithubCache(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Github cache Suite")
}

var _ = Describe("Github cache test", func() {
	var (
		ghc          repositoryhosts.RepositoryHost
		rls          githubhttpcachefakes.FakeRateLimitSource
		repositories githubhttpcachefakes.FakeRepositories
		git          githubhttpcachefakes.FakeGit
		client       httpclient.Client
		os           osshim.Os
	)

	BeforeEach(func() {
		rls = githubhttpcachefakes.FakeRateLimitSource{}
		repositories = githubhttpcachefakes.FakeRepositories{}
		git = githubhttpcachefakes.FakeGit{}
	})

	JustBeforeEach(func() {
		ghc = githubhttpcache.NewGHC("testing", &rls, &repositories, &git, client, os, []string{"github.com"}, map[string]string{}, manifest.ParsingOptions{ExtractedFilesFormats: []string{".md"}, Hugo: true})
	})

	Describe("#GetRateLimit", func() {
		BeforeEach(func() {
			rls.RateLimitsReturns(nil, nil, errors.New("yataa error"))
		})

		It("return correct rate limit", func() {
			_, _, _, err := ghc.GetRateLimit(context.TODO())
			Expect(err).To(Equal(errors.New("yataa error")))

		})
	})

	Describe("#FileTreeFromURL", func() {
		It("not a tree url", func() {
			_, err := ghc.FileTreeFromURL("https://github.com/gardener/docforge/blob/master/README.md")
			Expect(err.Error()).To(ContainSubstring("not a tree url"))
		})

		Describe("not found", func() {
			BeforeEach(func() {
				resp := github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}
				git.GetTreeReturns(nil, &resp, nil)
			})

			It("not found", func() {
				_, err := ghc.FileTreeFromURL("https://github.com/gardener/docforge/tree/master/pkg")
				Expect(err.Error()).To(ContainSubstring("not found"))
			})
		})

		Describe("reading tree fails", func() {
			BeforeEach(func() {
				resp := github.Response{Response: &http.Response{StatusCode: 503}}
				git.GetTreeReturns(nil, &resp, nil)
			})

			It("not found", func() {
				_, err := ghc.FileTreeFromURL("https://github.com/gardener/docforge/tree/master/pkg")
				Expect(err.Error()).To(ContainSubstring("fails with HTTP status: 503"))
			})
		})

		Describe("reading tree succeeds", func() {
			BeforeEach(func() {
				tree := github.Tree{
					Entries: []*github.TreeEntry{
						{
							Path: github.String("/README.md"),
							Type: github.String("blob"),
						},
						{
							Path: github.String("/Makefile"),
							Type: github.String("blob"),
						},
						{
							Path: github.String("/pkg"),
							Type: github.String("tree"),
						},
						{
							Path: github.String("/pkg/main.go"),
							Type: github.String("blob"),
						},
						{
							Path: github.String("/docs/_index.md"),
							Type: github.String("blob"),
						},
					},
				}
				git.GetTreeReturns(&tree, nil, nil)
			})

			It("not found", func() {
				tree, err := ghc.FileTreeFromURL("https://github.com/gardener/docforge/tree/master/pkg")
				Expect(tree).To(Equal([]string{"README.md", "docs/_index.md"}))
				Expect(err).NotTo(HaveOccurred())

			})
		})

	})

	Describe("#ToAbsLink", func() {
		Describe("absolute link", func() {
			It("returns unmodified abs link", func() {
				url, err := ghc.ToAbsLink("https://github.com/gardener/docforge/blob/master/README.md", "https://github.com/gardener/docforge/raw/master/docs/one.png")
				Expect(err).NotTo(HaveOccurred())
				Expect(url).To(Equal("https://github.com/gardener/docforge/raw/master/docs/one.png"))
			})
		})

		Describe("relative path", func() {
			BeforeEach(func() {
				docsContent := []*github.RepositoryContent{
					{
						Name:    github.String("one.md"),
						Type:    github.String("file"),
						HTMLURL: github.String("https://github.com/gardener/docforge/blob/master/docs/one.md"),
						SHA:     github.String("123"),
					},
					{
						Name:    github.String("developer"),
						Type:    github.String("directory"),
						HTMLURL: github.String("https://github.com/gardener/docforge/tree/master/docs/developer"),
						SHA:     github.String("234"),
					},
				}
				repositories.GetContentsReturns(nil, docsContent, nil, nil)
			})

			It("returns correct abs link of a relative file", func() {
				url, err := ghc.ToAbsLink("https://github.com/gardener/docforge/blob/master/README.md", "./docs/one.md")
				Expect(err).NotTo(HaveOccurred())
				Expect(url).To(Equal("https://github.com/gardener/docforge/blob/master/docs/one.md"))
			})

			It("returns correct abs link of a relative file", func() {
				url, err := ghc.ToAbsLink("https://github.com/gardener/docforge/blob/master/foo/bar/baz/README.md", "../../docs/one.md")
				Expect(err).NotTo(HaveOccurred())
				Expect(url).To(Equal("https://github.com/gardener/docforge/blob/master/foo/docs/one.md"))
			})

			It("returns correct abs link of a directory", func() {
				url, err := ghc.ToAbsLink("https://github.com/gardener/docforge/blob/master/README.md", "../docs/developer")
				Expect(err).NotTo(HaveOccurred())
				Expect(url).To(Equal("https://github.com/gardener/docforge/tree/master/docs/developer"))

			})
		})
	})

	Describe("#Read", func() {
		Describe("md file", func() {
			BeforeEach(func() {
				byteContent := []byte("foo")
				docContent := &github.RepositoryContent{
					Content: github.String(base64.StdEncoding.EncodeToString(byteContent)),
				}
				repositories.GetContentsReturns(docContent, nil, nil, nil)
			})

			It("returns correct content", func() {
				content, err := ghc.Read(context.TODO(), "https://github.com/gardener/docforge/blob/master/README.md")
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(Equal("foo"))
			})
		})
		Describe("png file", func() {
			BeforeEach(func() {
				docsContent := []*github.RepositoryContent{
					{
						Name: github.String("logo.png"),
						SHA:  github.String("321"),
					},
				}
				repositories.GetContentsReturns(nil, docsContent, nil, nil)
				git.GetBlobRawReturns([]byte("logo_contents"), nil, nil)
			})

			It("returns correct content", func() {
				content, err := ghc.Read(context.TODO(), "https://github.com/gardener/docforge/blob/master/logo.png")
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(Equal("logo_contents"))
			})
		})
	})

	Describe("#ManifestFromURL", func() {
		BeforeEach(func() {
			byteContent := []byte("foo")
			docContent := &github.RepositoryContent{
				Content: github.String(base64.StdEncoding.EncodeToString(byteContent)),
			}
			repositories.GetContentsReturns(docContent, nil, nil, nil)
		})

		It("returns correct content", func() {
			content, err := ghc.ManifestFromURL("https://github.com/gardener/docforge/blob/master/manifest.yaml")
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(Equal("foo"))
		})

	})

	Describe("#ReadGitInfo", func() {
		BeforeEach(func() {
			time1 := time.Date(2024, time.February, 6, 13, 11, 0, 0, time.UTC)
			time2 := time.Date(2024, time.February, 7, 13, 11, 0, 0, time.UTC)
			commits := []*github.RepositoryCommit{
				{
					Author: &github.User{
						Name:  github.String("one"),
						Email: github.String("one@"),
						Type:  github.String("User"),
					},
					Commit: &github.Commit{
						Committer: &github.CommitAuthor{
							Date:  &time1,
							Name:  github.String("one"),
							Email: github.String("one@"),
						},
					},
					HTMLURL: github.String("foo"),
				},
				{
					Commit: &github.Commit{
						Committer: &github.CommitAuthor{
							Date: &time2,
						},
					},
					HTMLURL: github.String("bar"),
				},
			}
			repositories.ListCommitsReturns(commits, nil, nil)
		})

		It("returns correct git info", func() {
			content, err := ghc.ReadGitInfo(context.TODO(), "https://github.com/gardener/docforge/blob/master/README.md")
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(Equal("{\n  \"lastmod\": \"2024-02-07 13:11:00\",\n  \"publishdate\": \"2024-02-06 13:11:00\",\n  \"author\": {\n    \"name\": \"one\",\n    \"email\": \"one@\"\n  },\n  \"weburl\": \"bar\",\n  \"shaalias\": \"master\",\n  \"path\": \"README.md\"\n}"))
		})
	})

})
