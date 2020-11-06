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

// OnLink is a callback function invoked on each link
// by mardown#UpdateLinkRefs
// It is supplied a link and is expected to return destination,
// text, title or error.
// A nil destination will yield removing of this link/image markup,
// leaving only the text component if it's a link
// Nil text or title returned yield no change. Any other value replaces
// the original. If a returned title is empty string an originally
// existing title element will be completely removed
type OnLink func(markdownType Type, destination, text, title []byte) ([]byte, []byte, []byte, error)

// UpdateLinkRefs changes document links destinations, consulting
// with callback on the destination to use on each link or image in document.
// If a callback returns "" for a destination, this is interpreted as
// request to remove the link destination and leave only the link text or in
// case it's an image - to remove it completely.
// TODO: failfast vs fault tolerance support?
func UpdateLinkRefs(documentBlob []byte, callback OnLink) ([]byte, error) {
	p := parser.NewParser()
	document := p.Parse(documentBlob)
	if callback == nil {
		return nil, nil
	}
	document.ListLinks(func(l parser.Link) {
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
			return
		}
		updateLink(l, destination, text, title)
	})
	return document.Bytes(), nil
}

func updateLink(link parser.Link, destination, text, title []byte) {
	if destination == nil {
		link.Remove(text != nil && len(text) > 0)
		return
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
}
