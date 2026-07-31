// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"embed"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/gardener/docforge/pkg/gentoc"
	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	"gopkg.in/yaml.v3"
)

// update regenerates testdata golden files when set via -args -update.
var update = flag.Bool("update", false, "regenerate testdata golden files") //nolint:gochecknoglobals

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

// fixtures embeds the same test manifest/docs used in pkg/gentoc tests.
//
//go:embed all:tests/*
var cmdFixtures embed.FS //nolint:gochecknoglobals

const fixtureURLPrefix = "https://github.com/gardener/docforge"
const fixtureLocalPath = "tests"
const fixtureManifestURL = "https://github.com/gardener/docforge/blob/master/toc.yaml"

var cmdIndexNames = []string{"readme.md", "README.md", "index.md"} //nolint:gochecknoglobals

func newCmdRegistry() registry.Interface {
	rh := repositoryhost.NewLocalTest(cmdFixtures, fixtureURLPrefix, fixtureLocalPath)
	return registry.NewRegistry(rh)
}

// runGenTocInProcess exercises the Builder directly (same code path as runGenToc)
// without spawning a subprocess or touching the real GitHub API.
func runGenTocInProcess(t *testing.T, stripRoot bool) ([]byte, error) {
	t.Helper()
	reg := newCmdRegistry()
	nodes, err := manifest.ResolveManifest(fixtureManifestURL, reg)
	if err != nil {
		return nil, err
	}
	b := gentoc.NewBuilder(reg, cmdIndexNames)
	nav := b.FromNodes(context.Background(), nodes, stripRoot)
	return gentoc.Marshal(nav)
}

// --- Golden file test -------------------------------------------------------

func TestGenToc_GoldenFile(t *testing.T) {
	got, err := runGenTocInProcess(t, false)
	if err != nil {
		t.Fatalf("runGenTocInProcess: %v", err)
	}

	goldenPath := filepath.Join("testdata", "expected-toc.yaml")

	if *update {
		if err := os.WriteFile(goldenPath, got, 0644); err != nil {
			t.Fatalf("writing golden file: %v", err)
		}
		t.Logf("updated %s", goldenPath)
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading golden file %s: %v (run with -args -update to create it)", goldenPath, err)
	}

	if !bytes.Equal(got, want) {
		t.Errorf("output does not match golden file %s\n\n--- want ---\n%s\n--- got ---\n%s",
			goldenPath, want, got)
	}
}

// --- Structural invariants --------------------------------------------------

func TestGenToc_AllEntriesHaveNonEmptyTitle(t *testing.T) {
	got, err := runGenTocInProcess(t, false)
	if err != nil {
		t.Fatalf("runGenTocInProcess: %v", err)
	}

	var nav gentoc.Nav
	if err := yaml.Unmarshal(got, &nav); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	var checkEntries func(entries []*gentoc.NavEntry, path string)
	checkEntries = func(entries []*gentoc.NavEntry, path string) {
		for _, e := range entries {
			if e.Title == "" {
				t.Errorf("entry %s%s has empty title", path, e.Filename)
			}
			if e.Filename == "" {
				t.Errorf("entry under %s has empty filename", path)
			}
			checkEntries(e.Subnav, path+e.Filename+"/")
		}
	}
	checkEntries(nav.Nav, "")
}

func TestGenToc_StripRoot(t *testing.T) {
	// With stripRoot=false the top-level entries begin with the section dir name.
	// This test verifies the structural shape — not the exact YAML — so it is
	// not sensitive to golden-file regeneration.
	got, err := runGenTocInProcess(t, false)
	if err != nil {
		t.Fatalf("runGenTocInProcess: %v", err)
	}

	var nav gentoc.Nav
	if err := yaml.Unmarshal(got, &nav); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Expect top-level dirs from the fixture: Installation, Operations,
	// Troubleshooting, Reference.
	wantSections := []string{
		"Installation/README.md",
		"Operations/README.md",
		"Troubleshooting/README.md",
		"Reference/_index.md",
	}
	if len(nav.Nav) != len(wantSections) {
		t.Fatalf("top-level entries: got %d, want %d", len(nav.Nav), len(wantSections))
	}
	for i, want := range wantSections {
		if nav.Nav[i].Filename != want {
			t.Errorf("nav[%d].Filename = %q, want %q", i, nav.Nav[i].Filename, want)
		}
	}
}

func TestGenToc_ManifestTitleOverride(t *testing.T) {
	// Installation dir has frontmatter.title = "Installation Guide"; verify it
	// wins over the README's document title.
	got, err := runGenTocInProcess(t, false)
	if err != nil {
		t.Fatalf("runGenTocInProcess: %v", err)
	}

	var nav gentoc.Nav
	if err := yaml.Unmarshal(got, &nav); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(nav.Nav) == 0 {
		t.Fatal("empty nav")
	}
	installEntry := nav.Nav[0]
	if installEntry.Filename != "Installation/README.md" {
		t.Fatalf("first entry filename = %q, want Installation/README.md", installEntry.Filename)
	}
	if installEntry.Title != "Installation Guide" {
		t.Errorf("Installation title = %q, want %q", installEntry.Title, "Installation Guide")
	}
}

func TestGenToc_SubnavIntegrity(t *testing.T) {
	// Operations has a subnav with gardener-operator, which has its own subnav.
	got, err := runGenTocInProcess(t, false)
	if err != nil {
		t.Fatalf("runGenTocInProcess: %v", err)
	}

	var nav gentoc.Nav
	if err := yaml.Unmarshal(got, &nav); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	var findEntry func([]*gentoc.NavEntry, string) *gentoc.NavEntry
	findEntry = func(entries []*gentoc.NavEntry, filename string) *gentoc.NavEntry {
		for _, e := range entries {
			if e.Filename == filename {
				return e
			}
			if found := findEntry(e.Subnav, filename); found != nil {
				return found
			}
		}
		return nil
	}

	opsEntry := findEntry(nav.Nav, "Operations/README.md")
	if opsEntry == nil {
		t.Fatal("Operations/README.md not found")
	}
	if len(opsEntry.Subnav) == 0 {
		t.Fatal("Operations has no subnav")
	}

	goEntry := findEntry(opsEntry.Subnav, "Operations/gardener-operator/README.md")
	if goEntry == nil {
		t.Fatal("gardener-operator/README.md not found in Operations subnav")
	}
	// Title comes from operator-readme.md document frontmatter (no dir manifest title)
	if goEntry.Title != "Gardener Operator" {
		t.Errorf("gardener-operator title = %q, want %q", goEntry.Title, "Gardener Operator")
	}
	apiEntry := findEntry(goEntry.Subnav, "Operations/gardener-operator/api-resources.md")
	if apiEntry == nil {
		t.Fatal("api-resources.md not found in gardener-operator subnav")
	}
	if apiEntry.Title != "Api Resources" {
		t.Errorf("api-resources title = %q, want %q", apiEntry.Title, "Api Resources")
	}
}

func TestGenToc_OutputIsValidYAML(t *testing.T) {
	got, err := runGenTocInProcess(t, false)
	if err != nil {
		t.Fatalf("runGenTocInProcess: %v", err)
	}
	var v interface{}
	if err := yaml.Unmarshal(got, &v); err != nil {
		t.Errorf("output is not valid YAML: %v\n%s", err, got)
	}
}
