// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package writers

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gardener/docforge/pkg/manifest"
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

			err := fs.Write(tc.name, tc.path, tc.docBlob, tc.node, nil)

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
			err := w.Write(c.fileName, c.path, []byte("test-content"), nil, nil)

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
