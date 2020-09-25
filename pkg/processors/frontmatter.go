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
// TODO: failfast vs fault tolerant
func (f *FrontMatter) Process(documentBlob []byte, node *api.Node) ([]byte, error) {
	if node.Properties == nil {
		title := strings.Title(strings.TrimRight(node.Name, ".md"))
		node.Properties = map[string]interface{}{
			"Title": title,
		}
	}
	props, err := yaml.Marshal(node.Properties)
	if err != nil {
		return nil, err
	}
	var fm, content []byte
	if fm, content, err = markdown.StripFrontMatter(documentBlob); err != nil {
		return nil, err
	}
	if fm == nil {
		fm = []byte(props)
	} else {
		buf := bytes.NewBuffer(fm)
		buf.Write([]byte("\n"))
		buf.Write(props)
		if fm, err = ioutil.ReadAll(buf); err != nil {
			return nil, err
		}
	}
	if documentBlob, err = markdown.InsertFrontMatter(fm, content); err != nil {
		return nil, err
	}
	return documentBlob, nil
}
