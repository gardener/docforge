package markdown

import (
	"errors"
	"testing"
)

func TestStripFrontMatter(t *testing.T) {

	in := `---
title: Test
prop: A
---

# Head 1
`
	wantFM := `title: Test
prop: A
`
	wantContent := `
# Head 1
`
	var wantErr error

	fm, c, err := StripFrontMatter([]byte(in))
	if err != wantErr {
		t.Errorf("expected err %v != %v", err, wantErr)
	}
	if string(fm) != wantFM {
		t.Errorf("\nwant:\n%s\ngot:\n%s\n", wantFM, fm)
	}
	if string(c) != wantContent {
		t.Errorf("\nwant:\n%s\ngot:\n%s\n", wantContent, c)
	}
}

func TestStripFrontMatterNoFM(t *testing.T) {

	in := `# Head 1`
	wantFM := ""
	wantContent := `# Head 1`
	var wantErr error

	fm, c, err := StripFrontMatter([]byte(in))
	if err != wantErr {
		t.Errorf("expected err %v != %v", err, wantErr)
	}
	if string(fm) != wantFM {
		t.Errorf("\nwant:\n%s\ngot:\n%s\n", wantFM, fm)
	}
	if string(c) != wantContent {
		t.Errorf("\nwant:\n%s\ngot:\n%s\n", wantContent, c)
	}
}

func TestStripFrontMatterErr(t *testing.T) {

	in := `
---
Title: A
`
	wantFM := ""
	wantContent := ""
	wantErr := errors.New("Missing closing front-matter `---` mark found")

	fm, c, err := StripFrontMatter([]byte(in))
	if err.Error() != wantErr.Error() {
		t.Errorf("expected err %v != %v", err, wantErr)
	}
	if string(fm) != wantFM {
		t.Errorf("\nwant:\n%s\ngot:\n%s\n", wantFM, fm)
	}
	if string(c) != wantContent {
		t.Errorf("\nwant:\n%s\ngot:\n%s\n", wantContent, c)
	}
}

func TestStripFrontMatterNoErr(t *testing.T) {

	in := `
Some text

---
`
	wantFM := ""
	wantContent := `
Some text

---
`

	fm, c, err := StripFrontMatter([]byte(in))
	if err != nil {
		t.Errorf("expected err nil != %v", err)
	}
	if string(fm) != wantFM {
		t.Errorf("\nwant:\n%s\ngot:\n%s\n", wantFM, fm)
	}
	if string(c) != wantContent {
		t.Errorf("\nwant:\n%s\ngot:\n%s\n", wantContent, c)
	}
}
