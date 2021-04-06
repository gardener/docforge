// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package markdown

import (
	"bytes"
	"fmt"

	"github.com/gardener/docforge/pkg/markdown/parser"
	// "github.com/gomarkdown/markdown"
	// "github.com/gomarkdown/markdown/ast"
	// "github.com/gomarkdown/markdown/parser"
)

const (
	// extensions = parser.CommonExtensions | parser.AutoHeadingIDs

	// Link is a link markdown type
	Link Type = iota
	// Image is an image markdown type
	Image
)

// Type is an enumeration for markdown types
type Type int

func (m Type) String() string {
	return [...]string{"link", "image"}[m]
}

// NewType creates a markdown Type enum from string
func NewType(markdownTypeString string) (Type, error) {
	switch markdownTypeString {
	case "link":
		return Link, nil
	case "image":
		return Image, nil
	}
	return 0, fmt.Errorf("Unknown markdown type string '%s'. Must be one of %v", markdownTypeString, []string{"link", "image"})
}

// UpdateMarkdownLink is a callback function invoked on each link
// by mardown#UpdateMarkdownLinks
// It is supplied link attributes and is expected to return them, potentially
// updated, or error.
// A nil destination will yield removing of this link/image markup,
// leaving only the text component if it's a link
// Nil text or title returned yield no change. Any other value replaces
// the original. If a returned title is empty string an originally
// existing title element will be completely removed
type UpdateMarkdownLink func(markdownType Type, destination, text, title []byte) ([]byte, []byte, []byte, error)

// UpdateMarkdownLinks changes document links destinations, consulting
// with callback on the destination to use on each link or image in document.
// If a callback returns "" for a destination, this is interpreted as
// request to remove the link destination and leave only the link text or in
// case it's an image - to remove it completely.
// TODO: failfast vs fault tolerance support?
func UpdateMarkdownLinks(document parser.Document, callback UpdateMarkdownLink) ([]byte, error) {
	if callback == nil {
		return nil, nil
	}
	err := document.ListLinks(func(l parser.Link) (parser.Link, error) {
		var (
			destination, text, title []byte
			err                      error
			t                        Type
		)
		if l.IsImage() {
			t = Image
		} else {
			t = Link
		}
		text = l.GetText()
		if destination, text, title, err = callback(t, l.GetDestination(), text, l.GetTitle()); err != nil {
			return nil, err
		}
		return updateLink(l, destination, text, title), nil
	})
	if err != nil {
		return nil, err
	}
	return document.Bytes(), err
}

func updateLink(link parser.Link, destination, text, title []byte) parser.Link {
	if destination == nil {
		link.Remove(text != nil && len(text) > 0)
		return nil
	}
	if text != nil && !bytes.Equal(link.GetText(), text) {
		link.SetText(text)
	}
	if destination != nil && !bytes.Equal(link.GetDestination(), destination) {
		link.SetDestination(destination)
	}
	if title != nil && !bytes.Equal(link.GetTitle(), title) {
		link.SetTitle(title)
	}
	return link
}
