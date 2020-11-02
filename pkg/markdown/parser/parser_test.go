// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	testCases := []struct {
		in   []byte
		want *document
	}{
		{
			[]byte("some intro [b](c.com) some text [b1](c1.com)"),
			&document{
				links: []Link{
					&link{
						start: 11,
						end:   21,
						text: &bytesRange{
							start: 12,
							end:   13,
						},
						destination: &bytesRange{
							start: 15,
							end:   20,
						},
					},
					&link{
						start: 32,
						end:   44,
						text: &bytesRange{
							start: 33,
							end:   35,
						},
						destination: &bytesRange{
							start: 37,
							end:   43,
						},
					},
				},
			},
		},
		{
			[]byte(`some intro [b](c.com) 
			some text [b1](c1.com)`),
			&document{
				links: []Link{
					&link{
						start: 11,
						end:   21,
						text: &bytesRange{
							start: 12,
							end:   13,
						},
						destination: &bytesRange{
							start: 15,
							end:   20,
						},
					},
					&link{
						start: 36,
						end:   48,
						text: &bytesRange{
							start: 37,
							end:   39,
						},
						destination: &bytesRange{
							start: 41,
							end:   47,
						},
					},
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			tc.want.data = tc.in
			for _, l := range tc.want.links {
				l.(*link).document = tc.want
			}
			p := NewParser()
			got := p.Parse(tc.in)
			assert.Equal(t, tc.want, got)
		})
	}
}
