// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package parser

type document struct {
	data  []byte
	links []Link
}

func (d *document) ListLinks(cb OnLinkListed) error {
	if cb != nil && d.links != nil {
		i := 0
		for _, l := range d.links {
			_l, err := cb(l)
			if err != nil {
				return err
			}
			if _l != nil {
				// Copy over only links that were not removed
				d.links[i] = _l
				i++
			}
		}
		// Erase truncated values
		for j := i; j < len(d.links); j++ {
			d.links[j] = nil
		}
		d.links = d.links[:i]
	}
	return nil
}

func (d *document) Bytes() []byte {
	return d.data
}
