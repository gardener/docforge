package persona

import (
	"bytes"
	// for loading persona-filtering.json.tpl
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

// Plugin is the node plugin object
type Plugin struct {
	Root   *manifest.Node
	Writer writers.Writer
}

// Processor returns the persona processor
func (Plugin) Processor() string {
	return "persona"
}

// Process processes the node that will be constructed as the js file
func (p *Plugin) Process(node *manifest.Node) error {
	linkToPersonaMap := urlToPersonas(p.Root, map[string]string{})
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

func urlToPersonas(node *manifest.Node, res map[string]string) map[string]string {
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
		urlToPersonas(child, res)
	}
	return res
}
