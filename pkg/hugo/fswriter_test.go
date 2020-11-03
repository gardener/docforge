// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package hugo

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/util/tests"
	"github.com/gardener/docforge/pkg/writers"
	"github.com/google/uuid"
)

func init() {
	tests.SetKlogV(6)
}

func TestWrite(t *testing.T) {
	testCases := []struct {
		name         string
		path         string
		docBlob      []byte
		node         *api.Node
		wantErr      error
		wantFileName string
		wantContent  string
		mutate       func(writer *FSWriter)
	}{
		{
			name:         "test.md",
			path:         "a/b",
			docBlob:      []byte("# Test"),
			wantErr:      nil,
			wantFileName: `test.md`,
			wantContent:  `# Test`,
		},
		{
			name:         "test",
			path:         "a/b",
			docBlob:      []byte("# Test"),
			node:         &api.Node{},
			wantErr:      nil,
			wantFileName: `test.md`,
			wantContent:  `# Test`,
		},
		{
			name:    "test",
			path:    "a/b",
			docBlob: nil,
			node: &api.Node{
				Properties: map[string]interface{}{
					"frontmatter": map[string]string{
						"title": "Test1",
					},
				},
			},
			wantErr:      nil,
			wantFileName: filepath.Join("test", "_index.md"),
			wantContent: `---
title: Test1
---
`,
		},
		{
			name:    "README",
			path:    "a/b",
			docBlob: []byte("# Test"),
			node: &api.Node{
				Name: "README",
				Properties: map[string]interface{}{
					"index": true,
				},
				ContentSelectors: []api.ContentSelector{
					api.ContentSelector{
						Source: "github.com",
					},
				},
			},
			wantErr:      nil,
			wantFileName: filepath.Join("_index.md"),
			wantContent:  `# Test`,
			mutate: func(writer *FSWriter) {
				writer.IndexFileNames = []string{"readme"}
			},
		},
	}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			testFolder := fmt.Sprintf("test%s", uuid.New().String())
			testPath := filepath.Join(os.TempDir(), testFolder)
			fs := &FSWriter{
				Writer: &writers.FSWriter{
					Root: testPath,
				},
			}
			if tc.mutate != nil {
				tc.mutate(fs)
			}
			fPath := filepath.Join(testPath, tc.path, tc.wantFileName)
			defer func() {
				if err := os.RemoveAll(testPath); err != nil {
					t.Fatalf("%v\n", err)
				}
			}()

			if tc.node != nil {
				tc.node.SetParentsDownwards()
			}
			err := fs.Write(tc.name, tc.path, tc.docBlob, tc.node)

			if err != tc.wantErr {
				t.Errorf("expected err %v != %v", tc.wantErr, err)
			}
			if _, err := os.Stat(fPath); tc.wantErr == nil && os.IsNotExist(err) {
				t.Errorf("expected file %s not found: %v", fPath, err)
			}
			var (
				gotContent []byte
			)
			if gotContent, err = ioutil.ReadFile(fPath); err != nil {
				t.Errorf("unexpected error opening file %v", err)
			}
			if !reflect.DeepEqual(gotContent, []byte(tc.wantContent)) {
				t.Errorf("expected content \n%v\n != \n%v\n", tc.wantContent, string(gotContent))
			}
		})
	}
}
