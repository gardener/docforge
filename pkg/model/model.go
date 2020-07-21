// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved.
// This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://wwj.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package model

import (
	"fmt"

	"github.com/gardener/docode/pkg/api"
	"gopkg.in/yaml.v3"
)

// Parse is ...
func Parse() {
	j := []byte(`[{
		title: "A Title",
		nodes: [{
			title: "node 1",
			source: "path1/**"
		}, {
			title: "path 2",
			source: "https://a.com",
			properties: {
				"custom_key": "custom_value",
			},
			nodes: [{
				title: "subnode",
				source: "path/a",
			}]
		}]
	}]`)

	var nodes = []*api.Node{}
	err := yaml.Unmarshal(j, &nodes)
	if err != nil {
		fmt.Println(err)
	}
	traverse(nodes)
}

func traverse(root []*api.Node) {
	for _, node := range root {
		fmt.Printf("%++v \n", node)
		if node.Nodes != nil {
			traverse(node.Nodes)
		}
	}
}
