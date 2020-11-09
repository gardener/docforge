// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package parser

type document struct {
	data  []byte
	links []Link
}

func (d *document) ListLinks(cb UpdateMarkdownLinkListed) {
	if cb != nil && d.links != nil {
		for _, l := range d.links {
			cb(l)
		}
	}
}

func (d *document) Bytes() []byte {
	return d.data
}
