// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gardener/docforge/cmd/hugo"
	"github.com/gardener/docforge/cmd/vitepress"
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
