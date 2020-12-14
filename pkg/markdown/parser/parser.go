// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package parser

// for each character that triggers a response when parsing inline data.
type inlineParser func(p *parser, data []byte, offset int) (int, Link)

// Parser embeds properties used by the Parse operation
type parser struct {
	insideLink     bool
	inlineCallback [256]inlineParser
	refs           map[string]*link
}

// NewParser creates Parser objects
func NewParser() Parser {
	p := parser{}
	p.inlineCallback['['] = parseLink
	p.inlineCallback['!'] = maybeImage
	p.inlineCallback['<'] = parseLeftAngle
	p.inlineCallback['h'] = maybeAutoLink
	p.inlineCallback['m'] = maybeAutoLink
	p.inlineCallback['f'] = maybeAutoLink
	p.inlineCallback['H'] = maybeAutoLink
	p.inlineCallback['M'] = maybeAutoLink
	p.inlineCallback['F'] = maybeAutoLink
	p.inlineCallback['`'] = codeSpan
	p.refs = make(map[string]*link, 0)
	return &p
}

// Parse scans data and applies callbacks to character patterns
// to model a parsed Document
func (p *parser) Parse(data []byte) Document {
	var end int
	doc := &document{
		data:  data,
		links: []Link{},
	}

	n := len(data)
	for end < n {
		handler := p.inlineCallback[data[end]]
		if handler == nil {
			end++
			continue
		}
		consumed, node := handler(p, data, end)
		if consumed == 0 {
			// no action from the callback
			end++
			continue
		}
		end += consumed
		if node != nil {
			switch v := node.(type) {
			case *link:
				{
					v.document = doc
					doc.links = append(doc.links, node)
				}
			}
		}
	}
	return doc
}
