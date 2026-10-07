// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package document_test

import (
	"context"
	"testing"

	"github.com/gardener/docforge/cmd/vitepress"
	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/nodeplugins/markdown/document"
	"github.com/gardener/docforge/pkg/nodeplugins/markdown/linkresolver/linkresolverfakes"
	"github.com/gardener/docforge/pkg/registry/registryfakes"
	"github.com/gardener/docforge/pkg/sitegen"
	"github.com/gardener/docforge/pkg/writers/writersfakes"
)

func TestProcessNodeHugoIndexRename(t *testing.T) {
	cases := []struct {
		name        string
		hugoEnabled bool
		inputFile   string
		indexNames  []string
		wantWriteAs string
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
			h := sitegen.SimpleConfig{
				IsEnabled:       c.hugoEnabled,
				IndexFiles:      c.indexNames,
				IndexFileTarget: "_index.md",
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

func TestProcessNodeVitePressIndexRename(t *testing.T) {
	cases := []struct {
		name             string
		vpEnabled        bool
		inputFile        string
		extraIndexNames  []string
		wantWriteAs      string
	}{
		{
			name:        "README.md renamed to index.md when VitePress enabled",
			vpEnabled:   true,
			inputFile:   "README.md",
			extraIndexNames: []string{"readme.md", "README.md"},
			wantWriteAs: "index.md",
		},
		{
			name:        "readme.md (lowercase) renamed to index.md when VitePress enabled",
			vpEnabled:   true,
			inputFile:   "readme.md",
			extraIndexNames: []string{"readme.md", "README.md"},
			wantWriteAs: "index.md",
		},
		{
			name:        "index.md stays index.md when VitePress enabled",
			vpEnabled:   true,
			inputFile:   "index.md",
			wantWriteAs: "index.md",
		},
		{
			name:        "README.md NOT renamed when VitePress disabled",
			vpEnabled:   false,
			inputFile:   "README.md",
			extraIndexNames: []string{"readme.md", "README.md"},
			wantWriteAs: "README.md",
		},
		{
			name:        "guide.md unaffected when VitePress enabled",
			vpEnabled:   true,
			inputFile:   "guide.md",
			extraIndexNames: []string{"readme.md", "README.md"},
			wantWriteAs: "guide.md",
		},
		{
			// Manifest pulls _index.md natively from a Hugo-based external repo.
			// VitePress normalises it to index.md without touching the source repo.
			name:        "_index.md from Hugo external repo normalised to index.md",
			vpEnabled:   true,
			inputFile:   "_index.md",
			wantWriteAs: "index.md",
		},
		{
			name:        "_index.md NOT renamed when VitePress disabled",
			vpEnabled:   false,
			inputFile:   "_index.md",
			wantWriteAs: "_index.md",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := vitepress.NewAdapter(vitepress.VitePress{
				Enabled:        c.vpEnabled,
				IndexFileNames: c.extraIndexNames,
			})
			w := &writersfakes.FakeWriter{}
			dw := document.NewDocumentWorker(
				&linkresolverfakes.FakeInterface{},
				&registryfakes.FakeInterface{},
				cfg,
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
