package persona

import (
	"bytes"
	"path"
	"slices"

	// for loading persona-filtering.json.tpl
	_ "embed"
	"log"
	"strings"
	"text/template"

	"github.com/gardener/docforge/pkg/internal/link"
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
	if node.Frontmatter["persona"] != nil && strings.HasPrefix(node.HugoPrettyPath(), "content") {
		// bubble persona up
		persona, ok := node.Frontmatter["persona"].(string)
		must.BeTrue(ok)

		if node.Type == "manifest" || (node.Processor != "markdown" && node.Type == "file") {
			return res
		}

		url := node.HugoPrettyPath()
		trimmedPath := strings.TrimPrefix(url, "content")
		components := strings.Split(trimmedPath, "/")
		var subpaths []string
		for i := 1; i < len(components); i++ {
			subpath := must.Succeed(link.Build("/", path.Join(components[:i+1]...), "/"))
			if !slices.Contains(subpaths, subpath) {
				subpaths = append(subpaths, subpath)
				currentPersonas := []string{}
				if res[subpath] != "" {
					currentPersonas = strings.Split(res[subpath], ",")
				}
				if !slices.Contains(currentPersonas, persona) {
					currentPersonas = append(currentPersonas, persona)
				}
				res[subpath] = strings.Join(currentPersonas, ",")
			}
		}
	}
	for _, child := range node.Structure {
		urlToPersonas(child, res)
	}
	return res
}
