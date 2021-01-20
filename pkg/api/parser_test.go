// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/gardener/docforge/pkg/util/tests"
	"github.com/stretchr/testify/assert"
)

var b = []byte(`
structure:
- name: root
  nodes:
  - name: node_1
    contentSelectors:
    - source: path1/**
  - name: node_2
    contentSelectors:
    - source: https://a.com
    properties:
      "custom_key": custom_value
    links:
      downloads:
        scope:
          github.com/gardener/gardener: ~
    nodes:
    - name: subnode
      contentSelectors:
      - source: path/a
links:
  rewrites:
    github.com/gardener/gardener:
      version: v1.10.0
      text: b
`)

func traverse(node *Node) {
	fmt.Printf("%++v \n", node)
	if node.Nodes != nil {
		for _, node := range node.Nodes {
			traverse(node)
		}
	}
}

func TestParse(t *testing.T) {
	cases := []struct {
		in, want []byte
	}{
		{b, []byte{}},
	}
	for _, c := range cases {
		got, err := Parse(c.in)
		if err != nil {
			fmt.Println(err)
			return
		}
		for _, n := range got.Structure {
			traverse(n)
		}
		// if got != c.want {
		// 	t.Errorf("Something(%q) == %q, want %q", c.in, got, c.want)
		// }
	}
}

func TestSerialize(t *testing.T) {
	cases := []struct {
		in   *Documentation
		want string
	}{
		{
			&Documentation{
				Structure: []*Node{
					{
						Name: "A Title",
						Nodes: []*Node{
							{
								Name:             "node 1",
								ContentSelectors: []ContentSelector{{Source: "path1/**"}},
							},
							{
								Name:             "path 2",
								ContentSelectors: []ContentSelector{{Source: "https://a.com"}},
								Properties: map[string]interface{}{
									"custom_key": "custom_value",
								},
								Nodes: []*Node{
									{
										Name:             "subnode",
										ContentSelectors: []ContentSelector{{Source: "path/a"}},
									},
								},
							},
						},
					},
				},
			},
			string(b),
		},
	}
	for _, c := range cases {
		got, err := Serialize(c.in)
		fmt.Printf("%v\n", got)
		if err != nil {
			fmt.Println(err)
			return
		}
		// if got != c.want {
		// 	t.Errorf("Serialize(%v) == %q, want %q", c.in, got, c.want)
		// }
	}
}

func TestMe(t *testing.T) {
	d := &Documentation{
		Structure: []*Node{
			{
				Name: "docs",
				NodeSelector: &NodeSelector{
					Path: "https://github.com/gardener/gardener/tree/master/docs",
				},
				Nodes: []*Node{
					{
						Name: "calico",
						NodeSelector: &NodeSelector{
							Path: "https://github.com/gardener/gardener-extension-networking-calico/tree/master/docs",
						},
					},
					{
						Name: "aws",
						NodeSelector: &NodeSelector{
							Path: "https://github.com/gardener/gardener-extension-provider-aws/tree/master/docs",
						},
					},
				},
			},
		},
	}
	got, err := Serialize(d)
	fmt.Printf("%v\n", got)
	if err != nil {
		fmt.Println(err)
		return
	}
}

func TestFile(t *testing.T) {
	var (
		blob []byte
		err  error
		got  *Documentation
	)
	expected := &Documentation{
		Structure: []*Node{
			{
				Name: "00",
				Nodes: []*Node{
					{
						Name:   "01",
						Source: "https://github.com/gardener/gardener/blob/master/docs/concepts/gardenlet.md",
						Links: &Links{
							Rewrites: map[string]*LinkRewriteRule{
								"github.com/gardener/gardener": {
									Version: tests.StrPtr("v1.11.1"),
								},
							},
							Downloads: &Downloads{
								Scope: map[string]ResourceRenameRules{
									"github.com/gardener/gardener": nil,
								},
							},
						},
					},
					{
						Name: "02",
						ContentSelectors: []ContentSelector{
							{
								Source: "https://github.com/gardener/gardener/blob/master/docs/deployment/deploy_gardenlet.md",
							},
						},
					},
				},
			},
		},
		Links: &Links{
			Rewrites: map[string]*LinkRewriteRule{
				"github.com/gardener/gardener": {
					Version: tests.StrPtr("v1.10.0"),
				},
			},
			Downloads: &Downloads{
				Scope: map[string]ResourceRenameRules{
					"github.com/gardener/gardener": nil,
				},
			},
		},
	}

	if blob, err = ioutil.ReadFile(filepath.Join("testdata", "parse_test_00.yaml")); err != nil {
		t.Fatalf(err.Error())
	}
	got, err = Parse(blob)
	if err != nil {
		t.Errorf("%v\n", err)
	}
	if got != expected {
		assert.Equal(t, expected, got)
	}
}
