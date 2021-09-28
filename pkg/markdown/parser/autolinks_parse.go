// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"bytes"
	"regexp"
)

// autolinkType specifies a kind of autolink that gets detected.
type autolinkType int

// These are the possible flag values for the autolink renderer.
const (
	notAutolink autolinkType = iota
	normalAutolink
	emailAutolink
)

var (
	urlRe    = `((https?|ftp):\/\/|\/)[-A-Za-z0-9+&@#\/%?=~_|!:,.;\(\)]+`
	anchorRe = regexp.MustCompile(`^(<a\shref="` + urlRe + `"(\stitle="[^"<>]+")?\s?>` + urlRe + `<\/a>)`)

	// TODO: improve this regexp to catch all possible entities:
	htmlEntityRe = regexp.MustCompile(`&[a-z]{2,5};`)
)

// '<' when tags or autolinks are allowed
func parseLeftAngle(p *parser, data []byte, offset int) (int, Link) {
	data = data[offset:]
	start := offset
	altype, end := tagLength(data)
	// if size := p.inlineHTMLComment(data); size > 0 {
	// 	end = size
	// }
	if end <= 2 {
		return end, nil
	}
	if altype == notAutolink {
		// htmlTag := &ast.HTMLSpan{}
		// htmlTag.Literal = data[:end]
		return end, nil
	}

	var uLink bytes.Buffer
	unescapeText(&uLink, data[1:end+1-2])
	if uLink.Len() <= 0 {
		return end, nil
	}
	// link := uLink.Bytes()
	node := &link{
		// uLink.Bytes(),
		start: start,
		end:   offset + end,
		destination: &bytesRange{
			start: start + 1,
			end:   offset + end - 1,
		},
		linkType: linkAuto,
	}
	// if altype == emailAutolink {
	// 	node.Destination = append([]byte("mailto:"), link...)
	// }
	// link = stripMailto(link)
	return end, node
}

// '\\' backslash escape
var escapeChars = []byte("\\`*_{}[]()#+-.!:|&<>~^")

func escape(p *Parser, data []byte, offset int) (int, []byte) {
	data = data[offset:]

	if len(data) <= 1 {
		return 2, nil
	}

	// if p.extensions&NonBlockingSpace != 0 && data[1] == ' ' {
	// 	return 2, &ast.NonBlockingSpace{}
	// }

	// if p.extensions&BackslashLineBreak != 0 && data[1] == '\n' {
	// 	return 2, &ast.Hardbreak{}
	// }

	if bytes.IndexByte(escapeChars, data[1]) < 0 {
		return 0, nil
	}

	return 2, data[1:2]
}

// '&' escaped when it doesn't belong to an entity
// valid entities are assumed to be anything matching &#?[A-Za-z0-9]+;
func entity(p *Parser, data []byte, offset int) (int, []byte) {
	data = data[offset:]

	end := skipCharN(data, 1, '#', 1)
	end = skipAlnum(data, end)

	if end < len(data) && data[end] == ';' {
		end++ // real entity
	} else {
		return 0, nil // lone '&'
	}

	ent := data[:end]
	// undo &amp; escaping or it will be converted to &amp;amp; by another
	// escaper in the renderer
	if bytes.Equal(ent, []byte("&amp;")) {
		ent = []byte{'&'}
	}

	return end, ent
}

func linkEndsWithEntity(data []byte, linkEnd int) bool {
	entityRanges := htmlEntityRe.FindAllIndex(data[:linkEnd], -1)
	return entityRanges != nil && entityRanges[len(entityRanges)-1][1] == linkEnd
}

// hasPrefixCaseInsensitive is a custom implementation of
//     strings.HasPrefix(strings.ToLower(s), prefix)
// we rolled our own because ToLower pulls in a huge machinery of lowercasing
// anything from Unicode and that's very slow. Since this func will only be
// used on ASCII protocol prefixes, we can take shortcuts.
func hasPrefixCaseInsensitive(s, prefix []byte) bool {
	if len(s) < len(prefix) {
		return false
	}
	delta := byte('a' - 'A')
	for i, b := range prefix {
		if b != s[i] && b != s[i]+delta {
			return false
		}
	}
	return true
}

var protocolPrefixes = [][]byte{
	[]byte("http://"),
	[]byte("https://"),
	[]byte("ftp://"),
	[]byte("file://"),
	[]byte("mailto:"),
}

const shortestPrefix = 6 // len("ftp://"), the shortest of the above

func maybeAutoLink(p *parser, data []byte, offset int) (int, Link) {
	// quick check to rule out most false hits
	if p.insideLink || len(data) < offset+shortestPrefix {
		return 0, nil
	}
	for _, prefix := range protocolPrefixes {
		endOfHead := offset + 8 // 8 is the len() of the longest prefix
		if endOfHead > len(data) {
			endOfHead = len(data)
		}
		if hasPrefixCaseInsensitive(data[offset:endOfHead], prefix) {
			return parseAutoLink(p, data, offset)
		}
	}
	return 0, nil
}

func codeSpan(p *parser, data []byte, offset int) (int, Link) {
	var backtickCount, i, end int
	data = data[offset:]

	for backtickCount < len(data) && data[backtickCount] == '`' {
		backtickCount++
	}

	for end = backtickCount; end < len(data) && i < backtickCount; end++ {
		if data[end] == '`' {
			i++
		} else {
			i = 0
		}
	}

	// no matching delimiter?
	if i < backtickCount && end >= len(data) {
		return 0, nil
	}

	// trim outside whitespace
	fBegin := backtickCount
	for fBegin < end && data[fBegin] == ' ' {
		fBegin++
	}

	fEnd := end - backtickCount
	for fEnd > fBegin && data[fEnd-1] == ' ' {
		fEnd--
	}
	return end, nil
}

func parseAutoLink(p *parser, data []byte, offset int) (int, Link) {
	// Now a more expensive check to see if we're not inside an anchor element
	anchorStart := offset
	offsetFromAnchor := 0
	for anchorStart > 0 && data[anchorStart] != '<' {
		anchorStart--
		offsetFromAnchor++
	}

	anchorStr := anchorRe.Find(data[anchorStart:])
	if anchorStr != nil {
		// anchorClose := &ast.HTMLSpan{}
		// anchorClose.Literal = anchorStr[offsetFromAnchor:]
		return len(anchorStr) - offsetFromAnchor, nil
	}

	// scan backward for a word boundary
	rewind := 0
	for offset-rewind > 0 && rewind <= 7 && isLetter(data[offset-rewind-1]) {
		rewind++
	}
	if rewind > 6 { // longest supported protocol is "mailto" which has 6 letters
		return 0, nil
	}

	origData := data
	data = data[offset-rewind:]

	if !isSafeLink(data) {
		return 0, nil
	}

	linkB := offset - rewind
	linkEnd := 0
	for linkEnd < len(data) && !isEndOfLink(data[linkEnd]) {
		linkEnd++
	}

	// Skip punctuation at the end of the link
	if (data[linkEnd-1] == '.' || data[linkEnd-1] == ',' || data[linkEnd-1] == '?' || data[linkEnd-1] == '!') && data[linkEnd-2] != '\\' {
		linkEnd--
	}

	// But don't skip semicolon if it's a part of escaped entity:
	if data[linkEnd-1] == ';' && data[linkEnd-2] != '\\' && !linkEndsWithEntity(data, linkEnd) {
		linkEnd--
	}

	// See if the link finishes with a punctuation sign that can be closed.
	var copen byte
	switch data[linkEnd-1] {
	case '"':
		copen = '"'
	case '\'':
		copen = '\''
	case ')':
		copen = '('
	case ']':
		copen = '['
	case '}':
		copen = '{'
	default:
		copen = 0
	}

	if copen != 0 {
		bufEnd := offset - rewind + linkEnd - 2

		openDelim := 1

		/* Try to close the final punctuation sign in this same line;
		 * if we managed to close it outside of the URL, that means that it's
		 * not part of the URL. If it closes inside the URL, that means it
		 * is part of the URL.
		 *
		 * Examples:
		 *
		 *      foo http://www.pokemon.com/Pikachu_(Electric) bar
		 *              => http://www.pokemon.com/Pikachu_(Electric)
		 *
		 *      foo (http://www.pokemon.com/Pikachu_(Electric)) bar
		 *              => http://www.pokemon.com/Pikachu_(Electric)
		 *
		 *      foo http://www.pokemon.com/Pikachu_(Electric)) bar
		 *              => http://www.pokemon.com/Pikachu_(Electric))
		 *
		 *      (foo http://www.pokemon.com/Pikachu_(Electric)) bar
		 *              => foo http://www.pokemon.com/Pikachu_(Electric)
		 */

		for bufEnd >= 0 && origData[bufEnd] != '\n' && openDelim != 0 {
			if origData[bufEnd] == data[linkEnd-1] {
				openDelim++
			}

			if origData[bufEnd] == copen {
				openDelim--
			}

			bufEnd--
		}

		if openDelim == 0 {
			linkEnd--
		}
	}

	for ; data[linkEnd-1] == '*' || data[linkEnd-1] == '_' || data[linkEnd-1] == '`'; linkEnd-- {
	}

	var uLink bytes.Buffer
	unescapeText(&uLink, data[:linkEnd])

	if uLink.Len() > 0 {
		node := &link{
			// uLink.Bytes(),
			start: linkB,
			end:   offset + linkEnd,
			destination: &bytesRange{
				start: linkB,
				end:   offset + linkEnd,
			},
			linkType: linkAuto,
		}
		return linkEnd, node
	}

	return linkEnd, nil
}

func isEndOfLink(char byte) bool {
	return isSpace(char) || char == '<'
}

var validUris = [][]byte{[]byte("http://"), []byte("https://"), []byte("ftp://"), []byte("mailto://")}
var validPaths = [][]byte{[]byte("/"), []byte("./"), []byte("../")}

func isSafeLink(link []byte) bool {
	nLink := len(link)
	for _, path := range validPaths {
		nPath := len(path)
		linkPrefix := link[:nPath]
		if nLink >= nPath && bytes.Equal(linkPrefix, path) {
			if nLink == nPath {
				return true
			} else if isAlnum(link[nPath]) {
				return true
			}
		}
	}

	for _, prefix := range validUris {
		// TODO: handle unicode here
		// case-insensitive prefix test
		nPrefix := len(prefix)
		if nLink > nPrefix {
			linkPrefix := bytes.ToLower(link[:nPrefix])
			if bytes.Equal(linkPrefix, prefix) && isAlnum(link[nPrefix]) {
				return true
			}
		}
	}

	return false
}

// return the length of the given tag, or 0 is it's not valid
func tagLength(data []byte) (autolink autolinkType, end int) {
	var i, j int

	// a valid tag can't be shorter than 3 chars
	if len(data) < 3 {
		return notAutolink, 0
	}

	// begins with a '<' optionally followed by '/', followed by letter or number
	if data[0] != '<' {
		return notAutolink, 0
	}
	if data[1] == '/' {
		i = 2
	} else {
		i = 1
	}

	if !isAlnum(data[i]) {
		return notAutolink, 0
	}

	// scheme test
	autolink = notAutolink

	// try to find the beginning of an URI
	for i < len(data) && (isAlnum(data[i]) || data[i] == '.' || data[i] == '+' || data[i] == '-') {
		i++
	}

	if i > 1 && i < len(data) && data[i] == '@' {
		if j = isMailtoAutoLink(data[i:]); j != 0 {
			return emailAutolink, i + j
		}
	}

	if i > 2 && i < len(data) && data[i] == ':' {
		autolink = normalAutolink
		i++
	}

	// complete autolink test: no whitespace or ' or "
	switch {
	case i >= len(data):
		autolink = notAutolink
	case autolink != notAutolink:
		j = i

		for i < len(data) {
			if data[i] == '\\' {
				i += 2
			} else if data[i] == '>' || data[i] == '\'' || data[i] == '"' || isSpace(data[i]) {
				break
			} else {
				i++
			}

		}

		if i >= len(data) {
			return autolink, 0
		}
		if i > j && data[i] == '>' {
			return autolink, i + 1
		}

		// one of the forbidden chars has been found
		autolink = notAutolink
	}
	i += bytes.IndexByte(data[i:], '>')
	if i < 0 {
		return autolink, 0
	}
	return autolink, i + 1
}

// look for the address part of a mail autolink and '>'
// this is less strict than the original markdown e-mail address matching
func isMailtoAutoLink(data []byte) int {
	nb := 0

	// address is assumed to be: [-@._a-zA-Z0-9]+ with exactly one '@'
	for i, c := range data {
		if isAlnum(c) {
			continue
		}

		switch c {
		case '@':
			nb++

		case '-', '.', '_':
			break

		case '>':
			if nb == 1 {
				return i + 1
			}
			return 0
		default:
			return 0
		}
	}

	return 0
}
