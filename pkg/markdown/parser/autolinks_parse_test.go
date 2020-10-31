package parser

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseAutoLinks(t *testing.T) {
	testCases := []struct {
		in string
		// 0: link start
		// 1: link end
		want [][]int
	}{
		{
			`  <https://a.com>  `,
			[][]int{
				[]int{2, 17},
				[]int{3, 16},
			},
		},
		{
			`  <https://a.com>.  `,
			[][]int{
				[]int{2, 17},
				[]int{3, 16},
			},
		},
		{
			`  <https://a.com#q?a=b&c=3>.  `,
			[][]int{
				[]int{2, 27},
				[]int{3, 26},
			},
		},
		{
			`  <www.a.com>  `,
			nil,
		},
		{
			`  <./a.com>  `,
			nil,
		},
		{
			`  <  https://a.com   >  `,
			nil,
		},
		{
			`  <mailto://a@mail.com>  `,
			[][]int{
				[]int{2, 23},
				[]int{3, 22},
			},
		},
		{
			`  <a@mail.com>  `,
			[][]int{
				[]int{2, 14},
				[]int{3, 13},
			},
		},
		{
			`  <  mailto://a@mail.com  >  `,
			nil,
		},
	}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			p := NewParser()
			var want *link
			if tc.want != nil {
				want = &link{
					start: tc.want[0][0],
					end:   tc.want[0][1],
					destination: &bytesRange{
						start: tc.want[1][0],
						end:   tc.want[1][1],
					},
					linkType: linkAuto,
				}
			}
			s := strings.Split(tc.in, " ")
			offset := 0
			for _, w := range s {
				if len(w) > 0 {
					break
				}
				offset++
			}
			_, got := parseLeftAngle(p.(*parser), []byte(tc.in), offset)
			if tc.want == nil && assert.Nil(t, got) {
				return
			}
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

func TestParseAutoLinksExtended(t *testing.T) {
	testCases := []struct {
		in string
		// 0: link start
		// 1: link end
		want []int
	}{
		{
			`  https://a.com  `,
			[]int{2, 15},
		},
		{
			`  https://a.com.  `,
			[]int{2, 15},
		},
		{
			`  https://a.com#q?a=b&c=3.  `,
			[]int{2, 25},
		},
		{
			`  ./a.com  `,
			[]int{2, 9},
		},
		{
			`  www.a.com  `,
			nil,
		},
		{
			`  a.com  `,
			nil,
		},
		{
			"  https:\n//a.com  ",
			nil,
		},
		{
			`  (https://a.com?a=b).  `,
			nil,
		},
		{
			`  mailto://a@mail.com  `,
			[]int{2, 21},
		},
		{
			`  a@mail.com  `,
			nil,
		},
	}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			p := NewParser()
			var want *link
			if tc.want != nil {
				want = &link{
					start: tc.want[0],
					end:   tc.want[1],
					destination: &bytesRange{
						start: tc.want[0],
						end:   tc.want[1],
					},
					linkType: linkAuto,
				}
			}
			s := strings.Split(tc.in, " ")
			offset := 0
			for _, w := range s {
				if len(w) > 0 {
					break
				}
				offset++
			}
			_, got := parseAutoLink(p.(*parser), []byte(tc.in), offset)
			if tc.want == nil && assert.Nil(t, got) {
				return
			}
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
