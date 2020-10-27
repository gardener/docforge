package parser

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseLinks(t *testing.T) {
	testCases := []struct {
		in string
		// 1st tuple: link [start:end]
		// 2nd tuple: text [start:end]
		// 3d tuple: destination [start:end]
		// 4th tuple: title [start:end]
		want [][]int
	}{
		{
			`[link](/uri)`,
			[][]int{
				[]int{0, 12},
				[]int{1, 5},
				[]int{7, 11},
			},
		},
		{
			`[link](/uri "title")`,
			[][]int{
				[]int{0, 20},
				[]int{1, 5},
				[]int{7, 11},
				[]int{13, 18},
			},
		},
		{
			`[link](/uri 'title')`,
			[][]int{
				[]int{0, 20},
				[]int{1, 5},
				[]int{7, 11},
				[]int{13, 18},
			},
		},
		// {
		// 	// Not supported
		// 	`[link](/uri (title))`,
		// 	nil,
		// },
		{
			`[link]()`,
			[][]int{
				[]int{0, 8},
				[]int{1, 5},
			},
		},
		{
			`[link](<>)`,
			[][]int{
				[]int{0, 10},
				[]int{1, 5},
			},
		},
		{
			`[link](</my uri>)`,
			[][]int{
				[]int{0, 17},
				[]int{1, 5},
				[]int{8, 15},
			},
		},
		// {
		// 	`[a](<b)c>)`,
		// 	[][]int{
		// 		[]int{0, 9},
		// 		[]int{1, 2},
		// 		[]int{5, 7},
		// 	},
		// },
		// {
		// 	`[link](\(foo\))`,
		// 	[][]int{
		// 		[]int{0, 14},
		// 		[]int{1, 5},
		// 		[]int{9, 11},
		// 	},
		// },
		// {
		// 	`[link](foo(and(bar)))`,
		// 	[][]int{
		// 		[]int{0, 21},
		// 		[]int{1, 5},
		// 		[]int{7, 20},
		// 	},
		// },
		{
			`[link](foo\(and\(bar\))`,
			[][]int{
				[]int{0, 23},
				[]int{1, 5},
				[]int{7, 22},
			},
		},
		// {
		// 	`[link](<foo(and(bar)>)`,
		// 	[][]int{
		// 		[]int{0, 22},
		// 		[]int{1, 5},
		// 		[]int{7, 21},
		// 	},
		// },
		{
			`[link](foo\)\:)`,
			[][]int{
				[]int{0, 15},
				[]int{1, 5},
				[]int{7, 14},
			},
		},
		{
			`[link](#fragment)`,
			[][]int{
				[]int{0, 17},
				[]int{1, 5},
				[]int{7, 16},
			},
		},
		{
			`[link](http://example.com#fragment)`,
			[][]int{
				[]int{0, 35},
				[]int{1, 5},
				[]int{7, 34},
			},
		},
		{
			`[link](http://example.com?foo=3#frag)`,
			[][]int{
				[]int{0, 37},
				[]int{1, 5},
				[]int{7, 36},
			},
		},
		{
			`[link](/url 'title "and" title')`,
			[][]int{
				[]int{0, 32},
				[]int{1, 5},
				[]int{7, 11},
				[]int{13, 30},
			},
		},
		{
			`  [a](b.com   'c' )  `,
			[][]int{
				[]int{2, 19},
				[]int{3, 4},
				[]int{6, 11},
				[]int{15, 16},
			},
		},
		{
			`  [a](<b.com>)  `,
			[][]int{
				[]int{2, 14},
				[]int{3, 4},
				[]int{7, 12},
			},
		},
		{
			`  [a](<b.com> 'c')  `,
			[][]int{
				[]int{2, 18},
				[]int{3, 4},
				[]int{7, 12},
				[]int{15, 16},
			},
		},
		{
			`  [ a ](  b.com)`,
			[][]int{
				[]int{2, 16},
				[]int{4, 5},
				[]int{10, 15},
			},
		},
		{
			`  [ a 
 ](  b.com)`,
			[][]int{
				[]int{2, 18},
				[]int{4, 5},
				[]int{12, 17},
			},
		},
		{
			`[link](   /uri
  "title"  )`,
			[][]int{
				[]int{0, 27},
				[]int{1, 5},
				[]int{10, 14},
				[]int{18, 23},
			},
		},
		{
			`[link [foo [bar]]](/uri)`,
			[][]int{
				[]int{0, 24},
				[]int{1, 17},
				[]int{19, 23},
			},
		},
		// {
		// 	`[link [bar](/uri)`,
		// 	[][]int{
		// 		[]int{6, 17},
		// 		[]int{7, 10},
		// 		[]int{12, 16},
		// 	},
		// },
		{
			`[link \[bar](/uri)`,
			[][]int{
				[]int{0, 18},
				[]int{1, 11},
				[]int{13, 17},
			},
		},
		{
			"[link *foo **bar** `#`*](/uri)",
			[][]int{
				[]int{0, 30},
				[]int{1, 23},
				[]int{25, 29},
			},
		},
	}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			p := NewParser()
			want := &link{}
			for i, tuple := range tc.want {
				switch i {
				case 0:
					{
						want.start = tuple[0]
						want.end = tuple[1]
					}
				case 1:
					{
						want.text = &bytesRange{
							start: tuple[0],
							end:   tuple[1],
						}
					}
				case 2:
					{
						want.destination = &bytesRange{
							start: tuple[0],
							end:   tuple[1],
						}
					}
				case 3:
					{
						want.title = &bytesRange{
							start: tuple[0],
							end:   tuple[1],
						}
					}
				}
			}
			zeroValue := &link{}
			if *want == *zeroValue {
				want = nil
			}
			s := strings.Split(tc.in, " ")
			offset := 0
			for _, w := range s {
				if len(w) > 0 {
					break
				}
				offset++
			}
			_, got := parseLink(p.(*parser), []byte(tc.in), offset)
			if assert.Equal(t, want, got) {
				if got == nil {
					fmt.Println("|nil|")
				} else {
					l := got.(*link)
					fmt.Printf("|%s|\n", string([]byte(tc.in)[l.start:l.end]))
				}
			}
		})
	}
}

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
