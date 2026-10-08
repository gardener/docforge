// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gardener/docforge/cmd/hugo"
	"github.com/gardener/docforge/cmd/vitepress"
	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/osfakes/osshim"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	"github.com/gardener/docforge/pkg/sitegen"
)

func TestResolveSiteGenAdapter(t *testing.T) {
	cases := []struct {
		name          string
		siteGenerator string
		hugoEnabled   bool
		wantEnabled   bool
		wantIsVP      bool // true = VitePress adapter, false = Hugo adapter
	}{
		// Legacy fallback: --site-generator not set → honour --hugo flag as-is
		{"unset + hugo=true → Hugo enabled", "", true, true, false},
		{"unset + hugo=false → Hugo disabled", "", false, false, false},
		// Explicit --site-generator values override --hugo
		{"site-generator=hugo forces Hugo enabled", "hugo", false, true, false},
		{"site-generator=none forces Hugo disabled", "none", true, false, false},
		{"site-generator=vitepress returns VitePress adapter", "vitepress", false, true, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := hugo.Hugo{Enabled: c.hugoEnabled}
			vp := vitepress.VitePress{}

			got := resolveSiteGenAdapter(c.siteGenerator, h, vp)

			if got.Enabled() != c.wantEnabled {
				t.Errorf("Enabled() = %v, want %v", got.Enabled(), c.wantEnabled)
			}
			// Distinguish VitePress from Hugo by the index file name they produce.
			_, isVP := got.(*vitepress.Adapter)
			if isVP != c.wantIsVP {
				t.Errorf("isVitePress = %v, want %v", isVP, c.wantIsVP)
			}
		})
	}
}

func TestCleanDestination(t *testing.T) {
	tests := []struct {
		name          string
		clean         bool
		dryRun        bool
		destination   string
		setupFiles    []string
		wantErr       string
		wantFilesGone bool
	}{
		{
			name:          "clean=false: destination left untouched",
			clean:         false,
			destination:   t.TempDir(),
			setupFiles:    []string{"stale.md"},
			wantFilesGone: false,
		},
		{
			name:          "dry-run: destination left untouched even when clean=true",
			clean:         true,
			dryRun:        true,
			destination:   t.TempDir(),
			setupFiles:    []string{"stale.md"},
			wantFilesGone: false,
		},
		{
			name:          "clean=true: destination removed",
			clean:         true,
			destination:   t.TempDir(),
			setupFiles:    []string{"stale.md", "sub/other.md"},
			wantFilesGone: true,
		},
		{
			name:    "clean=true with empty destination path: returns error",
			clean:   true,
			wantErr: "--clean-destination requires --destination to be set",
		},
		{
			name:          "clean=true destination does not exist: no error",
			clean:         true,
			destination:   filepath.Join(t.TempDir(), "nonexistent"),
			wantFilesGone: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// create setup files inside destination
			for _, f := range tt.setupFiles {
				full := filepath.Join(tt.destination, f)
				if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, []byte("content"), 0644); err != nil {
					t.Fatal(err)
				}
			}

			err := cleanDestination(tt.clean, tt.dryRun, tt.destination)

			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("want error %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for _, f := range tt.setupFiles {
				full := filepath.Join(tt.destination, f)
				_, statErr := os.Stat(full)
				if tt.wantFilesGone && statErr == nil {
					t.Errorf("expected %s to be gone, but it still exists", f)
				}
				if !tt.wantFilesGone && statErr != nil {
					t.Errorf("expected %s to exist, but got: %v", f, statErr)
				}
			}
		})
	}
}

func TestValidateOutputCollisions(t *testing.T) {
	mkNode := func(nodePath, file, src string) *manifest.Node {
		return &manifest.Node{
			FileType: manifest.FileType{File: file, Source: src},
			Type:     "file",
			Path:     nodePath,
		}
	}

	// Hugo config with README.md recognised as an index file.
	hugoCfg := sitegen.SimpleConfig{
		IsEnabled:       true,
		IndexFiles:      []string{"README.md"},
		IndexFileTarget: "_index.md",
	}
	// VitePress config with README.md recognised as an index file.
	vpCfg := sitegen.SimpleConfig{
		IsEnabled:       true,
		IndexFiles:      []string{"README.md"},
		IndexFileTarget: "index.md",
	}

	cases := []struct {
		name       string
		nodes      []*manifest.Node
		cfg        sitegen.Config
		wantErr    bool
		errContain []string // substrings that must appear in the error message
	}{
		{
			name: "Hugo: _index.md and README.md both map to _index.md → collision",
			nodes: []*manifest.Node{
				mkNode("extensions", "_index.md", "https://example.com/a/_index.md"),
				mkNode("extensions", "README.md", "https://example.com/b/README.md"),
			},
			cfg:        hugoCfg,
			wantErr:    true,
			errContain: []string{"extensions/_index.md", "https://example.com/a/_index.md", "https://example.com/b/README.md"},
		},
		{
			name: "VitePress: _index.md and README.md both map to index.md → collision",
			nodes: []*manifest.Node{
				mkNode("extensions", "_index.md", "https://example.com/a/_index.md"),
				mkNode("extensions", "README.md", "https://example.com/b/README.md"),
			},
			cfg:        vpCfg,
			wantErr:    true,
			errContain: []string{"extensions/index.md"},
		},
		{
			name: "two README.md from different sources in same dir → collision",
			nodes: []*manifest.Node{
				mkNode("docs", "README.md", "https://example.com/repo1/README.md"),
				mkNode("docs", "README.md", "https://example.com/repo2/README.md"),
			},
			cfg:        nil,
			wantErr:    true,
			errContain: []string{"docs/readme.md", "repo1", "repo2"},
		},
		{
			name: "case-only difference → collision",
			nodes: []*manifest.Node{
				mkNode("docs", "README.md", "https://example.com/README.md"),
				mkNode("docs", "readme.md", "https://example.com/readme.md"),
			},
			cfg:        nil,
			wantErr:    true,
			errContain: []string{"docs/readme.md"},
		},
		{
			name: "multiple collisions: all reported in one error",
			nodes: []*manifest.Node{
				mkNode("a", "_index.md", "https://example.com/a1"),
				mkNode("a", "README.md", "https://example.com/a2"),
				mkNode("b", "_index.md", "https://example.com/b1"),
				mkNode("b", "README.md", "https://example.com/b2"),
			},
			cfg:        hugoCfg,
			wantErr:    true,
			errContain: []string{"2 output-path collision(s)"},
		},
		{
			name: "normal manifest with unique paths → no error",
			nodes: []*manifest.Node{
				mkNode("docs", "_index.md", "https://example.com/docs/_index.md"),
				mkNode("extensions", "_index.md", "https://example.com/extensions/_index.md"),
				mkNode("docs", "guide.md", "https://example.com/guide.md"),
			},
			cfg:     hugoCfg,
			wantErr: false,
		},
		{
			name: "same filename in different directories → no error",
			nodes: []*manifest.Node{
				mkNode("docs", "README.md", "https://example.com/docs/README.md"),
				mkNode("extensions", "README.md", "https://example.com/extensions/README.md"),
			},
			cfg:     nil,
			wantErr: false,
		},
		{
			name: "dir nodes are ignored",
			nodes: []*manifest.Node{
				{DirType: manifest.DirType{Dir: "docs"}, Type: "dir", Path: ""},
				{DirType: manifest.DirType{Dir: "docs"}, Type: "dir", Path: ""},
			},
			cfg:     nil,
			wantErr: false,
		},
		{
			name: "non-.md resources with same path are excluded from check",
			nodes: []*manifest.Node{
				{FileType: manifest.FileType{File: "logo.png", Source: "https://example.com/a/logo.png"}, Type: "file", Path: "images"},
				{FileType: manifest.FileType{File: "logo.png", Source: "https://example.com/b/logo.png"}, Type: "file", Path: "images"},
			},
			cfg:     nil,
			wantErr: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateOutputCollisions(c.nodes, c.cfg)
			if c.wantErr && err == nil {
				t.Fatal("expected error but got nil")
			}
			if !c.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if err != nil {
				for _, sub := range c.errContain {
					if !strings.Contains(err.Error(), sub) {
						t.Errorf("error %q does not contain %q", err.Error(), sub)
					}
				}
			}
		})
	}
}

// TestCollisionEndToEnd exercises the full ResolveManifest → validateOutputCollisions
// pipeline with a local registry (no GitHub credentials required). It proves that
// when two manifest nodes resolve to the same output path the error is returned
// before any file is written, leaving the destination directory empty.
func TestCollisionEndToEnd(t *testing.T) {
	// Register a fake host so the URL parser treats it as a valid resource host.
	repositoryhost.RegisterHost("docforge-test.local")

	dir := t.TempDir()
	destDir := filepath.Join(dir, "out")

	// Two .md files that collide in Hugo mode (both become extensions/_index.md).
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# README"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "_index.md"), []byte("# Index"), 0600); err != nil {
		t.Fatal(err)
	}

	manifestContent := `structure:
  - dir: extensions
    structure:
      - file: https://docforge-test.local/org/repo/blob/main/README.md
      - file: https://docforge-test.local/org/repo/blob/main/_index.md
`
	manifestPath := filepath.Join(dir, "manifest.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0600); err != nil {
		t.Fatal(err)
	}

	localRH := repositoryhost.NewLocal(&osshim.OsShim{},
		"https://docforge-test.local/org/repo", dir)
	r := registry.NewRegistry(localRH)

	nodes, err := manifest.ResolveManifest(
		"https://docforge-test.local/org/repo/blob/main/manifest.yaml", r,
	)
	if err != nil {
		t.Fatalf("ResolveManifest: %v", err)
	}

	hugoCfg := sitegen.SimpleConfig{
		IsEnabled:       true,
		IndexFiles:      []string{"README.md"},
		IndexFileTarget: "_index.md",
	}

	collErr := validateOutputCollisions(nodes, hugoCfg)
	if collErr == nil {
		t.Fatal("expected collision error but got nil")
	}
	t.Logf("collision error:\n%s", collErr.Error())

	// Destination must not exist (no files written before collision check).
	if _, statErr := os.Stat(destDir); !os.IsNotExist(statErr) {
		t.Errorf("destination %q should not exist after collision error", destDir)
	}

	for _, must := range []string{"extensions/_index.md", "README.md", "_index.md"} {
		if !strings.Contains(collErr.Error(), must) {
			t.Errorf("collision error does not contain %q:\n%s", must, collErr.Error())
		}
	}
}
