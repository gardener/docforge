package processors

import (
	"fmt"
	"strings"

	"github.com/gardener/docforge/pkg/api"
	"gopkg.in/yaml.v3"
)

// FrontMatter is a processor implementation responsible to inject front-matter
// properties defined on nodes
type FrontMatter struct{}

// Process implements Processor#Process
func (f *FrontMatter) Process(documentBlob []byte, node *api.Node) ([]byte, error) {
	if node.Properties == nil {
		title := strings.Title(strings.TrimRight(node.Name, ".md"))
		node.Properties = map[string]interface{}{
			"Title": title,
		}
	}
	b, err := yaml.Marshal(node.Properties)
	if err != nil {
		return nil, err
	}
	annotatedDocument := fmt.Sprintf("---%s---\n%s", b, documentBlob)
	return []byte(annotatedDocument), nil
}
