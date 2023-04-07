package adocs_test

import (
	"embed"
	_ "embed"
	"testing"

	"github.com/gardener/docforge/pkg/renderers/adocs"
	. "github.com/onsi/ginkgo"

	. "github.com/onsi/gomega"
)

func TestAdoc(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Adoc Suite")
}

//go:embed tests/*
var tests embed.FS

func simpleResolve(dest string, isEmbeddable bool) (string, error) {
	return "/__resources/" + dest, nil
}

var _ = Describe("Process adoc file", func() {
	var (
		properties map[string]interface{}
		content    []byte
	)
	BeforeEach(func() {
		content, _ = tests.ReadFile("tests/example1.adoc")
	})
	When("node does not have any properties", func() {
		BeforeEach(func() {
			properties = map[string]interface{}{}
		})
		It("should repalce image destinations correctly", func() {
			expected, _ := tests.ReadFile("tests/parsed1.adoc")
			got, err := adocs.ProcessAdocContent(content, properties, simpleResolve)
			Expect(string(got)).To(Equal(string(expected)))
			Expect(err).ToNot(HaveOccurred())
		})
	})
	When("node has frontmatter", func() {
		BeforeEach(func() {
			properties = map[string]interface{}{
				"frontmatter": map[string]interface{}{
					"title":  "foo",
					"weight": 50,
				},
			}
		})
		It("should add frontmatter and repalce image destinations correctly", func() {
			expected, _ := tests.ReadFile("tests/parsed2.adoc")
			got, err := adocs.ProcessAdocContent(content, properties, simpleResolve)
			Expect(string(got)).To(Equal(string(expected)))
			Expect(err).ToNot(HaveOccurred())
		})
	})
	When("node has adocPath", func() {
		BeforeEach(func() {
			properties = map[string]interface{}{
				"adocPath": "fooPath/barPath",
			}
		})
		It("should prefix inports and repalce image destinations correctly", func() {
			expected, _ := tests.ReadFile("tests/parsed3.adoc")
			got, err := adocs.ProcessAdocContent(content, properties, simpleResolve)
			Expect(string(got)).To(Equal(string(expected)))
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
