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
	"testing"
)

var b = []byte(`{
	root: {
	  title: "A Title",
	  nodes: [{
		  title: "node 1",
		  source: ["path1/**"]
	    }, {
		  title: "path 2",
		  source: ["https://a.com"],
		  properties: {
			"custom_key": "custom_value",
		  },
		  nodes: [{
			title: "subnode",
			source: ["path/a"],
		  }]
	  }]
	}
  }`)

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
						&Node{
							Title:  "node 1",
							Source: []string{"path1/**"},
						},
						&Node{
							Title:  "path 2",
							Source: []string{"https://a.com"},
							Properties: map[string]interface{}{
								"custom_key": "custom_value",
							},
							Nodes: []*Node{
								&Node{
									Title:  "subnode",
									Source: []string{"path/a"},
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
