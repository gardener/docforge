// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package app_test contains integration-level tests for manifest × source combinations.
// The four cases are:
//
//	(A) local manifest  × remote sources  — new: manifest loaded from disk via -f ./path
//	(B) local manifest  × local sources   — new: relative sources resolved to file:// URLs
//	(C) remote manifest × remote sources  — existing behaviour (regression)
//	(D) remote manifest × local sources   — existing resourceMappings behaviour (regression)
package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/osfakes/osshim"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func fileNodesFrom(nodes []*manifest.Node) []*manifest.Node {
	var out []*manifest.Node
	for _, n := range nodes {
		if n.Type == "file" {
			out = append(out, n)
		}
	}
	return out
}

// TestCaseA_LocalManifest_RemoteSources: the manifest lives on the local filesystem;
// source URLs inside it are absolute GitHub URLs served by a separate (non-LocalPath) host.
// This is the primary new-behaviour case enabled by this feature.
func TestCaseA_LocalManifest_RemoteSources(t *testing.T) {
	manifestDir := t.TempDir()
	remoteDir := t.TempDir()

	// Manifest on disk; source is an absolute GitHub-style URL.
	writeFile(t, filepath.Join(manifestDir, "manifest.yaml"), `structure:
- dir: docs
  structure:
  - file: README.md
    source: https://github.com/gardener/docforge/blob/master/README.md
`)
	writeFile(t, filepath.Join(remoteDir, "README.md"), "# remote content")

	// LocalPath handles the manifest; NewLocal simulates a remote GitHub host.
	r := registry.NewRegistry(
		repositoryhost.NewLocalPath(&osshim.OsShim{}, manifestDir),
		repositoryhost.NewLocal(&osshim.OsShim{},
			"https://github.com/gardener/docforge/blob/master", remoteDir),
	)

	nodes, err := manifest.ResolveManifest(
		"file://"+filepath.Join(manifestDir, "manifest.yaml"), r)
	if err != nil {
		t.Fatalf("ResolveManifest: %v", err)
	}

	fn := fileNodesFrom(nodes)
	if len(fn) != 1 {
		t.Fatalf("expected 1 file node, got %d", len(fn))
	}
	// Remote source URL must be preserved unchanged.
	want := "https://github.com/gardener/docforge/blob/master/README.md"
	if fn[0].Source != want {
		t.Errorf("Source = %q, want %q", fn[0].Source, want)
	}
}

// TestCaseB_LocalManifest_LocalSources: manifest and sources are both on the local
// filesystem. Relative source paths in the manifest are resolved to absolute file:// URLs.
func TestCaseB_LocalManifest_LocalSources(t *testing.T) {
	manifestDir := t.TempDir()

	writeFile(t, filepath.Join(manifestDir, "manifest.yaml"), `structure:
- dir: docs
  structure:
  - file: README.md
    source: ./README.md
`)
	writeFile(t, filepath.Join(manifestDir, "README.md"), "# local content")

	r := registry.NewRegistry(repositoryhost.NewLocalPath(&osshim.OsShim{}, manifestDir))

	nodes, err := manifest.ResolveManifest(
		"file://"+filepath.Join(manifestDir, "manifest.yaml"), r)
	if err != nil {
		t.Fatalf("ResolveManifest: %v", err)
	}

	fn := fileNodesFrom(nodes)
	if len(fn) != 1 {
		t.Fatalf("expected 1 file node, got %d", len(fn))
	}
	// Source must be the absolute file:// URL pointing to README.md.
	got := fn[0].Source
	if !strings.HasPrefix(got, "file://") {
		t.Errorf("Source = %q: expected file:// scheme", got)
	}
	if !strings.HasSuffix(got, filepath.Join(manifestDir, "README.md")) {
		t.Errorf("Source = %q: expected suffix %s", got, filepath.Join(manifestDir, "README.md"))
	}
}

// TestCaseC_RemoteManifest_RemoteSources: regression — a manifest at a GitHub-style URL
// with remote sources continues to work exactly as before.
func TestCaseC_RemoteManifest_RemoteSources(t *testing.T) {
	remoteDir := t.TempDir()
	writeFile(t, filepath.Join(remoteDir, "manifest.yaml"), `structure:
- dir: docs
  structure:
  - file: README.md
    source: https://github.com/gardener/docforge/blob/master/README.md
`)
	writeFile(t, filepath.Join(remoteDir, "README.md"), "# content")

	r := registry.NewRegistry(
		repositoryhost.NewLocal(&osshim.OsShim{},
			"https://github.com/gardener/docforge/blob/master", remoteDir),
	)

	nodes, err := manifest.ResolveManifest(
		"https://github.com/gardener/docforge/blob/master/manifest.yaml", r)
	if err != nil {
		t.Fatalf("ResolveManifest: %v", err)
	}
	if len(fileNodesFrom(nodes)) != 1 {
		t.Fatalf("expected 1 file node, got %d", len(fileNodesFrom(nodes)))
	}
}

// TestCaseD_RemoteManifest_LocalSources: regression — resourceMappings behaviour.
// The manifest is served by a "remote" host; sources are remapped to a local directory
// by registering a second NewLocal host for the same URL prefix (simulating --resourceMappings).
func TestCaseD_RemoteManifest_LocalSources(t *testing.T) {
	// A single local directory contains both manifest and source files.
	// In practice this represents a local clone of the remote repository.
	localClone := t.TempDir()
	writeFile(t, filepath.Join(localClone, "manifest.yaml"), `structure:
- dir: docs
  structure:
  - file: README.md
    source: https://github.com/gardener/docforge/blob/master/README.md
`)
	writeFile(t, filepath.Join(localClone, "README.md"), "# local clone of remote source")

	// A single NewLocal host serves both the manifest and the sources from the local clone,
	// as if the remote URL prefix is remapped via resourceMappings.
	r := registry.NewRegistry(
		repositoryhost.NewLocal(&osshim.OsShim{},
			"https://github.com/gardener/docforge/blob/master", localClone),
	)

	nodes, err := manifest.ResolveManifest(
		"https://github.com/gardener/docforge/blob/master/manifest.yaml", r)
	if err != nil {
		t.Fatalf("ResolveManifest: %v", err)
	}
	fn := fileNodesFrom(nodes)
	if len(fn) != 1 {
		t.Fatalf("expected 1 file node, got %d", len(fn))
	}
	// Source URL is still the remote GitHub URL; the local mapping applies at read time.
	if !strings.HasPrefix(fn[0].Source, "https://github.com") {
		t.Errorf("unexpected Source = %q", fn[0].Source)
	}
}
