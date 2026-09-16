// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package hugo_test

import (
	"testing"

	pkghugo "github.com/gardener/docforge/pkg/hugo"
	"github.com/gardener/docforge/pkg/manifest"
)

func TestHugoPrettyPath(t *testing.T) {
	cases := []struct {
		name           string
		nodeType       string
		nodePath       string
		nodeFile       string
		nodeDir        string
		indexFileNames []string
		want           string
	}{
		{
			name:           "regular .md file becomes pretty path",
			nodeType:       "file",
			nodePath:       "docs",
			nodeFile:       "guide.md",
			indexFileNames: []string{"readme.md", "README.md"},
			want:           "docs/guide/",
		},
		{
			name:           "README.md stem trimmed via indexFileNames",
			nodeType:       "file",
			nodePath:       "docs",
			nodeFile:       "README.md",
			indexFileNames: []string{"readme.md", "README.md"},
			want:           "docs/",
		},
		{
			name:           "readme.md (lowercase) stem trimmed via indexFileNames",
			nodeType:       "file",
			nodePath:       "docs",
			nodeFile:       "readme.md",
			indexFileNames: []string{"readme.md", "README.md"},
			want:           "docs/",
		},
		{
			name:           "_index.md stem trimmed",
			nodeType:       "file",
			nodePath:       "docs",
			nodeFile:       "_index.md",
			indexFileNames: []string{"readme.md", "README.md"},
			want:           "docs/",
		},
		{
			name:           "non-.md file returned as-is",
			nodeType:       "file",
			nodePath:       "docs",
			nodeFile:       "logo.svg",
			indexFileNames: []string{"readme.md", "README.md"},
			want:           "docs/logo.svg",
		},
		{
			name:           "dir node gets trailing slash",
			nodeType:       "dir",
			nodePath:       "docs",
			nodeDir:        "sub",
			indexFileNames: nil,
			want:           "docs/sub/",
		},
		{
			name:           "custom index file name trimmed",
			nodeType:       "file",
			nodePath:       "section",
			nodeFile:       "index.md",
			indexFileNames: []string{"index.md"},
			want:           "section/",
		},
		{
			name:           "empty indexFileNames leaves README intact as pretty path",
			nodeType:       "file",
			nodePath:       "docs",
			nodeFile:       "README.md",
			indexFileNames: []string{},
			want:           "docs/README/",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n := &manifest.Node{
				Type: c.nodeType,
				Path: c.nodePath,
			}
			n.File = c.nodeFile
			n.Dir = c.nodeDir
			got := pkghugo.PrettyPath(n, c.indexFileNames)
			if got != c.want {
				t.Errorf("HugoPrettyPath() = %q, want %q", got, c.want)
			}
		})
	}
}
