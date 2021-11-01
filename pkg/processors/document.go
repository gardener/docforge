// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package processors

import (
	"github.com/gardener/docforge/pkg/api"
)

// Document represents a markdown for the Node
type Document struct {
	Node          *api.Node
	DocumentBytes []byte
	Links         []*Link
	FrontMatter   []byte
}

// Link defines a markdown link
type Link struct {
	DestinationNode     *api.Node
	IsResource          bool
	OriginalDestination string
	AbsLink             *string
	Destination         *string
	Text                *string
	Title               *string
}

// Append adds content to the document
func (d *Document) Append(bytes []byte) {
	if d.DocumentBytes == nil {
		d.DocumentBytes = bytes
		return
	}
	d.DocumentBytes = append(d.DocumentBytes, bytes...)
}

// AddFrontMatter adds front matter content to the document
// TODO: adding to frontmatter multiple times should merge instead of override
func (d *Document) AddFrontMatter(frontMatter []byte) {
	if d.FrontMatter == nil {
		d.FrontMatter = make([]byte, 0)
	}
	d.FrontMatter = append(d.FrontMatter, frontMatter...)
}

// AddLink adds link to the document
func (d *Document) AddLink(link *Link) {
	if d.Links == nil {
		d.Links = make([]*Link, 0)
	}

	d.Links = append(d.Links, link)
}

// GetLinkByDestination returns the links for a given destination
func (d *Document) GetLinkByDestination(destination string) *Link {
	for _, l := range d.Links {
		if *l.Destination == string(destination) {
			return l
		}
	}
	return nil
}
