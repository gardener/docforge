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
	p.refs = make(map[string]*link, 0)
	return &p
}

// Parse scans data and applies callbacks to character patterns
// to model a parsed Document
func (p *parser) Parse(data []byte) Document {
	doc := &document{
		data:  data,
		links: []Link{},
	}
	beg, end := 0, 0

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
		beg = end + consumed
		end = beg
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
	// TODO: leftover block from the original parser.
	if beg < n {
		if data[end-1] == '\n' {
			end--
		}
		// 	ast.AppendChild(currBlock, newTextNode(data[beg:end]))
	}
	return doc
}
