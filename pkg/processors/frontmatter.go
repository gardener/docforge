package processors

import (
	"bytes"
	"io/ioutil"
	"strings"

	"github.com/gardener/docforge/pkg/markdown"

	"github.com/gardener/docforge/pkg/api"
	"gopkg.in/yaml.v3"
)

// FrontMatter is a processor implementation responsible to inject front-matter
// properties defined on nodes
type FrontMatter struct{}

// Process implements Processor#Process
func (f *FrontMatter) Process(documentBlob []byte, node *api.Node) ([]byte, error) {
	var (
		nodeFmBytes, fmBytes, content []byte
		props, fm, docFm              map[string]interface{}
		ok                            bool
	)
	// Frontmatter from node
	if props = node.Properties; props == nil {
		props = map[string]interface{}{}
	}
	if _fm, found := props["frontmatter"]; found {
		if fm, ok = _fm.(map[string]interface{}); !ok {
			panic("node frontmatter type cast failed")
		}
	}

	if fm == nil {
		// Minimal standard front matter, injected by default
		// if no other has been defined on a node
		// TODO: make this configurable option, incl. the default frontmatter we inject
		fm = map[string]interface{}{
			"title": strings.Title(node.Name),
		}
	}

	nodeFmBytes, err := yaml.Marshal(fm)
	if err != nil {
		return nil, err
	}

	// document front matter
	if fmBytes, content, err = markdown.StripFrontMatter(documentBlob); err != nil {
		return nil, err
	}
	docFm = map[string]interface{}{}
	if err := yaml.Unmarshal(fmBytes, docFm); err != nil {
		return nil, err
	}

	// TODO: merge node + doc frontmatter per configurable strategy:
	// - merge where node frontmatter entries win over document frontmatter
	// - merge where document frontmatter entries win over node frontmatter
	// - merge where document frontmatter are merged wiht node frontmatter ignoring duplicates (currently impl.)
	buf := bytes.NewBuffer([]byte{})
	if fmBytes != nil {
		buf.Write(fmBytes)
	}
	if nodeFmBytes != nil {
		buf.Write(nodeFmBytes)
	}
	if fmBytes, err = ioutil.ReadAll(buf); err != nil {
		return nil, err
	}
	if documentBlob, err = markdown.InsertFrontMatter(fmBytes, content); err != nil {
		return nil, err
	}
	return documentBlob, nil
}
