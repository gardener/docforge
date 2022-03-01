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
			doc, err := api.Parse(manifest, map[string]string{})
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
	Describe("Parsing with metadata", func() {
		var (
			manifest     []byte
			targetBranch string
			got          *api.Documentation
			err          error
		)
		JustBeforeEach(func() {
			vars := map[string]string{}

			got, err = api.ParseWithMetadata(manifest, targetBranch, vars)
		})
		Context("given a general use case", func() {
			BeforeEach(func() {
				manifest = []byte(`structure:
- name: community
  source: https://github.com/gardener/docforge/edit/master/integration-test/tested-doc/merge-test/testFile.md
- name: docs
  source: https://github.com/gardener/docforge/blob/master/integration-test/tested-doc/merge-test/testFile.md
`)
				targetBranch = "master"
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
					},
				}))
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
			got, err = api.Parse(manifest, map[string]string{})
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
