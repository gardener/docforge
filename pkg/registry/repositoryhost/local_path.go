// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package repositoryhost

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/gardener/docforge/pkg/osfakes/httpclient"
	"github.com/gardener/docforge/pkg/osfakes/osshim"
)

// LocalPath is a repository host that reads from the local filesystem using file:// URLs.
// Unlike Local (which maps GitHub resource URLs to local paths via resourceMappings),
// LocalPath accepts file:// URLs and reads the absolute paths they encode — but ONLY for
// paths that are contained within localDir.
//
// The localDir scope is a security boundary: it prevents a malicious remote sub-manifest
// from injecting a file:// URL (e.g. file:///etc/passwd) that would be served by this host.
// Only files under the directory of the original local manifest are accessible.
//
// It is automatically registered when -f / --manifest receives a local filesystem path.
type LocalPath struct {
	os       osshim.Os
	localDir string // only serve files at or beneath this directory (always symlink-resolved)
}

// NewLocalPath creates a LocalPath repository host that only serves file:// URLs
// contained within localDir (the directory of the original manifest file).
// localDir is resolved through symlinks at construction time so that all later
// containment checks use real (non-symlink) paths.
func NewLocalPath(os osshim.Os, localDir string) Interface {
	clean := filepath.Clean(localDir)
	if resolved, err := filepath.EvalSymlinks(clean); err == nil {
		clean = resolved
	}
	return &LocalPath{os: os, localDir: clean}
}

// pathEscapes reports whether p is not contained within root.
// p is cleaned before the check; root must already be clean.
func pathEscapes(root, p string) bool {
	rel, err := filepath.Rel(root, filepath.Clean(p))
	return err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// evalSymlinksLoose resolves symlinks in p as far as possible.
// Unlike filepath.EvalSymlinks it does not require p to exist: it walks up the
// directory tree until it finds a prefix that can be resolved, then appends the
// remaining (non-existent) components. This handles platforms like macOS where
// t.TempDir() returns /var/... but the real path is /private/var/...
func evalSymlinksLoose(p string) string {
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		return resolved
	}
	// Walk up until we find a directory that can be resolved.
	remaining := []string{}
	cur := p
	for {
		parent := filepath.Dir(cur)
		if parent == cur {
			break // reached filesystem root without resolution
		}
		remaining = append([]string{filepath.Base(cur)}, remaining...)
		cur = parent
		if resolved, err := filepath.EvalSymlinks(cur); err == nil {
			return filepath.Join(append([]string{resolved}, remaining...)...)
		}
	}
	return p // fallback: return as-is
}

// Accept returns true only for file:// URLs whose resolved path is contained within localDir.
// Symlinks are resolved (best-effort) before the containment check so that a symlink inside
// localDir pointing outside it is rejected even though its path string is within localDir.
func (l *LocalPath) Accept(link string) bool {
	if !strings.HasPrefix(link, "file://") {
		return false
	}
	p := evalSymlinksLoose(filepath.Clean(strings.TrimPrefix(link, "file://")))
	return !pathEscapes(l.localDir, p)
}

// ResourceURL parses a file:// URL into a URL struct whose GetResourcePath() returns
// the absolute filesystem path.
func (l *LocalPath) ResourceURL(resourceURL string) (*URL, error) {
	if !strings.HasPrefix(resourceURL, "file://") {
		return nil, fmt.Errorf("not a file URL: %s", resourceURL)
	}
	// new() handles file:// by setting host="local" and resourcePath=u.Path.
	return new(resourceURL)
}

// ResolveRelativeLink resolves relativeLink relative to source using filesystem path semantics
// and returns a file:// URL for the resolved absolute path.
//
// Returns an error if the resolved path escapes localDir, so that a malicious manifest
// cannot reach files outside the manifest directory via relative traversal (e.g. ../../etc/passwd).
func (l *LocalPath) ResolveRelativeLink(source URL, relativeLink string) (string, error) {
	dir := filepath.Dir(source.GetResourcePath())
	abs := filepath.Clean(filepath.Join(dir, relativeLink))
	if pathEscapes(l.localDir, evalSymlinksLoose(abs)) {
		return "", fmt.Errorf("relative link %q escapes manifest directory", relativeLink)
	}
	return "file://" + abs, nil
}

// LoadRepository is a no-op for local paths.
func (l *LocalPath) LoadRepository(_ context.Context, _ string) error {
	return nil
}

// Tree returns the relative paths of all files under the directory encoded in resource.
func (l *LocalPath) Tree(resource URL) ([]string, error) {
	dirPath := resource.GetResourcePath()
	if pathEscapes(l.localDir, evalSymlinksLoose(dirPath)) {
		return nil, fmt.Errorf("path %q escapes manifest directory", dirPath)
	}
	var files []string
	err := filepath.Walk(dirPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			rel, relErr := filepath.Rel(dirPath, path)
			if relErr != nil {
				return relErr
			}
			files = append(files, rel)
		}
		return nil
	})
	return files, err
}

// Read reads the file at the absolute path encoded in resource.GetResourcePath().
// Symlinks are resolved before reading; if the resolved path escapes localDir the
// read is rejected, preventing exfiltration via in-scope symlinks that point outside.
func (l *LocalPath) Read(_ context.Context, resource URL) ([]byte, error) {
	p := resource.GetResourcePath()
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		if l.os.IsNotExist(err) {
			return nil, ErrResourceNotFound(resource.String())
		}
		return nil, fmt.Errorf("reading local file %s: %v", p, err)
	}
	if pathEscapes(l.localDir, resolved) {
		return nil, fmt.Errorf("path traversal: %q escapes manifest directory", p)
	}
	cnt, err := l.os.ReadFile(resolved)
	if err != nil {
		if l.os.IsNotExist(err) {
			return nil, ErrResourceNotFound(resource.String())
		}
		return nil, fmt.Errorf("reading local file %s: %v", p, err)
	}
	return cnt, nil
}

// Name returns "local-path".
func (l *LocalPath) Name() string {
	return "local-path"
}

// Repositories returns nil — LocalPath has no GitHub-style repository API.
// This also ensures the registry never routes LoadRepository or ReadGitInfo to this host.
func (l *LocalPath) Repositories() Repositories {
	return nil
}

// GetClient returns nil — LocalPath performs no HTTP requests.
func (l *LocalPath) GetClient() httpclient.Client {
	return nil
}

// GetRateLimit is not applicable to local paths.
func (l *LocalPath) GetRateLimit(_ context.Context) (int, int, time.Time, error) {
	return 0, 0, time.Time{}, errors.New("not implemented")
}
