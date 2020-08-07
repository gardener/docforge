package reactor

import (
	"fmt"

	"github.com/gardener/docode/pkg/api"
)

func (r *Reactor) PreProcess(contentBytes []byte, source string, node *api.Node) error {
	if h := r.Handlers.Get(source); h != nil {
		if expr := h.GetContentSelector(source); expr != nil {
			SelectContent(contentBytes, expr)
		}
		HarvestLinks(contentBytes)
	}
	return fmt.Errorf("No ResourceHandler found for URI %s", source)
}

// SelectContent ...
func SelectContent(contentBytes []byte, selectorExpression string) error {
	// TODO: select content sections from contentBytes if source has a content selector and then filter the rest of it.
	// TODO: define selector expression language. Do CSS/SaaS selectors or alike apply/ can be adapted?
	// Example: "h1-first-of-type" -> the first level one heading (#) in the document
	return nil
}

// HarvestLinks ...
func HarvestLinks(contentBytes []byte) ([]string, error) {
	// TODO: harvest links from this contentBytes
	// and resolve them to downloadable addresses and serialization targets
	return nil, nil
}
