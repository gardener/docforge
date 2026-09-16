// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package document_test

import (
	"context"
	"testing"

	"github.com/gardener/docforge/cmd/hugo"
	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/nodeplugins/markdown/document"
	"github.com/gardener/docforge/pkg/nodeplugins/markdown/linkresolver/linkresolverfakes"
	"github.com/gardener/docforge/pkg/registry/registryfakes"
	"github.com/gardener/docforge/pkg/writers/writersfakes"
)

func TestProcessNodeHugoIndexRename(t *testing.T) {
	cases := []struct {
		name         string
		hugoEnabled  bool
		inputFile    string
		indexNames   []string
		wantWriteAs  string
	}{
		{
			name:        "README.md renamed to _index.md when Hugo enabled",
			hugoEnabled: true,
			inputFile:   "README.md",
			indexNames:  []string{"readme.md", "README.md"},
			wantWriteAs: "_index.md",
		},
		{
			name:        "readme.md (lowercase) renamed to _index.md when Hugo enabled",
			hugoEnabled: true,
			inputFile:   "readme.md",
			indexNames:  []string{"readme.md", "README.md"},
			wantWriteAs: "_index.md",
		},
		{
			name:        "README.md NOT renamed when Hugo disabled",
			hugoEnabled: false,
			inputFile:   "README.md",
			indexNames:  []string{"readme.md", "README.md"},
			wantWriteAs: "README.md",
		},
		{
			name:        "non-index file unaffected when Hugo enabled",
			hugoEnabled: true,
			inputFile:   "guide.md",
			indexNames:  []string{"readme.md", "README.md"},
			wantWriteAs: "guide.md",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := hugo.Hugo{
				Enabled:        c.hugoEnabled,
				IndexFileNames: c.indexNames,
			}
			w := &writersfakes.FakeWriter{}
			dw := document.NewDocumentWorker(
				&linkresolverfakes.FakeInterface{},
				&registryfakes.FakeInterface{},
				h,
				w,
			)
			node := &manifest.Node{
				FileType: manifest.FileType{File: c.inputFile},
				Type:     "file",
				Path:     "docs",
			}
			if err := dw.ProcessNode(context.Background(), node); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if w.WriteCallCount() != 1 {
				t.Fatalf("expected Write to be called once, got %d", w.WriteCallCount())
			}
			gotName, _, _, _ := w.WriteArgsForCall(0)
			if gotName != c.wantWriteAs {
				t.Errorf("Write called with name %q, want %q", gotName, c.wantWriteAs)
			}
		})
	}
}
