package repositoryhost_test

// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

import (
	"context"
	"errors"

	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func testRepositoryHost(ghc repositoryhost.Interface) {
	Describe("#Tree", func() {
		It("should return error when a non tree url is given ", func() {
			resourceURl, err := ghc.ResourceURL("https://github.com/gardener/docforge/blob/master/README.md")
			Expect(err).NotTo(HaveOccurred())
			_, err = ghc.Tree(*resourceURl)
			Expect(err.Error()).To(ContainSubstring("expected a tree url got https://github.com/gardener/docforge/blob/master/README.md"))
		})

		It("should list all files", func() {
			resourceURl, err := ghc.ResourceURL("https://github.com/gardener/docforge/tree/master/pkg")
			Expect(err).NotTo(HaveOccurred())
			tree, err := ghc.Tree(*resourceURl)
			Expect(tree).To(ContainElements("api/type.go", "main.go"))
			Expect(err).NotTo(HaveOccurred())

		})
		It("should list the proper files", func() {
			resourceURl, err := ghc.ResourceURL("https://github.com/gardener/docforge/tree/master/docs")
			Expect(err).NotTo(HaveOccurred())
			tree, err := ghc.Tree(*resourceURl)
			Expect(tree).To(ContainElements("index.md", "section/page.md"))
			Expect(err).NotTo(HaveOccurred())

		})
	})

	Describe("#ResolveRelativeLink", func() {
		It("resolving relative tree path", func() {
			resourceURl, err := ghc.ResourceURL("https://github.com/gardener/docforge/tree/master/docs")
			Expect(err).NotTo(HaveOccurred())
			link, err := ghc.ResolveRelativeLink(*resourceURl, "../pkg")
			Expect(link).To(Equal("https://github.com/gardener/docforge/tree/master/pkg"))
			Expect(err).To(Not(HaveOccurred()))
		})
		It("resolving relative blob path", func() {
			resourceURl, err := ghc.ResourceURL("https://github.com/gardener/docforge/blob/master/docs/index.md")
			Expect(err).NotTo(HaveOccurred())
			link, err := ghc.ResolveRelativeLink(*resourceURl, "/pkg/main.go")
			Expect(link).To(Equal("https://github.com/gardener/docforge/blob/master/pkg/main.go"))
			Expect(err).To(Not(HaveOccurred()))
		})
		It("resolving non-existing resource should fail", func() {
			resourceURl, err := ghc.ResourceURL("https://github.com/gardener/docforge/blob/master/docs/index.md")
			Expect(err).NotTo(HaveOccurred())
			_, err = ghc.ResolveRelativeLink(*resourceURl, "/pkg/main_test.go")
			Expect(err).To(Equal(repositoryhost.ErrResourceNotFound("/pkg/main_test.go with source https://github.com/gardener/docforge/blob/master/docs/index.md")))
		})
		It("resolving absolute link should fail", func() {
			resourceURl, err := ghc.ResourceURL("https://github.com/gardener/docforge/blob/master/docs/index.md")
			Expect(err).NotTo(HaveOccurred())
			_, err = ghc.ResolveRelativeLink(*resourceURl, "https://github.com/gardener/docforge/blob/master/pkg/main_test.go")
			Expect(err).To(Equal(errors.New("expected relative link, got https://github.com/gardener/docforge/blob/master/pkg/main_test.go")))
		})

	})

	Describe("#Read", func() {
		Describe("md file", func() {
			It("returns correct content", func() {
				resourceURl, err := ghc.ResourceURL("https://github.com/gardener/docforge/blob/master/README.md")
				Expect(err).NotTo(HaveOccurred())
				content, err := ghc.Read(context.TODO(), *resourceURl)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(Equal("foo"))
			})

			It("reading a tree should fail", func() {
				resourceURl, err := ghc.ResourceURL("https://github.com/gardener/docforge/tree/master/pkg")
				Expect(err).NotTo(HaveOccurred())
				_, err = ghc.Read(context.TODO(), *resourceURl)
				Expect(err).To(Equal(errors.New("not a blob/raw url: https://github.com/gardener/docforge/tree/master/pkg")))
			})

			It("reading non-existent file should fail", func() {
				_, err := ghc.ResourceURL("https://github.com/gardener/docforge/blob/master/Makefilev2")
				Expect(err).To(Equal(repositoryhost.ErrResourceNotFound("https://github.com/gardener/docforge/blob/master/Makefilev2")))
			})
		})
	})
}
