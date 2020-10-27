package parser

type document struct {
	data  []byte
	links []Link
}

func (d *document) ListLinks(cb OnLinkListed) {
	if cb != nil && d.links != nil {
		for _, l := range d.links {
			cb(l)
		}
	}
}

func (d *document) Bytes() []byte {
	return d.data
}
