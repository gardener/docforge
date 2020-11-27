// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"bytes"
)

func maybeImage(p *parser, data []byte, offset int) (int, Link) {
	if offset < len(data)-1 && data[offset+1] == '[' {
		return parseLink(p, data, offset)
	}
	return 0, nil
}

// '[': parse a link or an image or a footnote or a citation
func parseLink(p *parser, data []byte, offset int) (int, Link) {
	// no links allowed inside regular links, footnote, and deferred footnotes
	if p.insideLink && (offset > 0 && data[offset-1] == '[' || len(data)-1 > offset && data[offset+1] == '^') {
		return 0, nil
	}

	var t linkType
	switch {
	// special case: ![^text] == deferred footnote (that follows something with
	// an exclamation point)
	// case p.extensions&Footnotes != 0 && len(data)-1 > offset && data[offset+1] == '^':
	// 	t = linkDeferredFootnote
	// ![alt] == image
	case offset >= 0 && data[offset] == '!':
		t = linkImg
		offset++
	// [@citation], [@-citation], [@?citation], [@!citation]
	// case p.extensions&Mmark != 0 && len(data)-1 > offset && data[offset+1] == '@':
	// 	t = linkCitation
	// [text] == regular link
	// ^[text] == inline footnote
	// [^refId] == deferred footnote
	// case p.extensions&Footnotes != 0:
	// 	if offset >= 0 && data[offset] == '^' {
	// 		t = linkInlineFootnote
	// 		offset++
	// 	} else if len(data)-1 > offset && data[offset+1] == '^' {
	// 		t = linkDeferredFootnote
	// 	}
	default:
		t = linkNormal
	}

	data = data[offset:]

	// if t == linkCitation {
	// 	return citation(p, data, 0)
	// }

	var (
		i = 1
		// noteID                                         int
		/*title,*/
		// link, linkID, altContent []byte
		end, linkB, linkE, txtB, txtE, titleB, titleE int
		textHasNl                                     = false
	)
	start := offset
	i = skipSpace(data, i)
	txtB = i

	// if t == linkDeferredFootnote {
	// 	i++
	// }

	// look for the matching closing bracket
	for level := 1; level > 0 && i < len(data); i++ {
		switch {
		case data[i] == '\n':
			textHasNl = true

		case data[i-1] == '\\':
			continue

		case data[i] == '[':
			level++

		case data[i] == ']':
			level--
			if level <= 0 {
				i-- // compensate for extra i++ in for loop
			}
		}
	}

	if i >= len(data) {
		return 0, nil
	}

	txtE = i
	// remove whitespace at the end of the text
	for txtE > txtB && isSpace(data[txtE-1]) {
		txtE--
	}
	i++
	// var footnoteNode ast.Node

	// skip any amount of whitespace or newline
	// (this is much more lax than original markdown syntax)
	i = skipSpace(data, i)

	// inline style link
	switch {
	case i < len(data) && data[i] == '(':
		// skip initial whitespace
		i++

		i = skipSpace(data, i)

		linkB = i
		brace := 0

		// look for link end: ' " )
	findlinkend:
		for i < len(data) {
			switch {
			case data[i] == '\\':
				i += 2

			case data[i] == '(':
				brace++
				i++

			case data[i] == ')':
				if brace <= 0 {
					break findlinkend
				}
				brace--
				i++

			case data[i] == '\'' || data[i] == '"':
				break findlinkend

			default:
				i++
			}
		}
		if i >= len(data) {
			return 0, nil
		}
		linkE = i

		// look for title end if present
		titleB, titleE = 0, 0
		if data[i] == '\'' || data[i] == '"' {
			i++
			titleB = i
			titleEndCharFound := false

		findtitleend:
			for i < len(data) {
				switch {
				case data[i] == '\\':
					i++

				case data[i] == data[titleB-1]: // matching title delimiter
					titleEndCharFound = true

				case titleEndCharFound && data[i] == ')':
					end = i
					break findtitleend
				}
				i++
			}

			if i >= len(data) {
				return 0, nil
			}

			linkE = titleB - 1

			// skip whitespace after title
			titleE = i - 1
			for titleE > titleB && isSpace(data[titleE]) {
				titleE--
			}

			// check for closing quote presence
			if data[titleE] != '\'' && data[titleE] != '"' {
				titleB, titleE = 0, 0
				linkE = i
			}
		} else {
			end = linkE
		}

		if len(data) >= end+1 {
			end++
		}

		// remove whitespace at the end of the link
		for linkE > linkB && isSpace(data[linkE-1]) {
			linkE--
		}

		// link whitespace not allowed if not in <>
		if data[linkB] != '<' && data[linkE-1] != '>' {
			if bytes.ContainsAny(data[linkB:linkE], " ") {
				return 0, nil
			}
		}

		// remove optional angle brackets around the link
		if data[linkB] == '<' {
			linkB++
		}
		if data[linkE-1] == '>' {
			linkE--
		}

		// build escaped link and title
		// if linkE > linkB {
		// 	link = data[linkB:linkE]
		// }

		// if titleE > titleB {
		// 	title = data[titleB:titleE]
		// }

		i++

	// reference style link
	// case isReferenceStyleLink(data, i, t):
	// 	var id []byte
	// 	altContentConsidered := false

	// 	// look for the id
	// 	i++
	// 	linkB = i
	// 	i = skipUntilChar(data, i, ']')

	// 	if i >= len(data) {
	// 		return 0, nil
	// 	}
	// 	linkE := i

	// 	// find the reference
	// 	if linkB == linkE {
	// 		if textHasNl {
	// 			var b bytes.Buffer

	// 			for j := 1; j < txtE; j++ {
	// 				switch {
	// 				case data[j] != '\n':
	// 					b.WriteByte(data[j])
	// 				case data[j-1] != ' ':
	// 					b.WriteByte(' ')
	// 				}
	// 			}

	// 			id = b.Bytes()
	// 		} else {
	// 			id = data[1:txtE]
	// 			altContentConsidered = true
	// 		}
	// 	} else {
	// 		id = data[linkB:linkE]
	// 	}

	// 	// find the reference with matching id
	// 	lr, ok := p.getRef(string(id))
	// 	if !ok {
	// 		return 0, nil
	// 	}

	// 	// keep link and title from reference
	// 	linkID = id
	// 	link = lr.link
	// 	title = lr.title
	// 	if altContentConsidered {
	// 		altContent = lr.text
	// 	}
	// 	i++

	// shortcut reference style link or reference or inline footnote
	default:
		// var id []byte

		// craft the id
		if textHasNl {
			var b bytes.Buffer

			for j := 1; j < txtE; j++ {
				switch {
				case data[j] != '\n':
					b.WriteByte(data[j])
				case data[j-1] != ' ':
					b.WriteByte(' ')
				}
			}

			// id = b.Bytes()
		} else {
			// if t == linkDeferredFootnote {
			// 	id = data[2:txtE] // get rid of the ^
			// } else {
			// 	id = data[1:txtE]
			// }
		}

		// footnoteNode = &ast.ListItem{}
		// if t == linkInlineFootnote {
		// 	// create a new reference
		// 	noteID = len(p.notes) + 1

		// 	var fragment []byte
		// 	if len(id) > 0 {
		// 		if len(id) < 16 {
		// 			fragment = make([]byte, len(id))
		// 		} else {
		// 			fragment = make([]byte, 16)
		// 		}
		// 		copy(fragment, slugify(id))
		// 	} else {
		// 		fragment = append([]byte("footnote-"), []byte(strconv.Itoa(noteID))...)
		// 	}

		// 	ref := &reference{
		// 		noteID:   noteID,
		// 		hasBlock: false,
		// 		link:     fragment,
		// 		title:    id,
		// 		footnote: footnoteNode,
		// 	}

		// 	p.notes = append(p.notes, ref)
		// 	p.refsRecord[string(ref.link)] = struct{}{}

		// 	link = ref.link
		// 	title = ref.title
		// }
		// } else {
		// 	// find the reference with matching id
		// 	lr, ok := p.getRef(string(id))
		// 	if !ok {
		// 		return 0, nil
		// 	}

		// 	if t == linkDeferredFootnote && !p.isFootnote(lr) {
		// 		lr.noteID = len(p.notes) + 1
		// 		lr.footnote = footnoteNode
		// 		p.notes = append(p.notes, lr)
		// 		p.refsRecord[string(lr.link)] = struct{}{}
		// 	}

		// 	// keep link and title from reference
		// 	link = lr.link
		// 	// if inline footnote, title == footnote contents
		// 	title = lr.title
		// 	noteID = lr.noteID
		// 	if len(lr.text) > 0 {
		// 		altContent = lr.text
		// 	}
		// }

		// rewind the whitespace
		i = txtE + 1
	}

	// var uLink []byte
	// link = data[linkB:linkE]
	// if t == linkNormal || t == linkImg {
	// 	if len(link) > 0 {
	// 		var uLinkBuf bytes.Buffer
	// 		unescapeText(&uLinkBuf, link)
	// 		uLink = uLinkBuf.Bytes()
	// 	}

	// 	// links need something to click on and somewhere to go
	// 	if len(uLink) == 0 || (t == linkNormal && txtE <= 1) {
	// 		return 0, nil
	// 	}
	// }

	// call the relevant rendering function
	switch t {
	case linkNormal:
		if txtE-txtB <= 0 {
			return i, nil
		}
		if linkE-linkB <= 0 {
			return i, nil
		}
		maybeTitle := &bytesRange{
			start: offset + titleB,
			end:   offset + titleE,
		}
		if titleB == titleE {
			maybeTitle = nil
		}
		maybeDestination := &bytesRange{
			start: offset + linkB,
			end:   offset + linkE,
		}
		if linkB == linkE {
			maybeDestination = nil
		}
		link := &link{
			start: start,
			end:   offset + end,
			text: &bytesRange{
				start: offset + txtB,
				end:   offset + txtE,
			},
			linkType: linkNormal,
			// &LiteralComponent{
			// normalizeURI(uLink),
			// },
			// title:       title,
			// DeferredID:  linkID,
		}
		link.title = maybeTitle
		link.destination = maybeDestination
		// if len(altContent) > 0 {
		// 	ast.AppendChild(link, newTextNode(altContent))
		// } else {
		// links cannot contain other links, so turn off link parsing
		// temporarily and recurse
		insideLink := p.insideLink
		p.insideLink = true
		p.Parse(data[1:txtE])
		p.insideLink = insideLink
		// }
		return i, link

	case linkImg:
		if linkE-linkB <= 0 {
			return i, nil
		}
		image := &link{
			start: start,
			end:   offset + end,
			text: &bytesRange{
				start: offset + txtB,
				end:   offset + txtE,
			},
			destination: &bytesRange{
				start: offset + linkB,
				end:   offset + linkE,
			},
			title: &bytesRange{
				start: offset + titleB,
				end:   offset + titleE,
			},
			linkType: linkImg,
		}
		return i + 1, image

	// case linkInlineFootnote, linkDeferredFootnote:
	// 	link := &ast.Link{
	// 		destination: link,
	// 		title:       title,
	// 		NoteID:      noteID,
	// 		Footnote:    footnoteNode,
	// 	}
	// 	if t == linkDeferredFootnote {
	// 		link.DeferredID = data[2:txtE]
	// 	}
	// 	if t == linkInlineFootnote {
	// 		i++
	// 	}
	// 	return i, link

	default:
		return 0, nil
	}
}

func normalizeURI(s []byte) []byte {
	return s // TODO: implement
}

func isReferenceStyleLink(data []byte, pos int, t linkType) bool {
	if t == linkDeferredFootnote {
		return false
	}
	return pos < len(data)-1 && data[pos] == '[' && data[pos+1] != '^'
}
