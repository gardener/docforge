// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved.
// This file is licensed under the Apache Software License, v.2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package api

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

var b = []byte(`
root:
  name: root
  nodes:
  - name: node_1
    contentSelectors:
    - source: path1/**
  - name: node_2
    contentSelectors:
    - source: https://a.com
    properties:
      "custom_key": custom_value
    localityDomain:
      github.com/gardener/gardener:
        exclude:
        - a
    nodes:
    - name: subnode
      contentSelectors:	
      - source: path/a
localityDomain:
  github.com/gardener/gardener:
    version: v1.10.0
    path: gardener/gardener/docs
    LinkSubstitutes:
      a: b
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
		traverse(got.Root)
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
				Root: &Node{
					Title: "A Title",
					Nodes: []*Node{
						{
							Title:            "node 1",
							ContentSelectors: []ContentSelector{{Source: "path1/**"}},
						},
						{
							Title:            "path 2",
							ContentSelectors: []ContentSelector{{Source: "https://a.com"}},
							Properties: map[string]interface{}{
								"custom_key": "custom_value",
							},
							Nodes: []*Node{
								{
									Title:            "subnode",
									ContentSelectors: []ContentSelector{{Source: "path/a"}},
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
		Root: &Node{
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
		Root: &Node{
			Name: "00",
			Nodes: []*Node{
				&Node{
					Name: "01",
					ContentSelectors: []ContentSelector{
						ContentSelector{
							Source: "https://github.com/gardener/gardener/blob/master/docs/concepts/gardenlet.md",
						},
					},
					LocalityDomain: &LocalityDomain{
						LocalityDomainMap: LocalityDomainMap{
							"github.com/gardener/gardener": &LocalityDomainValue{
								Version: "v1.11.1",
								Path:    "gardener/gardener",
								LinksMatchers: LinksMatchers{
									Exclude: []string{
										"example",
									},
								},
							},
						},
					},
				},
				&Node{
					Name: "02",
					ContentSelectors: []ContentSelector{
						ContentSelector{
							Source: "https://github.com/gardener/gardener/blob/master/docs/deployment/deploy_gardenlet.md",
						},
					},
				},
			},
		},
		LocalityDomain: &LocalityDomain{
			LocalityDomainMap: LocalityDomainMap{
				"github.com/gardener/gardener": &LocalityDomainValue{
					Version: "v1.10.0",
					Path:    "gardener/gardener",
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
