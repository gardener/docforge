// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package repositoryhost_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gardener/docforge/pkg/osfakes/osshim"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
)

// --- IsLocalPath ---

func TestIsLocalPath(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"./manifest.yaml", true},
		{"../docs/manifest.yaml", true},
		{"/abs/path/manifest.yaml", true},
		{"manifest.yaml", true},
		{"file:///abs/path/manifest.yaml", true},
		{"", false},
		{"https://github.com/org/repo/blob/main/file.yaml", false},
		{"http://example.com/manifest.yaml", false},
	}
	for _, c := range cases {
		got := repositoryhost.IsLocalPath(c.input)
		if got != c.want {
			t.Errorf("IsLocalPath(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

// --- LocalPath host ---

func TestLocalPathAccept(t *testing.T) {
	dir := t.TempDir()
	lp := repositoryhost.NewLocalPath(&osshim.OsShim{}, dir)
	cases := []struct {
		link string
		want bool
	}{
		{"file://" + filepath.Join(dir, "file.yaml"), true},
		{"file://" + filepath.Join(dir, "sub", "file.yaml"), true},
		{"file:///etc/passwd", false},                                                   // outside localDir
		{"file://" + filepath.Join(filepath.Dir(dir), "sibling", "file.yaml"), false},  // sibling dir
		{"https://github.com/org/repo/blob/main/file.yaml", false},
		{"./local.yaml", false},
		{"", false},
	}
	for _, c := range cases {
		got := lp.Accept(c.link)
		if got != c.want {
			t.Errorf("Accept(%q) = %v, want %v", c.link, got, c.want)
		}
	}
}

// TestLocalPathAcceptContainment specifically verifies the security boundary:
// a file:// URL that escapes the localDir must be rejected.
func TestLocalPathAcceptContainment(t *testing.T) {
	dir := t.TempDir()
	lp := repositoryhost.NewLocalPath(&osshim.OsShim{}, dir)

	outside := []string{
		"file:///etc/passwd",
		"file:///etc/cron.d/evil",
		"file://" + filepath.Join(dir, "..", "sibling"),
		"file://" + filepath.Dir(dir), // the parent itself
	}
	for _, link := range outside {
		if lp.Accept(link) {
			t.Errorf("Accept(%q) = true, want false (must not escape localDir)", link)
		}
	}

	inside := []string{
		"file://" + dir,
		"file://" + filepath.Join(dir, "manifest.yaml"),
		"file://" + filepath.Join(dir, "sub", "deep", "file.md"),
	}
	for _, link := range inside {
		if !lp.Accept(link) {
			t.Errorf("Accept(%q) = false, want true (should be within localDir)", link)
		}
	}
}

func TestLocalPathRead(t *testing.T) {
	dir := t.TempDir()
	content := []byte("# test manifest\n")
	if err := os.WriteFile(filepath.Join(dir, "manifest.yaml"), content, 0644); err != nil {
		t.Fatal(err)
	}

	lp := repositoryhost.NewLocalPath(&osshim.OsShim{}, dir)
	fileURL := "file://" + filepath.Join(dir, "manifest.yaml")

	u, err := lp.ResourceURL(fileURL)
	if err != nil {
		t.Fatalf("ResourceURL: %v", err)
	}

	got, err := lp.Read(context.Background(), *u)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("Read content = %q, want %q", got, content)
	}
}

func TestLocalPathReadMissing(t *testing.T) {
	dir := t.TempDir()
	lp := repositoryhost.NewLocalPath(&osshim.OsShim{}, dir)
	u, err := lp.ResourceURL("file://" + filepath.Join(dir, "nonexistent.yaml"))
	if err != nil {
		t.Fatalf("ResourceURL: %v", err)
	}
	_, err = lp.Read(context.Background(), *u)
	if err == nil {
		t.Error("expected error reading nonexistent file, got nil")
	}
}

func TestLocalPathResolveRelativeLink(t *testing.T) {
	dir := t.TempDir()
	// create the file so existence is not an issue; our impl does not check existence
	if err := os.WriteFile(filepath.Join(dir, "sub", "other.yaml"), []byte{}, 0644); err != nil {
		// sub/ may not exist yet — create it
		_ = os.MkdirAll(filepath.Join(dir, "sub"), 0755)
		_ = os.WriteFile(filepath.Join(dir, "sub", "other.yaml"), []byte{}, 0644)
	}

	lp := repositoryhost.NewLocalPath(&osshim.OsShim{}, dir)
	sourceURL := "file://" + filepath.Join(dir, "manifest.yaml")
	u, err := lp.ResourceURL(sourceURL)
	if err != nil {
		t.Fatalf("ResourceURL: %v", err)
	}

	resolved, err := lp.ResolveRelativeLink(*u, "./sub/other.yaml")
	if err != nil {
		t.Fatalf("ResolveRelativeLink: %v", err)
	}
	want := "file://" + filepath.Join(dir, "sub", "other.yaml")
	if resolved != want {
		t.Errorf("ResolveRelativeLink = %q, want %q", resolved, want)
	}
}

// --- Security tests ---

// TestLocalPathReadSymlinkEscape verifies that a symlink inside localDir that points
// outside it is rejected at Read time (CRIT-1).
func TestLocalPathReadSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()

	secretFile := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secretFile, []byte("sensitive"), 0644); err != nil {
		t.Fatal(err)
	}
	symlinkPath := filepath.Join(dir, "evil")
	if err := os.Symlink(secretFile, symlinkPath); err != nil {
		t.Fatal(err)
	}

	lp := repositoryhost.NewLocalPath(&osshim.OsShim{}, dir)
	u, err := lp.ResourceURL("file://" + symlinkPath)
	if err != nil {
		t.Fatalf("ResourceURL: %v", err)
	}
	_, err = lp.Read(context.Background(), *u)
	if err == nil {
		t.Error("Read(symlink escaping localDir) succeeded, want error")
	}
}

// TestLocalPathAcceptSymlinkEscape verifies that Accept rejects a file:// URL whose
// resolved symlink target is outside localDir (CRIT-1, second layer).
func TestLocalPathAcceptSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()

	symlinkPath := filepath.Join(dir, "evil")
	if err := os.Symlink(outside, symlinkPath); err != nil {
		t.Fatal(err)
	}

	lp := repositoryhost.NewLocalPath(&osshim.OsShim{}, dir)
	if lp.Accept("file://" + symlinkPath) {
		t.Error("Accept(symlink pointing outside localDir) = true, want false")
	}
}

// TestLocalPathResolveRelativeLinkEscape verifies that path traversal via relative links
// is rejected before a file:// URL is returned (HIGH-1).
func TestLocalPathResolveRelativeLinkEscape(t *testing.T) {
	dir := t.TempDir()
	lp := repositoryhost.NewLocalPath(&osshim.OsShim{}, dir)
	u, err := lp.ResourceURL("file://" + filepath.Join(dir, "manifest.yaml"))
	if err != nil {
		t.Fatalf("ResourceURL: %v", err)
	}
	_, err = lp.ResolveRelativeLink(*u, "../../etc/passwd")
	if err == nil {
		t.Error("ResolveRelativeLink(traversal) succeeded, want error")
	}
}

// TestLocalPathTreeEscape verifies that Tree rejects a root URL outside localDir (MED-1).
func TestLocalPathTreeEscape(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	lp := repositoryhost.NewLocalPath(&osshim.OsShim{}, dir)
	u, err := lp.ResourceURL("file://" + outside)
	if err != nil {
		t.Fatalf("ResourceURL: %v", err)
	}
	_, err = lp.Tree(*u)
	if err == nil {
		t.Error("Tree(path outside localDir) succeeded, want error")
	}
}
