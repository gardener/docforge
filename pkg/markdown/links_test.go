// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package markdown

import (
	"testing"

	"github.com/gardener/docforge/pkg/markdown/parser"
	"github.com/stretchr/testify/assert"
)

func TestUpdateMarkdownLinks(t *testing.T) {
	testCases := []struct {
		in       []byte
		cb       UpdateMarkdownLink
		wantBlob []byte
		wantErr  error
	}{
		{
			[]byte(`[abc](https://abc.xyz/a/b/c.md)`),
			func(markdownType Type, destination, text, title []byte) ([]byte, []byte, []byte, error) {
				return []byte("https://abc.xyz/a/b/c/d/e/f/g.md"), []byte("abcdefg"), nil, nil
			},
			[]byte(`[abcdefg](https://abc.xyz/a/b/c/d/e/f/g.md)`),
			nil,
		},
		{
			[]byte(`[abcdefg](https://abc.xyz/a/b/c/d/e/f/g.md)`),
			func(markdownType Type, destination, text, title []byte) ([]byte, []byte, []byte, error) {
				return []byte("https://abc.xyz/a/b/c.md"), []byte("abc"), nil, nil
			},
			[]byte(`[abc](https://abc.xyz/a/b/c.md)`),
			nil,
		},
		{
			[]byte(`[abc](https://abc.xyz/a/b/c.md)`),
			func(markdownType Type, destination, text, title []byte) ([]byte, []byte, []byte, error) {
				return []byte("https://abc.xyz/a/b/c.md"), []byte("abc"), nil, nil
			},
			[]byte(`[abc](https://abc.xyz/a/b/c.md)`),
			nil,
		},
		{
			[]byte(`[abc](https://abc.xyz/a/b/c.md)`),
			func(markdownType Type, destination, text, title []byte) ([]byte, []byte, []byte, error) {
				return []byte("https://abc.xyz/a/b/d.md"), []byte("abd"), nil, nil
			},
			[]byte(`[abd](https://abc.xyz/a/b/d.md)`),
			nil,
		},
	}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {

			p := parser.NewParser()
			document := p.Parse(tc.in)
			gotBlob, gotErr := UpdateMarkdownLinks(document, tc.cb)
			assert.Equal(t, string(tc.wantBlob), string(gotBlob))
			if tc.wantErr != nil {
				assert.Error(t, gotErr)
			} else {
				assert.Nil(t, gotErr)
			}
		})
	}
}
