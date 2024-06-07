package repositoryhost_test

import (
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("URL", func() {
	var (
		r   *repositoryhost.URL
		err error
	)

	Describe("anchors with /", func() {
		BeforeEach(func() {
			r, err = repositoryhost.NewResourceURL("https://github.com/owner/repo/blob/master/docs/dev/local_setup.md#foo/bar")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should build resource.URL correctly", func() {
			Expect(r.ResourceURL()).To(Equal("https://github.com/owner/repo/blob/master/docs/dev/local_setup.md"))
			Expect(r.GetResourceSuffix()).To(Equal("#foo/bar"))
		})

	})

	Describe("image lins", func() {
		It("should build resource.URL correctly", func() {
			r, err = repositoryhost.NewResourceURL("https://raw.githubusercontent.com/owner/repo/master/images/logo.png")
			Expect(err).NotTo(HaveOccurred())
			Expect(r.String()).To(Equal("https://github.com/owner/repo/blob/master/images/logo.png"))
		})
	})

	Describe("#ResolveRelativeLink", func() {
		BeforeEach(func() {
			r, err = repositoryhost.NewResourceURL("https://github.com/owner/repo/blob/master/docs/dev/local_setup.md")
			Expect(err).NotTo(HaveOccurred())
		})

		Context("with valid relative link", func() {
			It("should resolve the link correctly", func() {
				relativeLink := "../user/getting_started.md/"
				finalTreeResource, finalBlobResource, err := r.ResolveRelativeLink(relativeLink)
				Expect(err).NotTo(HaveOccurred())
				Expect(finalTreeResource).To(Equal("https://github.com/owner/repo/blob/master/docs/user/getting_started.md"))
				Expect(finalBlobResource).To(Equal("https://github.com/owner/repo/tree/master/docs/user/getting_started.md"))
			})
			Context("anchor link", func() {
				It("anchor in source", func() {
					relativeLink := "#anchor"
					finalTreeResource, finalBlobResource, err := r.ResolveRelativeLink(relativeLink)
					Expect(err).NotTo(HaveOccurred())
					Expect(finalTreeResource).To(Equal("https://github.com/owner/repo/blob/master/docs/dev/local_setup.md#anchor"))
					Expect(finalBlobResource).To(Equal("https://github.com/owner/repo/tree/master/docs/dev/local_setup.md#anchor"))
				})

				It("outside anchor", func() {
					relativeLink := "./testing.md#anchor"
					finalTreeResource, finalBlobResource, err := r.ResolveRelativeLink(relativeLink)
					Expect(err).NotTo(HaveOccurred())
					Expect(finalTreeResource).To(Equal("https://github.com/owner/repo/blob/master/docs/dev/testing.md#anchor"))
					Expect(finalBlobResource).To(Equal("https://github.com/owner/repo/tree/master/docs/dev/testing.md#anchor"))
				})
			})

			Context("query link", func() {
				It("query in source", func() {
					relativeLink := "?foo=bar"
					finalTreeResource, finalBlobResource, err := r.ResolveRelativeLink(relativeLink)
					Expect(err).NotTo(HaveOccurred())
					Expect(finalTreeResource).To(Equal("https://github.com/owner/repo/blob/master/docs/dev/local_setup.md?foo=bar"))
					Expect(finalBlobResource).To(Equal("https://github.com/owner/repo/tree/master/docs/dev/local_setup.md?foo=bar"))
				})

				It("outside query", func() {
					relativeLink := "../../pkg/main.go?foo=bar"
					finalTreeResource, finalBlobResource, err := r.ResolveRelativeLink(relativeLink)
					Expect(err).NotTo(HaveOccurred())
					Expect(finalTreeResource).To(Equal("https://github.com/owner/repo/blob/master/pkg/main.go?foo=bar"))
					Expect(finalBlobResource).To(Equal("https://github.com/owner/repo/tree/master/pkg/main.go?foo=bar"))
				})
			})

			Context("root link", func() {
				It("root", func() {
					relativeLink := "/"
					finalTreeResource, finalBlobResource, err := r.ResolveRelativeLink(relativeLink)
					Expect(err).NotTo(HaveOccurred())
					Expect(finalTreeResource).To(Equal("https://github.com/owner/repo/blob/master"))
					Expect(finalBlobResource).To(Equal("https://github.com/owner/repo/tree/master"))
				})

				It("root link", func() {
					relativeLink := "/pkg/testing/"
					finalTreeResource, finalBlobResource, err := r.ResolveRelativeLink(relativeLink)
					Expect(err).NotTo(HaveOccurred())
					Expect(finalTreeResource).To(Equal("https://github.com/owner/repo/blob/master/pkg/testing"))
					Expect(finalBlobResource).To(Equal("https://github.com/owner/repo/tree/master/pkg/testing"))
				})
			})

			Context("with encoded URL relative link", func() {
				It("should resolve the link correctly", func() {
					encodedRelativeLink := "path%20with%20spaces/resource"
					finalTreeResource, finalBlobResource, err := r.ResolveRelativeLink(encodedRelativeLink)
					Expect(err).NotTo(HaveOccurred())
					Expect(finalTreeResource).To(Equal("https://github.com/owner/repo/blob/master/docs/dev/path%20with%20spaces/resource"))
					Expect(finalBlobResource).To(Equal("https://github.com/owner/repo/tree/master/docs/dev/path%20with%20spaces/resource"))
				})
			})
		})

		Context("with invalid relative link", func() {
			It("should return an error", func() {
				relativeLink := "::invalid::link"
				_, _, err := r.ResolveRelativeLink(relativeLink)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("with absolute link", func() {
			It("should return an error", func() {
				absoluteLink := "http://github.com/owner/repo/blob/master/another/resource"
				_, _, err := r.ResolveRelativeLink(absoluteLink)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
