package repositoryhost_test

// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/gardener/docforge/pkg/osfakes/httpclient"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	"github.com/gardener/docforge/pkg/registry/repositoryhost/repositoryhostfakes"
	"github.com/google/go-github/v43/github"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestRepositoryHost(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Repository host test")
}

var _ = Describe("Github cache test", func() {
	var (
		client httpclient.Client
	)

	rls := repositoryhostfakes.FakeRateLimitSource{}
	repositories := repositoryhostfakes.FakeRepositories{}
	git := repositoryhostfakes.FakeGit{}
	git.GetBlobRawCalls(func(ctx context.Context, s1, s2, s3 string) ([]byte, *github.Response, error) {
		if s3 == "1" {
			return []byte("foo"), nil, nil
		} else if s3 == "2" {
			githubResp := &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}
			return nil, githubResp, errors.New("not found")
		}
		return nil, nil, errors.New("wrong test file")
	})
	ghc := repositoryhost.NewGHC("testing", &rls, &repositories, &git, client, []string{"github.com"}, repositoryhost.ParsingOptions{ExtractedFilesFormats: []string{".md"}, Hugo: true})
	tree := github.Tree{
		Entries: []*github.TreeEntry{
			{
				Path: github.String("README.md"),
				Type: github.String("blob"),
				SHA:  github.String("1"),
			},
			{
				Path: github.String("Makefile"),
				Type: github.String("blob"),
				SHA:  github.String("2"),
			},
			{
				Path: github.String("pkg"),
				Type: github.String("tree"),
				SHA:  github.String("3"),
			},
			{
				Path: github.String("pkg/main.go"),
				Type: github.String("blob"),
				SHA:  github.String("4"),
			},
			{
				Path: github.String("pkg/api"),
				Type: github.String("tree"),
				SHA:  github.String("5"),
			},
			{
				Path: github.String("pkg/api/type.go"),
				Type: github.String("blob"),
				SHA:  github.String("6"),
			},
			{
				Path: github.String("docs"),
				Type: github.String("tree"),
				SHA:  github.String("7"),
			},
			{
				Path: github.String("docs/index.md"),
				Type: github.String("blob"),
				SHA:  github.String("8"),
			},
			{
				Path: github.String("docs/section"),
				Type: github.String("tree"),
				SHA:  github.String("9"),
			},
			{
				Path: github.String("docs/section/page.md"),
				Type: github.String("blob"),
				SHA:  github.String("10"),
			},
		},
	}
	git.GetTreeReturns(&tree, nil, nil)
	ghc.LoadRepository(context.TODO(), "https://github.com/gardener/docforge/blob/master/README.md")

	Describe("#GetRateLimit", func() {
		BeforeEach(func() {
			rls.RateLimitsReturns(nil, nil, errors.New("yataa error"))
		})

		It("return correct rate limit", func() {
			_, _, _, err := ghc.GetRateLimit(context.TODO())
			Expect(err).To(Equal(errors.New("yataa error")))

		})
	})

	testRepositoryHost(ghc)

	It("repository updated after loading", func() {
		resourceURl, err := ghc.ResourceURL("https://github.com/gardener/docforge/blob/master/Makefile")
		Expect(err).NotTo(HaveOccurred())
		_, err = ghc.Read(context.TODO(), *resourceURl)
		Expect(err).To(Equal(repositoryhost.ErrResourceNotFound("https://github.com/gardener/docforge/blob/master/Makefile")))
	})
})
