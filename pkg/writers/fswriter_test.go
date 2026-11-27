// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package writers

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/sitegen"
	"github.com/google/uuid"
)

func TestWrite(t *testing.T) {
	testCases := []struct {
		name         string
		path         string
		docBlob      []byte
		node         *manifest.Node
		wantErr      error
		wantFileName string
		wantContent  string
	}{
		{
			name:         "test.md",
			path:         "a/b",
			docBlob:      []byte("# Test"),
			node:         &manifest.Node{},
			wantErr:      nil,
			wantFileName: `test.md`,
			wantContent:  `# Test`,
		},
		{
			name:         "test",
			path:         "a/b",
			docBlob:      []byte("# Test"),
			node:         &manifest.Node{},
			wantErr:      nil,
			wantFileName: `test`,
			wantContent:  `# Test`,
		},
	}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			testFolder := fmt.Sprintf("test%s", uuid.New().String())
			testPath := filepath.Join(os.TempDir(), testFolder)
			fs := &FSWriter{
				Root: testPath,
			}
			fPath := filepath.Join(fs.Root, tc.path, tc.wantFileName)
			defer func() {
				if err := os.RemoveAll(testPath); err != nil {
					t.Fatalf("%v\n", err)
				}
			}()

			err := fs.Write(tc.name, tc.path, tc.docBlob, tc.node)

			if err != tc.wantErr {
				t.Errorf("expected err %v != %v", tc.wantErr, err)
			}
			if _, err := os.Stat(fPath); tc.wantErr == nil && os.IsNotExist(err) {
				t.Errorf("expected file to be written, but it was not")
			}
			var (
				b []byte
			)
			if b, err = os.ReadFile(fPath); err != nil {
				t.Errorf("unexpected error opening file %v", err)
			}
			if !reflect.DeepEqual(b, []byte(tc.wantContent)) {
				t.Errorf("expected content %v != %v", tc.wantContent, tc.wantContent)
			}
		})
	}
}

func TestWritePathTraversal(t *testing.T) {
	cases := []struct {
		name     string
		fileName string
		path     string
		wantErr  bool
	}{
		{
			name:     "shallow traversal is rejected",
			fileName: "payload.txt",
			path:     "../OUTSIDE-ROOT",
			wantErr:  true,
		},
		{
			name:     "deep traversal is rejected",
			fileName: "authorized_keys",
			path:     "../../../../tmp/pwn",
			wantErr:  true,
		},
		{
			name:     "legitimate nested path is allowed",
			fileName: "readme.md",
			path:     "docs/sub",
			wantErr:  false,
		},
		{
			name:     "directory name starting with .. without separator is allowed",
			fileName: "file.md",
			path:     "..config",
			wantErr:  false,
		},
		{
			name:     "traversal via file name is rejected",
			fileName: "../../../sibling-secret",
			path:     "docs",
			wantErr:  true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := filepath.Join(os.TempDir(), fmt.Sprintf("fswriter-test-%s", uuid.New().String()))
			defer os.RemoveAll(root)

			w := &FSWriter{Root: root}
			err := w.Write(c.fileName, c.path, []byte("test-content"), nil)

			if c.wantErr && err == nil {
				t.Errorf("expected error for path %q but got none", c.path)
			}
			if !c.wantErr && err != nil {
				t.Errorf("unexpected error for path %q: %v", c.path, err)
			}

			if c.wantErr {
				escaped := filepath.Join(filepath.Clean(root), c.path, c.fileName)
				if _, statErr := os.Stat(escaped); statErr == nil {
					t.Errorf("file was written at %q despite traversal rejection", escaped)
				}
			}

			if !c.wantErr {
				expected := filepath.Join(root, c.path, c.fileName)
				if _, statErr := os.Stat(expected); os.IsNotExist(statErr) {
					t.Errorf("expected file at %q but it was not written", expected)
				}
			}
		})
	}
}

// TestIndexStubGeneration covers Bug 2: directories with no source file must
// emit an index stub in both Hugo (_index.md) and VitePress (index.md) modes.
// The stub file must contain the node frontmatter and nothing else.
func TestIndexStubGeneration(t *testing.T) {
	cases := []struct {
		name         string
		cfg          sitegen.Config
		writeName    string // name passed to Write (after document_worker renaming)
		frontmatter  map[string]interface{}
		docBlob      []byte // nil = stub (no source file); non-nil = real content
		wantWritten  bool
		wantContains string // substring that must appear in the written file
	}{
		{
			name: "Hugo: _index.md stub with frontmatter is written",
			cfg: sitegen.SimpleConfig{
				IsEnabled:       true,
				IndexFiles:      []string{"readme.md", "README.md"},
				IndexFileTarget: "_index.md",
			},
			writeName:    "_index.md",
			frontmatter:  map[string]interface{}{"title": "Extensions"},
			docBlob:      nil,
			wantWritten:  true,
			wantContains: "title: Extensions",
		},
		{
			// Bug 2: before the fix, VitePress wrote nothing because the check
			// hardcoded "_index.md"; after the fix it checks IndexFileName().
			name: "VitePress: index.md stub with frontmatter is written",
			cfg: sitegen.SimpleConfig{
				IsEnabled:       true,
				IndexFiles:      []string{"readme.md", "README.md"},
				IndexFileTarget: "index.md",
			},
			writeName:    "index.md",
			frontmatter:  map[string]interface{}{"title": "Extensions"},
			docBlob:      nil,
			wantWritten:  true,
			wantContains: "title: Extensions",
		},
		{
			// A real document (non-nil docBlob) must be written as-is — stub logic must not fire.
			name: "Hugo: real content is written verbatim, stub logic skipped",
			cfg: sitegen.SimpleConfig{
				IsEnabled:       true,
				IndexFiles:      []string{"readme.md", "README.md"},
				IndexFileTarget: "_index.md",
			},
			writeName:    "_index.md",
			frontmatter:  map[string]interface{}{"title": "Extensions"},
			docBlob:      []byte("# Real Content"),
			wantWritten:  true,
			wantContains: "# Real Content",
		},
		{
			// When site generator is disabled, stub logic must not fire either.
			name: "disabled generator: stub not written even with matching name",
			cfg: sitegen.SimpleConfig{
				IsEnabled:       false,
				IndexFileTarget: "_index.md",
			},
			writeName:   "_index.md",
			frontmatter: map[string]interface{}{"title": "Extensions"},
			docBlob:     nil,
			wantWritten: false,
		},
		{
			// A non-index file with no content must not be written (existing behaviour).
			name: "non-index file with nil docBlob is not written",
			cfg: sitegen.SimpleConfig{
				IsEnabled:       true,
				IndexFileTarget: "_index.md",
			},
			writeName:   "guide.md",
			frontmatter: map[string]interface{}{"title": "Guide"},
			docBlob:     nil,
			wantWritten: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := filepath.Join(os.TempDir(), fmt.Sprintf("fswriter-stub-%s", uuid.New().String()))
			defer os.RemoveAll(root)

			w := &FSWriter{Root: root, Config: c.cfg}
			node := &manifest.Node{
				FileType:    manifest.FileType{File: c.writeName},
				Type:        "file",
				Path:        "docs/extensions",
				Frontmatter: c.frontmatter,
			}

			if err := w.Write(c.writeName, "docs/extensions", c.docBlob, node); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			filePath := filepath.Join(root, "docs/extensions", c.writeName)
			_, statErr := os.Stat(filePath)
			fileExists := !os.IsNotExist(statErr)

			if c.wantWritten && !fileExists {
				t.Errorf("expected file %q to be written but it was not", filePath)
				return
			}
			if !c.wantWritten && fileExists {
				t.Errorf("expected file %q NOT to be written but it was", filePath)
				return
			}
			if c.wantWritten && c.wantContains != "" {
				content, err := os.ReadFile(filePath)
				if err != nil {
					t.Fatalf("reading written file: %v", err)
				}
				if !strings.Contains(string(content), c.wantContains) {
					t.Errorf("file content %q does not contain %q", string(content), c.wantContains)
				}
			}
		})
	}
}
