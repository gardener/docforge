package parser

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListLinkRewrites(t *testing.T) {
	testCases := []struct {
		in     *document
		listCb OnLinkListed
		want   []byte
	}{
		{
			&document{
				[]byte("some intro [b](c.com) some text [b1](c1.com)"),
				[]Link{
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
			func(l Link) {
				if bytes.Equal(l.GetText(), []byte("b")) {
					l.SetDestination([]byte("e.com"))
					l.SetText([]byte("d"))
				}
				if bytes.Equal(l.GetText(), []byte("b1")) {
					l.SetDestination([]byte("c2.com"))
				}
			},
			[]byte("some intro [d](e.com) some text [b1](c2.com)"),
		},
		{
			&document{
				[]byte("some intro [b](c.com)\n`x`some text [b1](c1.com)"),
				[]Link{
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
						start: 35,
						end:   47,
						text: &bytesRange{
							start: 36,
							end:   38,
						},
						destination: &bytesRange{
							start: 40,
							end:   46,
						},
					},
				},
			},
			func(l Link) {
				if bytes.Equal(l.GetText(), []byte("b")) {
					l.SetDestination([]byte("e.com"))
					l.SetText([]byte("d"))
				}
				if bytes.Equal(l.GetText(), []byte("b1")) {
					l.SetDestination([]byte("c2.com"))
				}
			},
			[]byte("some intro [d](e.com)\n`x`some text [b1](c2.com)"),
		},
		{
			&document{
				[]byte(`some intro [b](c.com)
   some text [b1](c1.com)`),
				[]Link{
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
						start: 35,
						end:   47,
						text: &bytesRange{
							start: 36,
							end:   38,
						},
						destination: &bytesRange{
							start: 40,
							end:   46,
						},
					},
				},
			},
			func(l Link) {
				if bytes.Equal(l.GetText(), []byte("b")) {
					l.SetDestination([]byte("e.com"))
					l.SetText([]byte("d"))
				}
				if bytes.Equal(l.GetText(), []byte("b1")) {
					l.SetDestination([]byte("c2.com"))
				}
			},
			[]byte(`some intro [d](e.com)
   some text [b1](c2.com)`),
		},
		{
			&document{
				[]byte("some intro [b](c.com)\n`x`some text [b1](c1.com)"),
				[]Link{
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
						start: 35,
						end:   47,
						text: &bytesRange{
							start: 36,
							end:   38,
						},
						destination: &bytesRange{
							start: 40,
							end:   46,
						},
					},
				},
			},
			func(l Link) {
				if bytes.Equal(l.GetText(), []byte("b")) {
					l.Remove(false)
				}
			},
			[]byte("some intro \n`x`some text [b1](c1.com)"),
		},
		{
			&document{
				[]byte("some intro [b](c.com)\n`x`some text [b1](c1.com)"),
				[]Link{
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
						start: 35,
						end:   47,
						text: &bytesRange{
							start: 36,
							end:   38,
						},
						destination: &bytesRange{
							start: 40,
							end:   46,
						},
					},
				},
			},
			func(l Link) {
				if bytes.Equal(l.GetText(), []byte("b")) {
					l.Remove(true)
				}
			},
			[]byte("some intro b\n`x`some text [b1](c1.com)"),
		},
	}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			for _, l := range tc.in.links {
				l.(*link).document = tc.in
			}
			tc.in.ListLinks(tc.listCb)
			got := tc.in.Bytes()
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestLinkReplaceBytes(t *testing.T) {
	testCases := []struct {
		in    []byte
		start int
		end   int
		text  []byte
		want  []byte
	}{
		{
			[]byte("some intro [a](b.com) some text"),
			15,
			20,
			[]byte("d.com"),
			[]byte("some intro [a](d.com) some text"),
		},
		{
			[]byte("[a](b.com) some text"),
			1,
			2,
			[]byte("d"),
			[]byte("[d](b.com) some text"),
		},
		{
			[]byte("some intro [a](b.com)"),
			12,
			13,
			[]byte("d"),
			[]byte("some intro [d](b.com)"),
		},
		{
			[]byte(`Eligendi id aut consequatur sed odit. Aut sit et eaque
ut facilis minima alias dolorum. Provident [explicabo](a.com)
et culpa rerum non soluta.`),
			110,
			115,
			[]byte("d.com"),
			[]byte(`Eligendi id aut consequatur sed odit. Aut sit et eaque
ut facilis minima alias dolorum. Provident [explicabo](d.com)
et culpa rerum non soluta.`),
		},
	}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			got := replaceBytes(tc.in, tc.start, tc.end, tc.text)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestOffsetLinkByteRange(t *testing.T) {
	testCases := []struct {
		link     *link
		offset   int
		wantLink *link
	}{
		{
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
			10,
			&link{
				start: 21,
				end:   31,
				text: &bytesRange{
					start: 22,
					end:   23,
				},
				destination: &bytesRange{
					start: 25,
					end:   30,
				},
			},
		},
		{
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
			-10,
			&link{
				start: 1,
				end:   11,
				text: &bytesRange{
					start: 2,
					end:   3,
				},
				destination: &bytesRange{
					start: 5,
					end:   10,
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			offsetLinkByteRanges(tc.link, tc.offset)
			assert.Equal(t, tc.wantLink, tc.link)
		})
	}
}
