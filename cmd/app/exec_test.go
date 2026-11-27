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
		name        string
		mode        string
		wantEnabled bool
		wantIsVP    bool // true = VitePress adapter, false = Hugo adapter
	}{
		// resolveSiteGenAdapter always receives a pre-validated mode from resolveSiteGenMode.
		{"hugo → Hugo enabled", "hugo", true, false},
		{"none → Hugo disabled", "none", false, false},
		{"vitepress → VitePress adapter", "vitepress", true, true},
		// Safety fallback: unknown mode behaves like "none".
		{"unknown mode → Hugo disabled", "unexpected", false, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := hugo.Hugo{}
			vp := vitepress.VitePress{}

			got := resolveSiteGenAdapter(c.mode, h, vp)

			if got.Enabled() != c.wantEnabled {
				t.Errorf("Enabled() = %v, want %v", got.Enabled(), c.wantEnabled)
			}
			_, isVP := got.(*vitepress.Adapter)
			if isVP != c.wantIsVP {
				t.Errorf("isVitePress = %v, want %v", isVP, c.wantIsVP)
			}
		})
	}
}

// TestResolveSiteGenMode covers all rows of the flag-resolution test table.
func TestResolveSiteGenMode(t *testing.T) {
	cases := []struct {
		name              string
		siteGenerator     string
		hugoSetCLI        bool
		hugoSetYAML       bool
		hugoValue         bool
		prettyURLsSetCLI  bool
		prettyURLsSetYAML bool
		wantMode          string
		wantErrContain    string   // non-empty → error expected containing this substring
		wantWarnings      []string // each entry must appear as a substring in some warning
		wantNoWarnings    bool
	}{
		{
			// Nothing set: matches master default (--hugo defaulted to true).
			name:           "nothing set → hugo (matches master default)",
			wantMode:       "hugo",
			wantNoWarnings: true,
		},
		{
			name:           "--site-generator=hugo",
			siteGenerator:  "hugo",
			wantMode:       "hugo",
			wantNoWarnings: true,
		},
		{
			name:           "--site-generator=vitepress",
			siteGenerator:  "vitepress",
			wantMode:       "vitepress",
			wantNoWarnings: true,
		},
		{
			name:           "--site-generator=none",
			siteGenerator:  "none",
			wantMode:       "none",
			wantNoWarnings: true,
		},
		{
			name:           "--site-generator=invalid → error listing allowed values",
			siteGenerator:  "invalid",
			wantErrContain: `invalid --site-generator "invalid"`,
		},
		{
			name:           "--site-generator=Hugo (uppercase) rejected: case-sensitive",
			siteGenerator:  "Hugo",
			wantErrContain: `invalid --site-generator "Hugo"`,
		},
		{
			// CLI flag: message uses --hugo prefix.
			name:         "--hugo=true (CLI) → hugo with CLI deprecation warning",
			hugoSetCLI:   true,
			hugoValue:    true,
			wantMode:     "hugo",
			wantWarnings: []string{"--hugo is deprecated"},
		},
		{
			// CLI flag false: mode is none, CLI-style warning.
			name:         "--hugo=false (CLI explicit) → none with CLI deprecation warning",
			hugoSetCLI:   true,
			hugoValue:    false,
			wantMode:     "none",
			wantWarnings: []string{"--hugo is deprecated"},
		},
		{
			// YAML key: message uses config key prefix, not --.
			name:         "YAML hugo: true → hugo with YAML deprecation warning",
			hugoSetYAML:  true,
			hugoValue:    true,
			wantMode:     "hugo",
			wantWarnings: []string{`config key "hugo" is deprecated`},
		},
		{
			// YAML key false: mode is none, YAML-style warning.
			name:         "YAML hugo: false → none with YAML deprecation warning",
			hugoSetYAML:  true,
			hugoValue:    false,
			wantMode:     "none",
			wantWarnings: []string{`config key "hugo" is deprecated`},
		},
		{
			// CLI hugo conflicts with site-generator.
			name:          "--hugo=true (CLI) + --site-generator=vitepress → conflict + CLI deprecation",
			siteGenerator: "vitepress",
			hugoSetCLI:    true,
			hugoValue:     true,
			wantMode:      "vitepress",
			wantWarnings:  []string{"--hugo is deprecated", "both --site-generator and --hugo are set"},
		},
		{
			// YAML hugo conflicts with site-generator.
			name:          "YAML hugo: true + --site-generator=vitepress → conflict + YAML deprecation",
			siteGenerator: "vitepress",
			hugoSetYAML:   true,
			hugoValue:     true,
			wantMode:      "vitepress",
			wantWarnings:  []string{`config key "hugo" is deprecated`, "both --site-generator and --hugo are set"},
		},
		{
			name:          "--hugo=false (CLI) + --site-generator=hugo → conflict + CLI deprecation",
			siteGenerator: "hugo",
			hugoSetCLI:    true,
			hugoValue:     false,
			wantMode:      "hugo",
			wantWarnings:  []string{"--hugo is deprecated", "both --site-generator and --hugo are set"},
		},
		{
			// Agree: no conflict warning, just deprecation.
			name:          "--hugo=true (CLI) + --site-generator=hugo (agree) → deprecation only, no conflict",
			siteGenerator: "hugo",
			hugoSetCLI:    true,
			hugoValue:     true,
			wantMode:      "hugo",
			wantWarnings:  []string{"--hugo is deprecated"},
		},
		{
			// Invalid site-generator is always an error even with --hugo set.
			name:           "--hugo=true (CLI) + --site-generator=invalid → error (no fallback)",
			siteGenerator:  "invalid",
			hugoSetCLI:     true,
			hugoValue:      true,
			wantErrContain: `invalid --site-generator "invalid"`,
		},
		{
			// CLI pretty-urls: CLI-style warning, mode unaffected (defaults to hugo).
			name:             "--hugo-pretty-urls (CLI) → hugo (default), CLI deprecation",
			prettyURLsSetCLI: true,
			wantMode:         "hugo",
			wantWarnings:     []string{"--hugo-pretty-urls is deprecated"},
		},
		{
			// YAML pretty-urls: YAML-style warning.
			name:              `YAML hugo-pretty-urls: true → hugo (default), YAML deprecation`,
			prettyURLsSetYAML: true,
			wantMode:          "hugo",
			wantWarnings:      []string{`config key "hugo-pretty-urls" is deprecated`},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mode, warnings, err := resolveSiteGenMode(
				c.siteGenerator,
				c.hugoSetCLI, c.hugoSetYAML, c.hugoValue,
				c.prettyURLsSetCLI, c.prettyURLsSetYAML,
			)

			if c.wantErrContain != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", c.wantErrContain)
				}
				if !strings.Contains(err.Error(), c.wantErrContain) {
					t.Errorf("error = %q, want it to contain %q", err.Error(), c.wantErrContain)
				}
				// On error: mode must be empty, no file/network ops occurred.
				if mode != "" {
					t.Errorf("on error mode = %q, want empty", mode)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if mode != c.wantMode {
				t.Errorf("mode = %q, want %q", mode, c.wantMode)
			}
			checkWarnings(t, warnings, c.wantWarnings, c.wantNoWarnings)
		})
	}
}

// checkWarnings asserts that warnings matches the expected set: each entry in
// wantWarnings must appear as a substring in some warning, wantNoWarnings must
// be satisfied, and no warning may be emitted more than once.
func checkWarnings(t *testing.T, warnings, wantWarnings []string, wantNoWarnings bool) {
	t.Helper()
	if wantNoWarnings && len(warnings) > 0 {
		t.Errorf("expected no warnings, got: %v", warnings)
	}
	for _, sub := range wantWarnings {
		found := false
		for _, w := range warnings {
			if strings.Contains(w, sub) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected a warning containing %q, got warnings: %v", sub, warnings)
		}
	}
	seen := map[string]int{}
	for _, w := range warnings {
		seen[w]++
	}
	for w, n := range seen {
		if n > 1 {
			t.Errorf("warning %q emitted %d times, want exactly once", w, n)
		}
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
