// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gentoc_test

import (
	"context"
	"embed"
	"errors"
	"testing"

	"github.com/gardener/docforge/pkg/gentoc"
	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/registryfakes"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
)

//go:embed all:tests/*
var fixtures embed.FS //nolint:gochecknoglobals

var defaultIndexNames = []string{"readme.md", "README.md", "index.md"} //nolint:gochecknoglobals

// newLocalRegistry returns a registry backed by the embedded test fixtures.
func newLocalRegistry() registry.Interface {
	rh := repositoryhost.NewLocalTest(fixtures, "https://github.com/gardener/docforge", "tests")
	return registry.NewRegistry(rh)
}

// resolveNodes is a test helper that calls manifest.ResolveManifest with the local registry.
func resolveNodes(t *testing.T, reg registry.Interface, manifestURL string) []*manifest.Node {
	t.Helper()
	nodes, err := manifest.ResolveManifest(manifestURL, reg)
	if err != nil {
		t.Fatalf("ResolveManifest(%s): %v", manifestURL, err)
	}
	return nodes
}

// entryByFilename finds a NavEntry (at any depth) by its filename.
func entryByFilename(entries []*gentoc.NavEntry, filename string) *gentoc.NavEntry {
	for _, e := range entries {
		if e.Filename == filename {
			return e
		}
		if found := entryByFilename(e.Subnav, filename); found != nil {
			return found
		}
	}
	return nil
}

// --- Tests ------------------------------------------------------------------

func TestTitle_ManifestFrontmatterWins(t *testing.T) {
	// Installation dir has frontmatter.title = "Installation Guide".
	// Its README.md has document frontmatter.title = "Installation Overview".
	// Manifest wins → dir entry title = "Installation Guide".
	reg := newLocalRegistry()
	nodes := resolveNodes(t, reg, "https://github.com/gardener/docforge/blob/master/toc.yaml")
	b := gentoc.NewBuilder(reg, defaultIndexNames)
	nav := b.FromNodes(context.Background(), nodes, false)

	entry := entryByFilename(nav.Nav, "Installation/README.md")
	if entry == nil {
		t.Fatal("expected entry for Installation/README.md, got nil")
	}
	if entry.Title != "Installation Guide" {
		t.Errorf("title = %q, want %q", entry.Title, "Installation Guide")
	}
}

func TestTitle_DocFrontmatterWinsOverFilename(t *testing.T) {
	// quickstart.md has no manifest frontmatter.title but has document title "Quick Start Guide".
	reg := newLocalRegistry()
	nodes := resolveNodes(t, reg, "https://github.com/gardener/docforge/blob/master/toc.yaml")
	b := gentoc.NewBuilder(reg, defaultIndexNames)
	nav := b.FromNodes(context.Background(), nodes, false)

	entry := entryByFilename(nav.Nav, "Installation/quickstart.md")
	if entry == nil {
		t.Fatal("expected entry for Installation/quickstart.md, got nil")
	}
	if entry.Title != "Quick Start Guide" {
		t.Errorf("title = %q, want %q", entry.Title, "Quick Start Guide")
	}
}

func TestTitle_FilenameDerivationFallback(t *testing.T) {
	// advanced.md has no manifest frontmatter and no document frontmatter.
	// Derived from filename: "Advanced"
	reg := newLocalRegistry()
	nodes := resolveNodes(t, reg, "https://github.com/gardener/docforge/blob/master/toc.yaml")
	b := gentoc.NewBuilder(reg, defaultIndexNames)
	nav := b.FromNodes(context.Background(), nodes, false)

	entry := entryByFilename(nav.Nav, "Installation/advanced.md")
	if entry == nil {
		t.Fatal("expected entry for Installation/advanced.md, got nil")
	}
	if entry.Title != "Advanced" {
		t.Errorf("title = %q, want %q", entry.Title, "Advanced")
	}
}

func TestTitle_IndexMd_UsesParentDirName(t *testing.T) {
	// Reference/_index.md: no manifest title on the dir, document has "Reference".
	// Dir entry title is derived from dir node (no manifest FM on dir), then from
	// dir doc FM (nil for dirs), then from dir name: "Reference".
	reg := newLocalRegistry()
	nodes := resolveNodes(t, reg, "https://github.com/gardener/docforge/blob/master/toc.yaml")
	b := gentoc.NewBuilder(reg, defaultIndexNames)
	nav := b.FromNodes(context.Background(), nodes, false)

	entry := entryByFilename(nav.Nav, "Reference/_index.md")
	if entry == nil {
		t.Fatal("expected entry for Reference/_index.md, got nil")
	}
	if entry.Title != "Reference" {
		t.Errorf("title = %q, want %q", entry.Title, "Reference")
	}
}

func TestTitle_IndexMdWithoutSource_NoPanic(t *testing.T) {
	// _index.md nodes with no source must not cause a registry.Read panic.
	// They fall back to manifest FM > filename derivation.
	// The Reference dir has no manifest frontmatter.title → derives "Reference".
	reg := newLocalRegistry()
	nodes := resolveNodes(t, reg, "https://github.com/gardener/docforge/blob/master/toc.yaml")
	b := gentoc.NewBuilder(reg, defaultIndexNames)
	// Must not panic:
	nav := b.FromNodes(context.Background(), nodes, false)
	if nav == nil {
		t.Fatal("expected non-nil Nav")
	}
}

func TestTitle_NoFrontmatterFile_FilenameDerivation(t *testing.T) {
	// no-frontmatter.md has no frontmatter at all.
	// Filename derivation: "No Frontmatter"
	reg := newLocalRegistry()
	nodes := resolveNodes(t, reg, "https://github.com/gardener/docforge/blob/master/toc.yaml")
	b := gentoc.NewBuilder(reg, defaultIndexNames)
	nav := b.FromNodes(context.Background(), nodes, false)

	entry := entryByFilename(nav.Nav, "Reference/no-frontmatter.md")
	if entry == nil {
		t.Fatal("expected entry for Reference/no-frontmatter.md, got nil")
	}
	if entry.Title != "No Frontmatter" {
		t.Errorf("title = %q, want %q", entry.Title, "No Frontmatter")
	}
}

func TestTitle_ManagedResources_FilenameDerivation(t *testing.T) {
	// managed-resources.md: no frontmatter → "Managed Resources"
	reg := newLocalRegistry()
	nodes := resolveNodes(t, reg, "https://github.com/gardener/docforge/blob/master/toc.yaml")
	b := gentoc.NewBuilder(reg, defaultIndexNames)
	nav := b.FromNodes(context.Background(), nodes, false)

	entry := entryByFilename(nav.Nav, "Reference/managed-resources.md")
	if entry == nil {
		t.Fatal("expected entry for Reference/managed-resources.md, got nil")
	}
	if entry.Title != "Managed Resources" {
		t.Errorf("title = %q, want %q", entry.Title, "Managed Resources")
	}
}

func TestTitle_ApiResources_FilenameDerivation(t *testing.T) {
	// api-resources.md: no frontmatter → "Api Resources"
	reg := newLocalRegistry()
	nodes := resolveNodes(t, reg, "https://github.com/gardener/docforge/blob/master/toc.yaml")
	b := gentoc.NewBuilder(reg, defaultIndexNames)
	nav := b.FromNodes(context.Background(), nodes, false)

	entry := entryByFilename(nav.Nav, "Operations/gardener-operator/api-resources.md")
	if entry == nil {
		t.Fatal("expected entry for Operations/gardener-operator/api-resources.md, got nil")
	}
	if entry.Title != "Api Resources" {
		t.Errorf("title = %q, want %q", entry.Title, "Api Resources")
	}
}

func TestReadError_GracefulDegradation(t *testing.T) {
	// When registry.Read returns an error, gen-toc must NOT crash.
	// It falls back to manifest FM / filename derivation and returns a valid Nav.
	fake := &registryfakes.FakeInterface{}
	fake.ResourceURLCalls(func(u string) (*repositoryhost.URL, error) {
		// Delegate to the real registry for URL parsing
		return newLocalRegistry().ResourceURL(u)
	})
	fake.LoadRepositoryReturns(nil)
	fake.ResolveRelativeLinkCalls(func(src, rel string) (string, error) {
		return newLocalRegistry().ResolveRelativeLink(src, rel)
	})
	// Read always returns an error
	fake.ReadReturns(nil, errors.New("simulated read failure"))

	// We can't use ResolveManifest with the fake (it calls Read for the manifest
	// itself), so build a minimal node tree directly.
	nodes := []*manifest.Node{
		{
			Type: "dir",
			DirType: manifest.DirType{
				Dir: ".",
				Structure: []*manifest.Node{
					{
						Type: "file",
						FileType: manifest.FileType{
							File:   "guide.md",
							Source: "https://github.com/gardener/docforge/blob/master/guide.md",
						},
						Path: ".",
					},
				},
			},
		},
	}

	b := gentoc.NewBuilder(fake, defaultIndexNames)
	nav := b.FromNodes(context.Background(), nodes, false)

	if nav == nil || len(nav.Nav) == 0 {
		t.Fatal("expected non-nil, non-empty Nav even on read error")
	}
	entry := nav.Nav[0]
	if entry == nil {
		t.Fatal("expected non-nil entry")
	}
	// Falls back to filename derivation: "guide.md" → "Guide"
	if entry.Title != "Guide" {
		t.Errorf("title = %q, want %q (filename fallback)", entry.Title, "Guide")
	}
}

func TestAssetsDir_SkippedFromNav(t *testing.T) {
	// The Troubleshooting/assets/ dir contains only diagram.png (non-content).
	// It must not appear in the nav.
	reg := newLocalRegistry()
	nodes := resolveNodes(t, reg, "https://github.com/gardener/docforge/blob/master/toc.yaml")
	b := gentoc.NewBuilder(reg, defaultIndexNames)
	nav := b.FromNodes(context.Background(), nodes, false)

	if e := entryByFilename(nav.Nav, "Troubleshooting/assets"); e != nil {
		t.Errorf("expected assets dir to be absent from nav, got entry: %+v", e)
	}
}

func TestSubnavTree_Structure(t *testing.T) {
	// Operations has a README (promoted) + subdir gardener-operator with
	// its own README and api-resources.md.
	reg := newLocalRegistry()
	nodes := resolveNodes(t, reg, "https://github.com/gardener/docforge/blob/master/toc.yaml")
	b := gentoc.NewBuilder(reg, defaultIndexNames)
	nav := b.FromNodes(context.Background(), nodes, false)

	opsEntry := entryByFilename(nav.Nav, "Operations/README.md")
	if opsEntry == nil {
		t.Fatal("expected Operations/README.md entry")
	}
	if len(opsEntry.Subnav) == 0 {
		t.Fatal("expected Operations to have subnav")
	}
	goEntry := entryByFilename(opsEntry.Subnav, "Operations/gardener-operator/README.md")
	if goEntry == nil {
		t.Fatal("expected gardener-operator/README.md in subnav")
	}
	if goEntry.Title != "Gardener Operator" {
		t.Errorf("gardener-operator title = %q, want %q", goEntry.Title, "Gardener Operator")
	}
}

func TestHTMLFile_GetsTitle(t *testing.T) {
	// .html files are content files and should get a title via filename derivation
	// when no frontmatter is available.
	nodes := []*manifest.Node{
		{
			Type: "dir",
			DirType: manifest.DirType{
				Dir: ".",
				Structure: []*manifest.Node{
					{
						Type: "file",
						FileType: manifest.FileType{
							File:   "getting-started.html",
							Source: "https://github.com/gardener/docforge/blob/master/getting-started.html",
						},
						Path: ".",
					},
				},
			},
		},
	}

	fake := &registryfakes.FakeInterface{}
	fake.ReadReturns(nil, errors.New("html source not needed"))
	b := gentoc.NewBuilder(fake, defaultIndexNames)
	nav := b.FromNodes(context.Background(), nodes, false)

	if len(nav.Nav) == 0 {
		t.Fatal("expected entries")
	}
	entry := nav.Nav[0]
	if entry.Title != "Getting Started" {
		t.Errorf("title = %q, want %q", entry.Title, "Getting Started")
	}
}

func TestAllEntriesHaveNonEmptyTitle(t *testing.T) {
	// Every NavEntry produced from the full toc.yaml fixture must have a non-empty title.
	reg := newLocalRegistry()
	nodes := resolveNodes(t, reg, "https://github.com/gardener/docforge/blob/master/toc.yaml")
	b := gentoc.NewBuilder(reg, defaultIndexNames)
	nav := b.FromNodes(context.Background(), nodes, false)

	var checkEntries func(entries []*gentoc.NavEntry, path string)
	checkEntries = func(entries []*gentoc.NavEntry, path string) {
		for _, e := range entries {
			if e.Title == "" {
				t.Errorf("entry at %s%s has empty title", path, e.Filename)
			}
			checkEntries(e.Subnav, path+e.Filename+"/")
		}
	}
	checkEntries(nav.Nav, "")
}
