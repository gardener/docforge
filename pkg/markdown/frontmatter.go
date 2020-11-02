// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package markdown

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"strings"
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
