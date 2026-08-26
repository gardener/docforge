// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package titleresolver_test

import (
	"testing"

	"github.com/gardener/docforge/pkg/titleresolver"
)

var defaultIndexNames = []string{"readme.md", "README.md"} //nolint:gochecknoglobals

func TestResolveTitle(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		manifestFM     map[string]interface{}
		docFM          map[string]interface{}
		nodeName       string
		indexFileNames []string
		parentName     string
		isRootIndex    bool
		want           string
	}{
		// ── Priority chain ──────────────────────────────────────────────────────
		{
			name:        "manifest title wins over doc and filename",
			manifestFM:  map[string]interface{}{"title": "Manifest Title"},
			docFM:       map[string]interface{}{"title": "Doc Title"},
			nodeName:    "api-resources.md",
			want:        "Manifest Title",
		},
		{
			name:       "doc title wins over filename when no manifest title",
			manifestFM: map[string]interface{}{},
			docFM:      map[string]interface{}{"title": "Doc Title"},
			nodeName:   "api-resources.md",
			want:       "Doc Title",
		},
		{
			name:     "filename derivation when no manifest or doc title",
			nodeName: "api-resources.md",
			want:     "Api Resources",
		},

		// ── Filename derivation ──────────────────────────────────────────────
		{
			name:     "dashes replaced and Title cased",
			nodeName: "gardener-architecture.md",
			want:     "Gardener Architecture",
		},
		{
			name:     "underscores replaced and Title cased",
			nodeName: "managed_resources.md",
			want:     "Managed Resources",
		},
		{
			name:     "md extension stripped",
			nodeName: "overview.md",
			want:     "Overview",
		},
		{
			name:     "no extension keeps name as-is (Title cased)",
			nodeName: "Overview",
			want:     "Overview",
		},

		// ── Index file resolution ────────────────────────────────────────────
		{
			name:           "README uses parent dir name",
			nodeName:       "README.md",
			indexFileNames: defaultIndexNames,
			parentName:     "installation",
			want:           "Installation",
		},
		{
			name:           "_index.md uses parent dir name",
			nodeName:       "_index.md",
			indexFileNames: defaultIndexNames,
			parentName:     "gardener-operator",
			want:           "Gardener Operator",
		},
		{
			name:           "readme.md (lowercase) uses parent dir name",
			nodeName:       "readme.md",
			indexFileNames: defaultIndexNames,
			parentName:     "operations",
			want:           "Operations",
		},
		{
			name:           "root index returns Root",
			nodeName:       "_index.md",
			indexFileNames: defaultIndexNames,
			isRootIndex:    true,
			want:           "Root",
		},
		{
			name:           "root README returns Root",
			nodeName:       "README.md",
			indexFileNames: defaultIndexNames,
			isRootIndex:    true,
			want:           "Root",
		},
		{
			name:           "index file without parent and not root uses own name (Title-cased)",
			nodeName:       "README.md",
			indexFileNames: defaultIndexNames,
			parentName:     "",
			isRootIndex:    false,
			want:           "Readme", // cases.Title converts "README" → "Readme"
		},

		// ── Nil / empty / whitespace guards ─────────────────────────────────
		{
			name:       "nil manifestFM falls through to docFM",
			manifestFM: nil,
			docFM:      map[string]interface{}{"title": "Doc Title"},
			nodeName:   "foo.md",
			want:       "Doc Title",
		},
		{
			name:     "nil both FMs falls through to filename",
			nodeName: "foo-bar.md",
			want:     "Foo Bar",
		},
		{
			name:       "empty string manifest title skipped",
			manifestFM: map[string]interface{}{"title": ""},
			docFM:      map[string]interface{}{"title": "Doc Title"},
			nodeName:   "foo.md",
			want:       "Doc Title",
		},
		{
			name:       "whitespace-only manifest title skipped",
			manifestFM: map[string]interface{}{"title": "   "},
			docFM:      map[string]interface{}{"title": "Doc Title"},
			nodeName:   "foo.md",
			want:       "Doc Title",
		},
		{
			name:     "empty string doc title skipped to filename",
			docFM:    map[string]interface{}{"title": ""},
			nodeName: "foo-bar.md",
			want:     "Foo Bar",
		},
		{
			name:     "whitespace-only doc title skipped to filename",
			docFM:    map[string]interface{}{"title": "  "},
			nodeName: "foo-bar.md",
			want:     "Foo Bar",
		},
		{
			name:       "non-string manifest title (int) skipped",
			manifestFM: map[string]interface{}{"title": 42},
			docFM:      map[string]interface{}{"title": "Doc Title"},
			nodeName:   "foo.md",
			want:       "Doc Title",
		},
		{
			name:    "non-string doc title (bool) falls to filename",
			docFM:   map[string]interface{}{"title": true},
			nodeName: "foo-bar.md",
			want:    "Foo Bar",
		},
		{
			name:       "manifest title absent (key not present) falls through",
			manifestFM: map[string]interface{}{"weight": 10},
			docFM:      map[string]interface{}{"title": "Doc Title"},
			nodeName:   "foo.md",
			want:       "Doc Title",
		},

		// ── All three levels present ─────────────────────────────────────────
		{
			name:           "all three levels: manifest wins",
			manifestFM:     map[string]interface{}{"title": "Override"},
			docFM:          map[string]interface{}{"title": "From Doc"},
			nodeName:       "README.md",
			indexFileNames: defaultIndexNames,
			parentName:     "installation",
			want:           "Override",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := titleresolver.ResolveTitle(tc.manifestFM, tc.docFM, tc.nodeName, tc.indexFileNames, tc.parentName, tc.isRootIndex)
			if got != tc.want {
				t.Errorf("ResolveTitle() = %q, want %q", got, tc.want)
			}
		})
	}
}
