// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
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
					&Node{
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
			&Node{
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
			&Node{
				Name: "00",
				Nodes: []*Node{
					&Node{
						Name:   "01",
						Source: "https://github.com/gardener/gardener/blob/master/docs/concepts/gardenlet.md",
						Links: &Links{
							Rewrites: map[string]*LinkRewriteRule{
								"github.com/gardener/gardener": &LinkRewriteRule{
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
		},
		Links: &Links{
			Rewrites: map[string]*LinkRewriteRule{
				"github.com/gardener/gardener": &LinkRewriteRule{
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

func TestGetLastNVersions(t *testing.T) {
	tests := []struct {
		inputTags  []string
		inputN     int
		outputTags []string
		err        error
	}{
		{
			inputTags:  nil,
			inputN:     -7,
			outputTags: nil,
			err:        fmt.Errorf("n can't be negative"),
		}, {
			inputTags:  []string{},
			inputN:     0,
			outputTags: []string{},
			err:        nil,
		}, {
			inputTags:  []string{},
			inputN:     2,
			outputTags: nil,
			err:        fmt.Errorf("number of tags is greater than the actual number of tags with latest patch:requested %d actual %d", 2, 0),
		}, {
			inputTags:  nil,
			inputN:     1,
			outputTags: nil,
			err:        fmt.Errorf("number of tags is greater than the actual number of tags with latest patch:requested %d actual %d", 1, 0),
		}, {
			inputTags:  []string{"v1.2.3", "v1.2.1"},
			inputN:     1,
			outputTags: []string{"v1.2.3"},
			err:        nil,
		}, {
			inputTags:  []string{"v1.2.3", "v1.2.8"},
			inputN:     1,
			outputTags: []string{"v1.2.8"},
			err:        nil,
		}, {
			inputTags:  []string{"v1.2.3", "v1.2.8.0"},
			inputN:     1,
			outputTags: nil,
			err:        fmt.Errorf("Error parsing version: v1.2.8.0"),
		}, {
			inputTags:  []string{"v1.2.3", "v1.2.8", "v1.1.5", "v1.1.0", "v1.1.3", "v2.0.1", "v2.0.8", "v2.1.0", "v2.0.6"},
			inputN:     4,
			outputTags: []string{"v1.1.5", "v1.2.8", "v2.0.8", "v2.1.0"},
			err:        nil,
		}, {
			inputTags:  []string{"v1.2.3", "v1.2.8", "v1.1.5", "v1.1.0", "v1.1.3", "v2.0.1", "v2.0.8", "v2.1.0", "v2.0.6"},
			inputN:     5,
			outputTags: nil,
			err:        fmt.Errorf("number of tags is greater than the actual number of tags with latest patch:requested %d actual %d", 5, 4),
		}, {
			inputTags:  []string{"1.2.3", "1.2.8", "1.1.5", "1.1.0", "1.1.3", "2.0.1", "2.0.8", "2.1.0", "2.0.6"},
			inputN:     4,
			outputTags: []string{"1.1.5", "1.2.8", "2.0.8", "2.1.0"},
			err:        nil,
		}, {
			inputTags:  []string{"1.2.3", "1.2.8", "1.1.5", "1.1.0", "1.1.3", "2.0.1", "2.0.8", "2.1.0", "2.0.6"},
			inputN:     3,
			outputTags: []string{"1.2.8", "2.0.8", "2.1.0"},
			err:        nil,
		},
	}
	for _, test := range tests {
		result, resultErr := getLastNVersions(test.inputTags, test.inputN)

		if !reflect.DeepEqual(result, test.outputTags) {
			t.Errorf("Expected and actual result differ respectively: %s %s", test.outputTags, result)
		}
		if !compareErrors(resultErr, test.err) {
			t.Errorf("Expected and actual errors differ respectively: %s %s", test.err, resultErr)
		}

	}
}

func TestParseWithMetadata(t *testing.T) {
	cases := []struct {
		tags []string
		b    []byte
		uri  string
		want *Documentation
		err  error
	}{
		{
			[]string{"v4.9", "v5.7", "v6.1", "v7.7"},
			[]byte(`structure:
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
{{- end }}`),
			"https://github.com/Kostov6/documentation/blob/master/.docforge/test.yamls",
			&Documentation{
				Structure: []*Node{
					&Node{
						Name:           "community",
						Source:         "https://github.com/gardener/docforge/edit/master/integration-test/tested-doc/merge-test/testFile.md",
						sourceLocation: "",
					},
					&Node{
						Name:           "docs",
						Source:         "https://github.com/gardener/docforge/blob/master/integration-test/tested-doc/merge-test/testFile.md",
						sourceLocation: "",
					},
					&Node{
						Name:           "v4.9",
						Source:         "https://github.com/gardener/docforge/blob/v4.9/integration-test/tested-doc/merge-test/testFile.md",
						sourceLocation: "",
					},
					&Node{
						Name:           "v5.7",
						Source:         "https://github.com/gardener/docforge/blob/v5.7/integration-test/tested-doc/merge-test/testFile.md",
						sourceLocation: "",
					},
					&Node{
						Name:           "v6.1",
						Source:         "https://github.com/gardener/docforge/blob/v6.1/integration-test/tested-doc/merge-test/testFile.md",
						sourceLocation: "",
					},
					&Node{
						Name:           "v7.7",
						Source:         "https://github.com/gardener/docforge/blob/v7.7/integration-test/tested-doc/merge-test/testFile.md",
						sourceLocation: "",
					},
				},
			},
			nil,
		},
		{
			[]string{"v4.9", "v5.7"},
			[]byte(`structure:
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
{{- end }}`),
			"https://github.com/Kostov6/documentation/blob/master/.docforge/test.yamls",
			&Documentation{
				Structure: []*Node{
					&Node{
						Name:           "community",
						Source:         "https://github.com/gardener/docforge/edit/master/integration-test/tested-doc/merge-test/testFile.md",
						sourceLocation: "",
					},
					&Node{
						Name:           "docs",
						Source:         "https://github.com/gardener/docforge/blob/master/integration-test/tested-doc/merge-test/testFile.md",
						sourceLocation: "",
					},
					&Node{
						Name:           "v4.9",
						Source:         "https://github.com/gardener/docforge/blob/v4.9/integration-test/tested-doc/merge-test/testFile.md",
						sourceLocation: "",
					},
					&Node{
						Name:           "v5.7",
						Source:         "https://github.com/gardener/docforge/blob/v5.7/integration-test/tested-doc/merge-test/testFile.md",
						sourceLocation: "",
					},
				},
			},
			nil,
		},
		{
			[]string{},
			[]byte(`structure:
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
{{- end }}`),
			"https://github.com/Kostov6/documentation/blob/master/.docforge/test.yamls",
			&Documentation{
				Structure: []*Node{
					&Node{
						Name:           "community",
						Source:         "https://github.com/gardener/docforge/edit/master/integration-test/tested-doc/merge-test/testFile.md",
						sourceLocation: "",
					},
					&Node{
						Name:           "docs",
						Source:         "https://github.com/gardener/docforge/blob/master/integration-test/tested-doc/merge-test/testFile.md",
						sourceLocation: "",
					},
				},
			},
			nil,
		},
	}
	v := map[string]int{}
	vars := map[string]string{}

	SetFlagsVariables(vars)
	for _, c := range cases {
		v["https://github.com/Kostov6/documentation/blob/master/.docforge/test.yamls"] = len(c.tags)
		SetVersions(v)
		got, gotErr := ParseWithMetadata(c.tags, c.b, false, c.uri, "master")
		assert.Equal(t, c.err, gotErr)
		assert.Equal(t, c.want, got)
	}
}

func compareErrors(e1, e2 error) bool {
	switch {
	case e1 == nil && e2 == nil:
		return true
	case e1 == nil && e2 != nil:
		return false
	case e1 != nil && e2 == nil:
		return false
	default:
		return e1.Error() == e2.Error()
	}
}
