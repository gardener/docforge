package processors

import (
	"fmt"

	"github.com/gardener/docode/pkg/api"
	"gopkg.in/yaml.v3"
)

// FrontMatter is a processor implementation responsible to inject front-matter
// properties defined on nodes
type FrontMatter struct{}

// Process implements Processor#Process
func (f *FrontMatter) Process(documentBlob []byte, node *api.Node) ([]byte, error) {
	if node.Properties != nil {
		b, err := yaml.Marshal(node.Properties)
		if err != nil {
			return nil, err
		}
		annotatedDocument := fmt.Sprintf("---\n%s\n---\n%s", b, documentBlob)
		return []byte(annotatedDocument), nil
	}
	return documentBlob, nil
}
