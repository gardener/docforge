package persona

import (
	"bytes"
	_ "embed"
	"html/template"
	"log"
	"strings"

	"github.com/gardener/docforge/pkg/internal/must"
	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/writers"
)

//go:embed persona-filtering.json.tpl
var jsTemplate string

type Plugin struct {
	Root   *manifest.Node
	Writer writers.Writer
}

func (Plugin) Processor() string {
	return "persona"
}

func (p *Plugin) Process(node *manifest.Node) error {
	linkToPersonaMap := UrlToPersonas(p.Root, map[string]string{})
	t, err := template.New("webpage").Parse(jsTemplate)
	if err != nil {
		log.Fatalf("Error parsing template: %v", err)
	}

	var renderedTemplate bytes.Buffer
	err = t.Execute(&renderedTemplate, linkToPersonaMap)
	if err != nil {
		log.Fatalf("Error executing template: %v", err)
	}

	return p.Writer.Write(node.Name(), node.Path, renderedTemplate.Bytes(), node, []string{})
}

func UrlToPersonas(node *manifest.Node, res map[string]string) map[string]string {
	if node.Frontmatter["persona"] != nil {
		// bubble persona up
		persona, ok := node.Frontmatter["persona"].(string)
		must.BeTrue(ok)
		for current := node; current != nil && strings.HasPrefix(current.HugoPrettyPath(), "content"); current = current.Parent() {
			if current.Type == "manifest" || (current.Processor != "markdown" && current.Type == "file") {
				continue
			}
			url := current.HugoPrettyPath()
			currentPersonas := res[strings.TrimPrefix(url, "content")]
			if !strings.Contains(currentPersonas, persona) {
				personas := []string{currentPersonas, persona}
				if currentPersonas == "" {
					personas = []string{persona}
				}
				res[strings.TrimPrefix(url, "content")] = strings.Join(personas, ",")
			}

		}
	}
	for _, child := range node.Structure {
		UrlToPersonas(child, res)
	}
	return res
}
