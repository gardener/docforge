// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0
package api_test

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/gardener/docforge/pkg/api"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Parser", func() {
	Describe("Parsing manifest", func() {
		DescribeTable("parsing tests", func(manifest []byte, expDoc *api.Documentation, expErr error) {
			doc, err := api.Parse(manifest)
			if expErr == nil {
				Expect(err).To(BeNil())
			} else {
				Expect(err.Error()).To(ContainSubstring(expErr.Error()))
			}
			if expDoc != nil {
				for _, n := range expDoc.Structure {
					n.SetParentsDownwards()
				}
			}
			Expect(doc).To(Equal(expDoc))
		},
			Entry("no valid documentation entry", []byte("source: https://github.com/gardener/docforge/blob/master/README.md"), nil,
				fmt.Errorf("the document structure must contains at least one of these properties: structure, nodesSelector")),
			Entry("documentation nodeSelector path is mandatory", []byte(`
                nodesSelector:
                  name: test`), nil,
				fmt.Errorf("nodesSelector under / must contains a path property")),
			Entry("node name or/and source should be set", []byte(`
                structure:
                - name: test
                  nodes:
                  - name: sub
                    properties: {index: true}`), nil,
				fmt.Errorf("node test/sub must contains at least one of these properties: source, nodesSelector, multiSource, nodes")),
			Entry("node document mandatory properties", []byte(`
                structure:
                - name: test
                  nodes:
                  - multiSource: [source1, source2]`), nil,
				fmt.Errorf("node test/ must contains at least one of these properties: source, name")),
			Entry("node is either document or container", []byte(`
                structure:
                - name: test
                  source: https://github.com/gardener/docforge/blob/master/README.md
                  nodes:
                  - name: test2
                    source: https://github.com/gardener/docforge/blob/master/README.md`), nil,
				fmt.Errorf("node /test must be categorized as a document or a container")),
			Entry("structure nodeSelector path is mandatory", []byte(`
                structure:
                - nodesSelector:
                    frontMatter:`), nil,
				fmt.Errorf("nodesSelector under / must contains a path property")),
			Entry("structure nodeSelector path is mandatory", []byte(`
                structure:
                - name: gardener-extension-shoot-dns-service
                  properties:
                    frontmatter:
                      title: DNS services
                      description: Gardener extension controller for DNS services for shoot clusters
                  nodes:
                  - nodesSelector:
                      path: https://github.com/gardener/gardener-extension-shoot-dns-service/blob/master/.docforge/manifest.yaml
                    name: selector
                - name: gardener-extension-shoot-cert-service
                  properties:
                    frontmatter:
                      title: Certificate services
                      description: Gardener extension controller for certificate services for shoot clusters
                  nodes:
                  - nodesSelector:
                      path: https://github.com/gardener/gardener-extension-shoot-cert-service/blob/master/.docforge/manifest.yaml
                    name: selector`),
				&api.Documentation{
					Structure: []*api.Node{
						{
							Name: "gardener-extension-shoot-dns-service",
							Nodes: []*api.Node{
								{
									Name: "selector",
									NodeSelector: &api.NodeSelector{
										Path: "https://github.com/gardener/gardener-extension-shoot-dns-service/blob/master/.docforge/manifest.yaml",
									},
								},
							},
							Properties: map[string]interface{}{
								"frontmatter": map[string]interface{}{
									"description": "Gardener extension controller for DNS services for shoot clusters",
									"title":       "DNS services",
								},
							},
						},
						{
							Name: "gardener-extension-shoot-cert-service",
							Nodes: []*api.Node{
								{
									Name: "selector",
									NodeSelector: &api.NodeSelector{
										Path: "https://github.com/gardener/gardener-extension-shoot-cert-service/blob/master/.docforge/manifest.yaml",
									},
								},
							},
							Properties: map[string]interface{}{
								"frontmatter": map[string]interface{}{
									"description": "Gardener extension controller for certificate services for shoot clusters",
									"title":       "Certificate services",
								},
							},
						},
					},
				}, nil),
		)
	})
	Describe("Version processing", func() {
		var (
			tags       []string
			n          int
			outputTags []string
			err        error
		)
		JustBeforeEach(func() {
			outputTags, err = api.GetLastNVersions(tags, n)
		})
		Context("given general use case input", func() {
			BeforeEach(func() {
				tags = []string{"v1.2.3", "v1.2.8", "v1.1.5", "v1.1.0", "v1.1.3", "v2.0.1", "v2.0.8", "v2.1.0", "v2.0.6"}
				n = 4
			})

			It("should process them correctly", func() {
				Expect(outputTags).To(Equal([]string{"v2.1.0", "v2.0.8", "v1.2.8", "v1.1.5"}))
				Expect(err).NotTo(HaveOccurred())
			})
		})
		Context("given versions without the v prefix", func() {
			BeforeEach(func() {
				tags = []string{"1.2.3", "1.2.8", "1.1.5", "1.1.0", "1.1.3", "2.0.1", "2.0.8", "2.1.0", "2.0.6"}
				n = 4
			})

			It("should process them correctly", func() {
				Expect(outputTags).To(Equal([]string{"2.1.0", "2.0.8", "1.2.8", "1.1.5"}))
				Expect(err).NotTo(HaveOccurred())
			})
		})
		Context("given a tag string with less versions as requested", func() {
			BeforeEach(func() {
				tags = []string{"v1.2.3", "v1.2.8", "v1.1.5", "v1.1.0", "v1.1.3", "v2.0.1", "v2.0.8", "v2.1.0", "v2.0.6"}
				n = 5
			})
			It("should return the appropriate error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("number of tags is greater than the actual number of all tags: wanted - 5, actual - 4"))
			})
		})
		Context("given a invalid version", func() {
			BeforeEach(func() {
				tags = []string{"v1.2.3", "v1.2.8.0"}
				n = 1
			})
			It("should return appropriate error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Error parsing version: v1.2.8.0"))
			})
		})
		Context("given negative number", func() {
			BeforeEach(func() {
				tags = nil
				n = -7
			})

			It("should throw error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("n can't be negative"))
			})
		})
		Context("given empty version array", func() {
			BeforeEach(func() {
				tags = []string{}
				n = 0
			})
			Context("and no num tags", func() {
				BeforeEach(func() {
					n = 0
				})
				It("should not return error", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(outputTags).To(Equal([]string{}))
				})
			})
			Context("and some num tags", func() {
				BeforeEach(func() {
					n = 2
				})
				It("should return error that the number of tags is greater than the actual number of all tags", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("number of tags is greater than the actual number of all tags: wanted - 2, actual - 0"))
				})
			})
		})
	})
	Describe("Parsing with metadata", func() {
		var (
			manifest     []byte
			tags         []string
			nVersions    int
			targetBranch string
			url          string
			got          *api.Documentation
			err          error
		)
		JustBeforeEach(func() {
			v := map[string]int{}
			vars := map[string]string{}

			api.SetFlagsVariables(vars)
			v[url] = len(tags)
			api.SetNVersions(v)
			got, err = api.ParseWithMetadata(manifest, tags, nVersions, targetBranch)
		})
		Context("given a general use case", func() {
			BeforeEach(func() {
				manifest = []byte(`structure:
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
{{- end }}`)
				tags = []string{"v4.9", "v5.7", "v5.7.5", "v6.1", "v7.7"}
				nVersions = 4
				targetBranch = "master"
				url = "https://github.com/Kostov6/documentation/blob/master/.docforge/test.yamls"
			})
			It("should work as expected", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(got).To(Equal(&api.Documentation{
					Structure: []*api.Node{
						{
							Name:   "community",
							Source: "https://github.com/gardener/docforge/edit/master/integration-test/tested-doc/merge-test/testFile.md",
						},
						{
							Name:   "docs",
							Source: "https://github.com/gardener/docforge/blob/master/integration-test/tested-doc/merge-test/testFile.md",
						},
						{
							Name:   "v7.7",
							Source: "https://github.com/gardener/docforge/blob/v7.7/integration-test/tested-doc/merge-test/testFile.md",
						},
						{
							Name:   "v6.1",
							Source: "https://github.com/gardener/docforge/blob/v6.1/integration-test/tested-doc/merge-test/testFile.md",
						},
						{
							Name:   "v5.7.5",
							Source: "https://github.com/gardener/docforge/blob/v5.7.5/integration-test/tested-doc/merge-test/testFile.md",
						},
						{
							Name:   "v4.9",
							Source: "https://github.com/gardener/docforge/blob/v4.9/integration-test/tested-doc/merge-test/testFile.md",
						},
					},
				}))
			})
			Context("and no versions are wanted", func() {
				BeforeEach(func() {
					nVersions = 0
				})
				It("should only use target branch", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(got).To(Equal(&api.Documentation{
						Structure: []*api.Node{
							{
								Name:   "community",
								Source: "https://github.com/gardener/docforge/edit/master/integration-test/tested-doc/merge-test/testFile.md",
							},
							{
								Name:   "docs",
								Source: "https://github.com/gardener/docforge/blob/master/integration-test/tested-doc/merge-test/testFile.md",
							},
						},
					}))
				})
			})
			Context("but more versions are requested than provided", func() {
				BeforeEach(func() {
					nVersions = 5
				})
				It("should return error that the number of tags is greater than the actual number of all tags", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("number of tags is greater than the actual number of all tags: wanted - 5, actual - 4"))
				})
			})
			Context("but with broken yaml manifest", func() {
				BeforeEach(func() {
					manifest = []byte(`structure:
-name: community
  source: https://github.com/gardener/docforge/edit/master/integration-test/tested-doc/merge-test/testFile.md
{{- $vers := Split .versions "," -}}
{{- range $i, $version := $vers -}}
{{- if eq $i 0  }}
- name: docs
{{- else }}
- name: {{$version}}
{{- end }}
  source: https://github.com/gardener/docforge/blob/{{$version}}/integration-test/tested-doc/merge-test/testFile.md
{{- end }}`)
				})
				It("should register the yaml error", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("yaml: line 3: mapping values are not allowed in this context"))
				})
			})
		})
		Context("but with broken template format", func() {
			BeforeEach(func() {
				manifest = []byte(`structure:
- name: community
source: https://github.com/gardener/docforge/edit/master/integration-test/tested-doc/merge-test/testFile.md
{{- $vers := Split .versions "," -}}
{{- range $i, $version := $vers -}}
{{- if eq $i 0  }}
- name: docs
{{- else }}
- name: {{$version}}
{{- end }}
source: https://github.com/gardener/docforge/blob/{{$version}}/integration-test/tested-doc/merge-test/testFile.md`)
			})
			It("should register the template error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("template: :11: unexpected EOF"))
			})
		})
	})
	Describe("Serialize", func() {
		var (
			doc *api.Documentation
			got string
			err error
			exp string
		)
		JustBeforeEach(func() {
			got, err = api.Serialize(doc)
		})
		Context("given valid documentation", func() {
			BeforeEach(func() {
				doc = &api.Documentation{
					Structure: []*api.Node{
						{
							Name: "A Title",
							Nodes: []*api.Node{
								{
									Name:   "node 1",
									Source: "https://test/source1.md",
								},
								{
									Name:        "node 2",
									MultiSource: []string{"https://multitest/a.md", "https://multitest/b.md"},
									Properties: map[string]interface{}{
										"custom_key": "custom_value",
									},
								},
								{
									Name: "path 1",
									Nodes: []*api.Node{
										{
											Name:   "subnode",
											Source: "https://test/subnode.md",
										},
									},
								},
							},
						},
					},
				}
				exp = `structure:
    - name: A Title
      nodes:
        - name: node 1
          source: https://test/source1.md
        - name: node 2
          multiSource:
            - https://multitest/a.md
            - https://multitest/b.md
          properties:
            custom_key: custom_value
        - name: path 1
          nodes:
            - name: subnode
              source: https://test/subnode.md
`
			})
			It("serialize as expected", func() {
				Expect(err).To(BeNil())
				Expect(got).To(Equal(exp))
			})
		})
	})
	Describe("Parse file", func() {
		var (
			manifest []byte
			got      *api.Documentation
			err      error
			exp      *api.Documentation
		)
		JustBeforeEach(func() {
			got, err = api.Parse(manifest)
		})
		Context("given manifest file", func() {
			BeforeEach(func() {
				manifest, err = ioutil.ReadFile(filepath.Join("testdata", "parse_test_00.yaml"))
				Expect(err).ToNot(HaveOccurred())
				exp = &api.Documentation{
					Structure: []*api.Node{
						{
							Name: "00",
							Nodes: []*api.Node{
								{
									Name:   "01",
									Source: "https://github.com/gardener/gardener/blob/master/docs/concepts/gardenlet.md",
								},
								{
									Name: "02",
									MultiSource: []string{
										"https://github.com/gardener/gardener/blob/master/docs/deployment/deploy_gardenlet.md",
									},
								},
							},
						},
					},
				}
				for _, n := range exp.Structure {
					n.SetParentsDownwards()
				}
			})
			It("parse as expected", func() {
				Expect(err).To(BeNil())
				Expect(got).To(Equal(exp))
			})
		})
	})
})
