// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package parser

// Parser is the interface for parsing data into Document
type Parser interface {
	Parse(data []byte) Document
}

// OnLinkListed is a callback function invoked by the
// ListLinks iterator in Document
type OnLinkListed func(link Link) (Link, error)

// Document is the markdown model parsed from bytes data
type Document interface {
	// ListLinks iterates parsed links in this document
	// and invokes cb on every link. Should the link be
	// removed, which is signaled by OnLinkListed returning
	// nil, ListLinks will update the document links list
	// referenced by each link.
	ListLinks(cb OnLinkListed) error
	// Bytes returns the  parsed document content bytes
	Bytes() []byte
}

// Link is a markdown link which can be a normal link or an
// embedded image reference
type Link interface {
	SetText(text []byte)
	SetDestination(text []byte)
	SetTitle(text []byte)
	GetText() []byte
	GetDestination() []byte
	GetTitle() []byte
	Remove(leaveText bool)
	IsImage() bool
	IsAutoLink() bool
	IsNormalLink() bool
}
