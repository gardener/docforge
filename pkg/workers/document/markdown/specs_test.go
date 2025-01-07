// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package markdown_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/gardener/docforge/pkg/workers/document/markdown"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	gmhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"golang.org/x/net/html"
)

type spec struct {
	Markdown string `json:"markdown"`
	HTML     string `json:"html"`
	Example  int    `json:"example"`
}

type specs []spec

// TestCommonmark executes commonmark compliance tests
func TestCommonmark(t *testing.T) {
	testSpecs, err := loadSpec("commonmark_v.0.30.json")
	if err != nil {
		t.Errorf("error load specs %v", err)
	}
	for _, tc := range testSpecs {
		t.Run(fmt.Sprintf("example %d", tc.Example), tc.executeSpecTest)
	}
}

func TestGFM(t *testing.T) {
	testSpecs, err := loadSpec("gfm_v.0.29.json")
	if err != nil {
		t.Errorf("error load specs %v", err)
	}
	for _, tc := range testSpecs {
		t.Run(fmt.Sprintf("example %d", tc.Example), tc.executeSpecTest)
	}
}

func TestCMarkGFMAfl(t *testing.T) {
	tc := &spec{
		Markdown: "# H1\n\nH2\n--\n\nt ☺  \n*b* **em** `c`\n&ge;\\&\\\n\\_e\\_\n\n4) I1\n\n5) I2\n   > [l](/u \"t\")\n   >\n   > - [f]\n   > - ![a](/u \"t\")\n   >\n   >> <ftp://hh>\n   >> <u@hh>\n\n~~~ l☺\ncb\n~~~\n\n    c1\n    c2\n\n***\n\n<div>\n<b>x</b>\n</div>\n\n| a | b |\n| --- | --- |\n| c | `d|` \\| e |\n\ngoogle ~~yahoo~~\n\ngoogle.com http://google.com google@google.com\n\nand <xmp> but\n\n<surewhynot>\nsure\n</surewhynot>\n\n[f]: /u \"t\"\n",
		HTML:     "<h1>H1</h1>\n<h2>H2</h2>\n<p>t ☺<br>\n<em>b</em> <strong>em</strong> <code>c</code>\n≥&amp;<br>\n_e_</p>\n<ol start=\"4\">\n<li>\n<p>I1</p>\n</li>\n<li>\n<p>I2</p>\n<blockquote>\n<p><a href=\"/u\" title=\"t\">l</a></p>\n<ul>\n<li><a href=\"/u\" title=\"t\">f</a></li>\n<li><img src=\"/u\" alt=\"a\" title=\"t\"></li>\n</ul>\n<blockquote>\n<p><a href=\"ftp://hh\">ftp://hh</a>\n<a href=\"mailto:u@hh\">u@hh</a></p>\n</blockquote>\n</blockquote>\n</li>\n</ol>\n<pre><code class=\"language-l☺\">cb\n</code></pre>\n<pre><code>c1\nc2\n</code></pre>\n<hr>\n<div>\n<b>x</b>\n</div>\n<table>\n<thead>\n<tr>\n<th>a</th>\n<th>b</th>\n</tr>\n</thead>\n<tbody>\n<tr>\n<td>c</td>\n<td>`d</td>\n</tr>\n</tbody>\n</table>\n<p>google <del>yahoo</del></p>\n<p>google.com <a href=\"http://google.com\">http://google.com</a> <a href=\"mailto:google@google.com\">google@google.com</a></p>\n<p>and <xmp> but</p>\n<surewhynot>\nsure\n</surewhynot>\n",
		Example:  1,
	}
	tc.executeSpecTest(t)
}

func loadSpec(specName string) ([]spec, error) {
	specFile := "spec/" + specName
	specBytes, err := os.ReadFile(specFile)
	if err != nil {
		return nil, fmt.Errorf("error reading spec file %s: %v", specFile, err)
	}
	testSpecs := make(specs, 0)
	err = json.Unmarshal(specBytes, &testSpecs)
	if err != nil {
		return nil, fmt.Errorf("error unmarshal specs: %v", err)
	}
	return testSpecs, nil
}

func (tc *spec) executeSpecTest(t *testing.T) {
	// Goldmark Markdown with GFM extensions
	var gm = goldmark.New(goldmark.WithRendererOptions(gmhtml.WithUnsafe()), goldmark.WithExtensions(extension.GFM))
	// Link modifier renderer
	var lmr = markdown.NewLinkModifierRenderer()
	var (
		doc ast.Node
		err error
	)
	doc = gm.Parser().Parse(text.NewReader([]byte(tc.Markdown)))
	if doc == nil {
		t.Errorf("parsing example %d returns nil", tc.Example)
	}
	buf := &bytes.Buffer{}
	err = lmr.Render(buf, []byte(tc.Markdown), doc)
	if err != nil {
		fmt.Println("", tc.Example, tc.Markdown)
		t.Errorf("render example %d fails with: %v", tc.Example, err)
	}
	_doc := gm.Parser().Parse(text.NewReader(buf.Bytes()))
	if _doc == nil {
		t.Errorf("parsing rendered example %d returns nil", tc.Example)
	}
	buf1, buf2 := &bytes.Buffer{}, &bytes.Buffer{}
	err = gm.Convert([]byte(tc.Markdown), buf1)
	if err != nil {
		t.Errorf("convert example %d fails: %v", tc.Example, err)
	}
	err = gm.Convert(buf.Bytes(), buf2)
	if err != nil {
		t.Errorf("convert rendered example %d fails: %v", tc.Example, err)
	}
	if !bytes.Equal(buf1.Bytes(), buf2.Bytes()) {
		// try with tc HTML
		if !bytes.Equal([]byte(tc.HTML), buf2.Bytes()) {
			// try to normalize HTMLs
			var hn1, hn2 *html.Node
			hn1, err = html.Parse(bytes.NewBuffer([]byte(tc.HTML)))
			if err != nil {
				t.Errorf("parse HTML for example %d fails: %v", tc.Example, err)
			}
			hn2, err = html.Parse(buf2)
			if err != nil {
				t.Errorf("parse HTML for rendered example %d fails: %v", tc.Example, err)
			}
			hBuf1, hBuf2 := &bytes.Buffer{}, &bytes.Buffer{}
			err = html.Render(hBuf1, hn1)
			if err != nil {
				t.Errorf("render HTML node for example %d fails: %v", tc.Example, err)
			}
			err = html.Render(hBuf2, hn2)
			if err != nil {
				t.Errorf("render HTML node for rendered example %d fails: %v", tc.Example, err)
			}
			if !bytes.Equal(hBuf1.Bytes(), hBuf2.Bytes()) {
				t.Errorf("compare HTML results for example %d fails", tc.Example)
			}
		}
	}
}
