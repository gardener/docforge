// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package markdown

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

// ErrFrontMatterNotClosed is raised to signal
// that the rules for defining a frontmatter element
// in a markdown document have been violated
var ErrFrontMatterNotClosed error = errors.New("Missing closing frontmatter `---` found")

// StripFrontMatter splits a provided document into front-matter
// and content.
func StripFrontMatter(b []byte) ([]byte, []byte, error) {
	var (
		started      bool
		yamlBeg      int
		yamlEnd      int
		contentStart int
	)

	buf := bytes.NewBuffer(b)

	for {
		line, err := buf.ReadString('\n')

		if errors.Is(err, io.EOF) {
			// handle documents that contain only forntmatter
			// and no line ending after closing ---
			if started && yamlEnd == 0 {
				if l := strings.TrimSpace(line); l == "---" {
					yamlEnd = len(b) - buf.Len() - len([]byte(line))
					contentStart = len(b)
				}

			}
			break
		}

		if err != nil {
			return nil, nil, err
		}

		if l := strings.TrimSpace(line); l != "---" {
			// Only whitespace is acceptable before front-matter
			// Any other preceding text is interpeted as frontmater-less
			// document
			if !started && len(l) > 0 {
				return nil, b, nil
			}
			continue
		}

		if !started {
			started = true
			yamlBeg = len(b) - buf.Len()
		} else {
			yamlEnd = len(b) - buf.Len() - len([]byte(line))
			contentStart = yamlEnd + len([]byte(line))
			break
		}
	}

	if started && yamlEnd == 0 {
		return nil, nil, ErrFrontMatterNotClosed
	}

	fm := b[yamlBeg:yamlEnd]
	content := b[contentStart:]

	return fm, content, nil
}

// InsertFrontMatter prepends the content bytes with
// front matter enclosed in the standard marks ---
func InsertFrontMatter(fm []byte, content []byte) ([]byte, error) {
	var (
		data []byte
		err  error
	)
	if len(fm) < 1 {
		return content, nil
	}
	buf := bytes.NewBuffer([]byte("---\n"))
	buf.Write(fm)
	// TODO: configurable empty line after frontmatter
	buf.WriteString("---\n")
	buf.Write(content)
	if data, err = ioutil.ReadAll(buf); err != nil {
		return nil, err
	}
	return data, nil
}

// MatchFrontMatterRules inspects content `b` for compliance with `frontMatter`
// and `excludeFrontMatter`.
func MatchFrontMatterRules(b []byte, frontMatter map[string]interface{}, excludeFrontMatter map[string]interface{}) (bool, error) {
	fmBytes, _, err := StripFrontMatter(b)
	if err != nil {
		return false, err
	}
	if fmBytes == nil {
		return false, nil
	}
	fm := map[string]interface{}{}
	if err := yaml.Unmarshal(fmBytes, fm); err != nil {
		return false, err
	}
	selected := false
	for path, v := range frontMatter {
		if MatchFrontMatterRule(path, v, fm) {
			selected = true
			break
		}
	}
	for path, v := range excludeFrontMatter {
		if MatchFrontMatterRule(path, v, fm) {
			selected = false
			break
		}
	}
	return selected, nil
}

// MatchFrontMatterRule explores a parsed frontmatter object `data` to matchFMRule
// `value` at `path` pattern and return true on successfull matchFMRule or false
// otherwise.
// Path is an expression with a JSONPath-like simplified notation.
// An object in path is modeled as dot (`.`). Paths start with the root object,
// i.e. the most minimal path is `.`.
// An object element value is referenced by its name (key) in the object map:
// `.a.b.c` is path to element `c` in map `b` in map `a` in root object map.
// Element values can be scalar, object maps or arrays.
// An element in an array is referenced by its index: `.a.b[1]` references `b`
// array element with index 1.
// Paths can include up to one wildcard `**` symbol that models *any* path node.
// A `.a.**.c` models any path starting with	`.a.` and ending with `.c`.
func MatchFrontMatterRule(path string, val interface{}, data interface{}) bool {
	return matchFMRule(path, val, nil, data)
}

func matchFMRule(pathPattern string, val interface{}, path []string, data interface{}) bool {
	if path == nil {
		path = []string{"."}
	}
	p := strings.Join(path, "")
	if _matchFMPath(pathPattern, p) {
		if reflect.DeepEqual(val, data) {
			return true
		}
	}
	switch vv := data.(type) {
	case []interface{}:
		for i, u := range vv {
			_p := append(path, fmt.Sprintf("[%d]", i))
			if ok := matchFMRule(pathPattern, val, _p, u); ok {
				return true
			}
		}
	case map[string]interface{}:
		for k, u := range vv {
			if path[(len(path))-1] != "." {
				path = append(path, ".")
			}
			_p := append(path, k)
			if ok := matchFMRule(pathPattern, val, _p, u); ok {
				return true
			}
		}
	}
	return false
}

func _matchFMPath(pathPattern, path string) bool {
	if pathPattern == path {
		return true
	}
	s := strings.Split(pathPattern, "**")
	if len(s) > 1 {
		if strings.HasPrefix(path, s[0]) {
			if len(s[1]) == 0 || strings.HasSuffix(path, s[1]) {
				return true
			}
		}
	}
	return false
}
