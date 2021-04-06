// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package processors

import (
	"github.com/gardener/docforge/pkg/api"
)

type Document struct {
	Node          *api.Node
	DocumentBytes []byte
	Links         []*Link
	FrontMatter   []byte
}

type Link struct {
	DestinationNode     *api.Node
	Resource            bool
	OriginalDestination string
	AbsLink             *string
	Destination         *string
	Text                *string
	Title               *string
}

func (d *Document) Append(bytes []byte) {
	if d.DocumentBytes == nil {
		d.DocumentBytes = bytes
		return
	}
	d.DocumentBytes = append(d.DocumentBytes, bytes...)
}

// TODO: adding to frontmatter multiple times should merge instead of override
func (d *Document) AddFrontMatter(frontMatter []byte) {
	if d.FrontMatter == nil {
		d.FrontMatter = make([]byte, 0)
	}
	d.FrontMatter = append(d.FrontMatter, frontMatter...)
}

func (d *Document) AddLink(link *Link) {
	if d.Links == nil {
		d.Links = make([]*Link, 0)
	}

	d.Links = append(d.Links, link)
}

func (d *Document) GetLinkByDestination(destination string) *Link {
	for _, l := range d.Links {
		if *l.Destination == string(destination) {
			return l
		}
	}
	return nil
}
