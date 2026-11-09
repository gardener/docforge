// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package vitepress_test

import (
	"testing"

	"github.com/gardener/docforge/cmd/vitepress"
	"github.com/gardener/docforge/pkg/manifest"
)

func TestAdapterEnabled(t *testing.T) {
	cases := []struct {
		enabled bool
		want    bool
	}{
		{true, true},
		{false, false},
	}
	for _, c := range cases {
		a := vitepress.NewAdapter(vitepress.VitePress{Enabled: c.enabled})
		if got := a.Enabled(); got != c.want {
			t.Errorf("Enabled() = %v, want %v", got, c.want)
		}
	}
}

func TestAdapterIndexFileName(t *testing.T) {
	a := vitepress.NewAdapter(vitepress.VitePress{})
	if got := a.IndexFileName(); got != "index.md" {
		t.Errorf("IndexFileName() = %q, want %q", got, "index.md")
	}
}

func TestAdapterIsIndexFile(t *testing.T) {
	cases := []struct {
		name            string
		inputFile       string
		extraIndexFiles []string
		want            bool
	}{
		{"index.md is always an index file", "index.md", nil, true},
		{"INDEX.MD case-insensitive", "INDEX.MD", nil, true},
		{"Index.md case-insensitive", "Index.md", nil, true},
		// Hugo-style _index.md normalisation: external repos may ship _index.md
		{"_index.md is normalised to index.md", "_index.md", nil, true},
		{"_INDEX.MD case-insensitive normalisation", "_INDEX.MD", nil, true},
		{"README.md is not an index file by default", "README.md", nil, false},
		{"README.md via IndexFileNames", "README.md", []string{"readme.md"}, true},
		{"guide.md is not an index file", "guide.md", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := vitepress.NewAdapter(vitepress.VitePress{IndexFileNames: c.extraIndexFiles})
			if got := a.IsIndexFile(c.inputFile); got != c.want {
				t.Errorf("IsIndexFile(%q) = %v, want %v", c.inputFile, got, c.want)
			}
		})
	}
}

func TestAdapterBaseURL(t *testing.T) {
	a := vitepress.NewAdapter(vitepress.VitePress{BaseURL: "https://example.com"})
	if got := a.BaseURL(); got != "https://example.com" {
		t.Errorf("BaseURL() = %q, want %q", got, "https://example.com")
	}
}

func TestAdapterStructuralDirs(t *testing.T) {
	dirs := []string{"docs", "public"}
	a := vitepress.NewAdapter(vitepress.VitePress{VitePressStructuralDirs: dirs})
	got := a.StructuralDirs()
	if len(got) != len(dirs) {
		t.Fatalf("StructuralDirs() len = %d, want %d", len(got), len(dirs))
	}
	for i, d := range dirs {
		if got[i] != d {
			t.Errorf("StructuralDirs()[%d] = %q, want %q", i, got[i], d)
		}
	}
}

func TestAdapterPrettyPath(t *testing.T) {
	cases := []struct {
		name     string
		nodePath string
		nodeFile string
		wantPath string
	}{
		{
			"non-index file returned unchanged",
			"docs",
			"guide.md",
			"docs/guide.md",
		},
		{
			// _index.md from a Hugo-based external repo must link to the output file index.md
			"_index.md normalised to index.md",
			"docs/extensions",
			"_index.md",
			"docs/extensions/index.md",
		},
		{
			"index.md stays index.md (no-op normalisation)",
			"docs/extensions",
			"index.md",
			"docs/extensions/index.md",
		},
		{
			// README.md is an index file (via IndexFileNames) → normalised to index.md
			"README.md (via IndexFileNames) normalised to index.md",
			"docs/extensions",
			"README.md",
			"docs/extensions/index.md",
		},
		{
			"top-level _index.md (path=.) normalised to index.md",
			".",
			"_index.md",
			"index.md",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := vitepress.NewAdapter(vitepress.VitePress{
				IndexFileNames: []string{"readme.md", "README.md"},
			})
			node := &manifest.Node{
				FileType: manifest.FileType{File: c.nodeFile},
				Type:     "file",
				Path:     c.nodePath,
			}
			if got := a.PrettyPath(node); got != c.wantPath {
				t.Errorf("PrettyPath() = %q, want %q", got, c.wantPath)
			}
		})
	}
}
