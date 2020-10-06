package hugo

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/gardener/docforge/pkg/api"
)

func TestHugoProcess(t *testing.T) {
	var (
		in, got, expected []byte
		err               error
	)
	in = []byte("[GitHub](./a/b.md) ![img](./images/img.png)")
	expected = []byte("[GitHub](../a/b) ![img](../images/img.png)\n")
	p := &Processor{
		PrettyUrls: true,
	}
	if got, err = p.Process(in, &api.Node{Name: "Test"}); err != nil {
		t.Errorf("%v!=nil", err)
	}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("`%v`\n!=\n`%v`", string(expected), string(got))
	}
}

func TestRewriteDestination(t *testing.T) {
	testCases := []struct {
		name            string
		destination     string
		nodeName        string
		wantDestination string
		wantError       error
		mutate          func(h *Processor)
	}{
		{
			"",
			"#fragment-id",
			"testnode",
			"#fragment-id",
			nil,
			nil,
		},
		{
			"",
			"https://github.com/a/b/sample.md",
			"testnode",
			"https://github.com/a/b/sample.md",
			nil,
			nil,
		},
		{
			"",
			"./a/b/sample.md",
			"testnode",
			"../a/b/sample",
			nil,
			func(h *Processor) {
				h.PrettyUrls = true
			},
		},
		{
			"",
			"./a/b/README.md",
			"testnode",
			"../a/b",
			nil,
			func(h *Processor) {
				h.PrettyUrls = true
				h.IndexFileNames = []string{"readme", "read.me", "index", "_index"}
			},
		},
		{
			"",
			"./a/b/README.md",
			"testnode",
			"./a/b/README.html",
			nil,
			nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h := &Processor{}
			if tc.mutate != nil {
				tc.mutate(h)
			}

			gotDestination, gotErr := h.rewriteDestination([]byte(tc.destination), tc.nodeName)
			if gotErr != tc.wantError {
				t.Errorf("expected error %v != %v", gotErr, tc.wantError)
			}
			if !bytes.Equal(gotDestination, []byte(tc.wantDestination)) {
				t.Errorf("expected destination %v != %v", string(gotDestination), tc.wantDestination)
			}
		})
	}
}
