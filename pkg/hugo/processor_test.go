package hugo

import (
	"bytes"
	"testing"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/util/tests"
	"github.com/stretchr/testify/assert"
)

func init() {
	tests.SetKlogV(6)
}

func TestHugoProcess(t *testing.T) {
	testCases := []struct {
		in      []byte
		want    []byte
		wantErr error
		mutate  func(p *Processor)
	}{
		{
			in:   []byte(`[GitHub](./a/b.md) [anyresource](./a/b.ppt) ![img](./images/img.png) <a href="a.md">A</a> <a href="https://a.com/b.md">B</a> <style src="a.css"/> <style src="https://a.com/b.css"/>`),
			want: []byte("[GitHub](../a/b) [anyresource](../a/b.ppt) ![img](../images/img.png) <a href=\"../a\">A</a> <a href=\"https://a.com/b.md\">B</a> <style src=\"../a.css\"/> <style src=\"https://a.com/b.css\"/>\n"),
			mutate: func(p *Processor) {
				p.PrettyUrls = true
			},
		},
		{
			in:   []byte(`[GitHub](./a/b.md) [anyresource](./a/b.ppt) ![img](./images/img.png) <a href="a.md">A</a> <a href="https://a.com/b.md">B</a> <style src="a.css"/> <style src="https://a.com/b.css"/>`),
			want: []byte("[GitHub](./a/b.html) [anyresource](./a/b.ppt) ![img](./images/img.png) <a href=\"a.html\">A</a> <a href=\"https://a.com/b.md\">B</a> <style src=\"a.css\"/> <style src=\"https://a.com/b.css\"/>\n"),
			mutate: func(p *Processor) {
				p.PrettyUrls = false
			},
		},
	}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			p := &Processor{}
			if tc.mutate != nil {
				tc.mutate(p)
			}
			got, err := p.Process(tc.in, &api.Node{Name: "Test"})

			if tc.wantErr != err {
				t.Errorf("want err %v != %v", tc.wantErr, err)
			}
			assert.Equal(t, string(tc.want), string(got))
		})
	}
}

func TestRewriteDestination(t *testing.T) {
	testCases := []struct {
		name            string
		destination     string
		text            string
		title           string
		nodeName        string
		wantDestination string
		wantText        string
		wantTitle       string
		wantError       error
		mutate          func(h *Processor)
	}{
		{
			"",
			"#fragment-id",
			"",
			"",
			"testnode",
			"#fragment-id",
			"",
			"",
			nil,
			nil,
		},
		{
			"",
			"https://github.com/a/b/sample.md",
			"",
			"",
			"testnode",
			"https://github.com/a/b/sample.md",
			"",
			"",
			nil,
			nil,
		},
		{
			"",
			"./a/b/sample.md",
			"",
			"",
			"testnode",
			"../a/b/sample",
			"",
			"",
			nil,
			func(h *Processor) {
				h.PrettyUrls = true
			},
		},
		{
			"",
			"./a/b/README.md",
			"",
			"",
			"testnode",
			"../a/b",
			"",
			"",
			nil,
			func(h *Processor) {
				h.PrettyUrls = true
				h.IndexFileNames = []string{"readme", "read.me", "index", "_index"}
			},
		},
		{
			"",
			"./a/b/README.md",
			"",
			"",
			"testnode",
			"./a/b/README.html",
			"",
			"",
			nil,
			nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p := &Processor{}
			if tc.mutate != nil {
				tc.mutate(p)
			}

			gotDestination, gotText, gotTitle, gotErr := p.rewriteDestination([]byte(tc.destination), []byte(tc.text), []byte(tc.title), tc.nodeName)

			if gotErr != tc.wantError {
				t.Errorf("want error %v != %v", gotErr, tc.wantError)
			}
			if !bytes.Equal(gotDestination, []byte(tc.wantDestination)) {
				t.Errorf("want destination %v != %v", string(gotDestination), tc.wantDestination)
			}
			if !bytes.Equal(gotText, []byte(tc.wantText)) {
				t.Errorf("want text %v != %v", string(gotText), tc.wantText)
			}
			if !bytes.Equal(gotTitle, []byte(tc.wantTitle)) {
				t.Errorf("want title %v != %v", string(gotTitle), tc.wantTitle)
			}
		})
	}
}
