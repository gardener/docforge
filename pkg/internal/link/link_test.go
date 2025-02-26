package link_test

import (
	"testing"

	_ "embed"

	"github.com/gardener/docforge/pkg/internal/link"
	"github.com/gardener/docforge/pkg/internal/must"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

func TestLink(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Link Suite")
}

var _ = Describe("Join", func() {
	DescribeTable("should join link path elements correctly",
		func(elements []string, expected string) {
			result := must.Succeed(link.Build(elements...))
			Expect(result).To(Equal(expected))
		},
		Entry("joins multiple path elements", []string{"a", "b", "c"}, "a/b/c"),
		Entry("collapses repeating slashes", []string{"a/", "//b", "c"}, "a/b/c"),
		Entry("handles empty elements", []string{"a", "", "c"}, "a/c"),
		Entry("returns an empty string when no elements are provided", []string{}, ""),
		Entry("joins elements with a leading slash", []string{"/", "foo"}, "/foo"),
		Entry("joins elements with leading and trailing slashes", []string{"/a", "b/"}, "/a/b/"),
		Entry("joins elements with leading and trailing slashes", []string{"/", "foo/"}, "/foo/"),
		Entry("joins elements with leading and trailing slashes", []string{"/", "foo", "/"}, "/foo/"),
	)

	DescribeTable("should join URL elements correctly",
		func(elements []string, expected string) {
			result := must.Succeed(link.Build(elements...))
			Expect(result).To(Equal(expected))
		},
		Entry("joins URL", []string{"https://", "foo", "bar", "baz"}, "https://foo/bar/baz"),
		Entry("joins URL with a trailing slash", []string{"https://", "foo", "bar", "baz/"}, "https://foo/bar/baz/"),
	)
})
