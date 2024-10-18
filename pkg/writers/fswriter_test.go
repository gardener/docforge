// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package writers

import (
	"fmt"
	"io/ioutil"
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
			if b, err = ioutil.ReadFile(fPath); err != nil {
				t.Errorf("unexpected error opening file %v", err)
			}
			if !reflect.DeepEqual(b, []byte(tc.wantContent)) {
				t.Errorf("expected content %v != %v", tc.wantContent, tc.wantContent)
			}
		})
	}
}
