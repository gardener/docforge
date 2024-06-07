package repositoryhost_test

import (
	"context"
	"time"

	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	"github.com/gardener/docforge/pkg/registry/repositoryhost/repositoryhostfakes"
	"github.com/google/go-github/v43/github"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("#ReadGitInfo", func() {
	var (
		repositories repositoryhostfakes.FakeRepositories
	)

	BeforeEach(func() {
		repositories = repositoryhostfakes.FakeRepositories{}
	})

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
		resourceURl, err := repositoryhost.NewResourceURL("https://github.com/gardener/docforge/blob/master/README.md")
		Expect(err).NotTo(HaveOccurred())
		content, err := repositoryhost.ReadGitInfo(context.TODO(), &repositories, *resourceURl)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(Equal("{\n  \"lastmod\": \"2024-02-07 13:11:00\",\n  \"publishdate\": \"2024-02-06 13:11:00\",\n  \"author\": {\n    \"name\": \"one\",\n    \"email\": \"one@\"\n  },\n  \"weburl\": \"bar\",\n  \"shaalias\": \"master\",\n  \"path\": \"README.md\"\n}"))
	})
})
