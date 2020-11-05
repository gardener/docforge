// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package parser

type linkType int

const (
	linkNormal linkType = iota
	linkImg
	linkAuto
	linkDeferredFootnote
	// linkInlineFootnote
	// linkCitation
)

type bytesRange struct {
	start int
	end   int
}

type link struct {
	document    *document
	start       int
	end         int
	text        *bytesRange
	destination *bytesRange
	title       *bytesRange
	linkType    linkType
}

func offsetSiblingsByteRanges(l *link, offset int) {
	var needOffset bool
	for _, _l := range l.document.links {
		if _l == l {
			needOffset = true
			continue
		}
		if needOffset {
			offsetLinkByteRanges(_l.(*link), offset)
		}
	}
}

func offsetLinkByteRanges(link *link, offset int) {
	link.start = offset + link.start
	link.end = offset + link.end
	if link.text != nil {
		link.text.start = offset + link.text.start
		link.text.end = offset + link.text.end
	}
	if link.destination != nil {
		link.destination.start = offset + link.destination.start
		link.destination.end = offset + link.destination.end
	}
	if link.title != nil {
		link.title.start = offset + link.title.start
		link.title.end = offset + link.title.end
	}
}

func replaceBytes(doc []byte, start, end int, text []byte) []byte {
	doc1 := doc[:start]
	doc2 := doc[end:]
	docUpdate := append([]byte{}, doc1...)
	docUpdate = append(docUpdate, text...)
	docUpdate = append(docUpdate, doc2...)
	return docUpdate
}

func (l *link) SetText(text []byte) {
	if l.text == nil {
		return
	}
	offset := len(text) - (l.text.end - l.text.start)
	l.document.data = replaceBytes(l.document.data, l.text.start, l.text.end, text)
	// offset next link components, if any
	if l.destination != nil {
		l.destination.start = offset + l.destination.start
		l.destination.end = offset + l.destination.end
	}
	if l.title != nil {
		l.title.start = offset + l.title.start
		l.title.end = offset + l.title.end
	}
	offsetSiblingsByteRanges(l, offset)
}

func (l *link) GetText() []byte {
	if l.text == nil {
		return nil
	}
	return l.document.data[l.text.start:l.text.end]
}

func (l *link) SetDestination(text []byte) {
	if l.destination == nil {
		return
	}
	offset := len(text) - (l.destination.end - l.destination.start)
	l.document.data = replaceBytes(l.document.data, l.destination.start, l.destination.end, text)
	// offset next link components, if any
	if l.title != nil {
		l.title.start = offset + l.title.start
		l.title.end = offset + l.title.end
	}
	offsetSiblingsByteRanges(l, offset)
}

func (l *link) GetDestination() []byte {
	if l.destination == nil {
		return nil
	}
	return l.document.data[l.destination.start:l.destination.end]
}

func (l *link) SetTitle(text []byte) {
	if l.title == nil {
		return
	}
	offset := len(text) - (l.title.end - l.title.start)
	l.document.data = replaceBytes(l.document.data, l.title.start, l.title.end, text)
	offsetSiblingsByteRanges(l, offset)
}

func (l *link) GetTitle() []byte {
	if l.title == nil {
		return nil
	}
	return l.document.data[l.title.start:l.title.end]
}

func (l *link) Remove(leaveText bool) {
	text := []byte("")
	if l.linkType != linkImg && leaveText {
		text = l.GetText()
	}
	doc1 := l.document.data[:l.start]
	doc2 := l.document.data[l.end:]
	l.document.data = append([]byte{}, doc1...)
	l.document.data = append(l.document.data, text...)
	l.document.data = append(l.document.data, doc2...)
	var (
		needsOffset         bool
		offset              int
		removedElementIndex int
	)
	if len(text) > 0 {
		offset = len(text) - (l.end - l.start)
	} else {
		offset = l.start - l.end
	}
	for i := 0; i < len(l.document.links); i++ {
		if l.document.links[i] == l {
			removedElementIndex = i
			needsOffset = true
			continue
		}
		if needsOffset {
			offsetLinkByteRanges(l.document.links[i].(*link), offset)
		}
	}
	l.document.links = remove(l.document.links, removedElementIndex)
}

func remove(slice []Link, s int) []Link {
	return append(slice[:s], slice[s+1:]...)
}

func (l *link) IsImage() bool {
	return l.linkType == linkImg
}

func (l *link) IsAutoLink() bool {
	return l.linkType == linkAuto
}

func (l *link) IsNormalLink() bool {
	return l.linkType == linkNormal
}
