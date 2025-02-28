// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package markdown_test

import (
	"bytes"
	"errors"

	"github.com/gardener/docforge/pkg/nodeplugins/markdown/document/markdown"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
)

var _ = Describe("Links modifier", func() {
	var (
		lr  *linkResolver
		rnd renderer.Renderer
		md  string
		doc ast.Node
		err error
		buf *bytes.Buffer
		exp string
	)
	BeforeEach(func() {
		lr = &linkResolver{}
		rnd = markdown.NewLinkModifierRenderer(markdown.WithLinkResolver(lr.fakeLink))
		md = "## Heading level 2\n\nI really like using Markdown.\n"
		exp = md
	})
	JustBeforeEach(func() {
		doc, err = markdown.Parse(markdown.New(), []byte(md))
		Expect(err).NotTo(HaveOccurred())
		Expect(doc).NotTo(BeNil())
		buf = &bytes.Buffer{}
		err = rnd.Render(buf, []byte(md), doc)
	})
	When("Render markdown", func() {
		It("renders the markdown successfully", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(buf.Bytes()).To(Equal([]byte(exp)))
		})
	})
	When("Render markdown with auto links", func() {
		Context("email autolink", func() {
			BeforeEach(func() {
				md = "mails:\nfoo@bar.baz\n<mailto:foo@bar.example.com>\n"
				exp = md
			})
			It("does not modify email", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(buf.Bytes()).To(Equal([]byte(exp)))
			})
		})
		Context("URL autolink", func() {
			BeforeEach(func() {
				lr.dst = "https://fake.com"
				md = "links:\nhttp://foo.bar.baz\n<irc://foo.bar:2233/baz>\n<https://foo.bar.baz/test?q=hello&id=22&boolean~>\n(www.google.com/search?q=Markup+(business))\n(https://foo.bar).\n"
				exp = "links:\nhttps://fake.com\n<https://fake.com>\n<https://fake.com>\n(https://fake.com)\n(<https://fake.com>).\n"
			})
			It("modifies links", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(buf.Bytes()).To(Equal([]byte(exp)))
			})
		})
		Context("URL autolink in brackets", func() {
			BeforeEach(func() {
				lr.dst = "https://fake.com"
				md = "links:\n(www.google.com/search?q=Markup+(business))\n(https://foo.bar/baz is a link)\n(This is tested because of hugo https://foo.bar)\n"
				exp = "links:\n(https://fake.com)\n(<https://fake.com> is a link)\n(This is tested because of hugo <https://fake.com>)\n"
			})
			It("modifies links", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(buf.Bytes()).To(Equal([]byte(exp)))
			})
		})
		Context("Not an autolink", func() {
			BeforeEach(func() {
				lr.dst = "https://fake.com"
				md = "not links:\n`http://foo.bar.baz`\n```\nhttp://foo.bar.baz\n```\n"
				exp = md
			})
			It("does not modify code span or code block content", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(buf.Bytes()).To(Equal([]byte(exp)))
			})
		})
		Context("link resolve error", func() {
			BeforeEach(func() {
				lr.err = errors.New("fake-error")
				md = "link:\nhttp://foo.bar.baz\n"
			})
			It("fails to render document", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-error"))
			})
		})
	})
	When("Render markdown with links", func() {
		BeforeEach(func() {
			lr.dst = "https://fake.com"
			md = "links:\n[link](/uri \"title\")\n[link](http://example.com?foo=3#frag)\n"
			exp = "links:\n[link](https://fake.com \"title\")\n[link](https://fake.com)\n"
		})
		It("modifies links", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(buf.Bytes()).To(Equal([]byte(exp)))
		})
		Context("reference link", func() {
			BeforeEach(func() {
				md = "link:\n[foo][bar]\n\n[bar]: /url \"title\"\n"
				exp = "link:\n[foo](https://fake.com \"title\")\n\n"
			})
			It("modifies reference link", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(buf.Bytes()).To(Equal([]byte(exp)))
			})
		})
		Context("URL in brackets", func() {
			BeforeEach(func() {
				lr.dst = "https://fake.com"
				md = "links:\n([Named link](https://fake.com/foo) is named link)\n(Named link: [Named link](https://fake.com/bar))\n"
				exp = "links:\n([Named link](https://fake.com) is named link)\n(Named link: [Named link](https://fake.com))\n"
			})
			It("modifies links", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(buf.Bytes()).To(Equal([]byte(exp)))
			})
		})
		Context("link resolve error", func() {
			BeforeEach(func() {
				lr.err = errors.New("fake-error")
			})
			It("fails to render document", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-error"))
			})
		})
	})
	When("Render markdown with images", func() {
		BeforeEach(func() {
			lr.dst = "https://fake.com"
			md = "images:\n![foo](/url \"title\")\n![foo [bar](/url)](/url2)\n"
			exp = "images:\n![foo](https://fake.com \"title\")\n![foo [bar](https://fake.com)](https://fake.com)\n"
		})
		It("modifies images", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(buf.Bytes()).To(Equal([]byte(exp)))
		})
		Context("reference image", func() {
			BeforeEach(func() {
				md = "image:\n![foo][bar]\n\n[bar]: /url\n"
				exp = "image:\n![foo](https://fake.com)\n\n"
			})
			It("modifies reference image", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(buf.Bytes()).To(Equal([]byte(exp)))
			})
		})
		Context("image resolve error", func() {
			BeforeEach(func() {
				lr.err = errors.New("fake-error")
			})
			It("fails to render document", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-error"))
			})
		})
	})
	When("Render markdown with HTML links", func() {
		BeforeEach(func() {
			lr.dst = "https://fake.com"
			md = "block:\n<p>\n<a href=\"http://foo.bar\">baz</a>\n</p>\n\nrow:\nfoo <a href=\"/bar\">\n"
			exp = "block:\n<p>\n<a href=\"https://fake.com\">baz</a>\n</p>\n\nrow:\nfoo <a href=\"https://fake.com\">\n"
		})
		It("modifies HTML links", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(buf.Bytes()).To(Equal([]byte(exp)))
		})
		Context("links in comments", func() {
			BeforeEach(func() {
				md = "block:\n<!-- <p>\n<a href=\"http://foo.bar\">baz</a>\n</p> -->\nrow:\nfoo <!-- <a href=\"/bar\"> -->\n"
				exp = md
			})
			It("does not modify the links", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(buf.Bytes()).To(Equal([]byte(exp)))
			})
		})
		Context("link resolve error", func() {
			BeforeEach(func() {
				lr.err = errors.New("fake-error")
			})
			It("fails to render document", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-error"))
			})
		})
	})
	When("Render markdown with HTML images", func() {
		BeforeEach(func() {
			lr.dst = "https://fake.com"
			md = "block:\n<p>\n<img src=\"/foo\" alt=\"bar\" title=\"baz\"/>\n</p>\n\nrow:\nfoo <img src=\"/bar\" alt=\"baz\"/>\n"
			exp = "block:\n<p>\n<img src=\"https://fake.com\" alt=\"bar\" title=\"baz\"/>\n</p>\n\nrow:\nfoo <img src=\"https://fake.com\" alt=\"baz\"/>\n"
		})
		It("modifies HTML links", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(buf.Bytes()).To(Equal([]byte(exp)))
		})
		Context("images in comments", func() {
			BeforeEach(func() {
				md = "block:\n<!-- <p>\n<img src=\"/foo\" alt=\"bar\" title=\"baz\"/>\n</p> -->\nrow:\nfoo <!-- <img src=\"/bar\" alt=\"baz\"/> -->\n"
				exp = md
			})
			It("does not modify the images", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(buf.String()).To(Equal(exp))
			})
		})
		Context("image resolve error", func() {
			BeforeEach(func() {
				lr.err = errors.New("fake-error")
			})
			It("fails to render document", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-error"))
			})
		})
	})
})

type linkResolver struct {
	dst string
	err error
}

// implements markdown.ResolveLink and fakes the result
func (lr *linkResolver) fakeLink(_ string, _ bool) (string, error) {
	return lr.dst, lr.err
}
