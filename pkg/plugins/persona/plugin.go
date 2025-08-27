// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
	"github.com/gardener/docforge/pkg/plugins"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/writers"
)

//go:embed persona-filtering.json.tpl
var jsTemplate string

// Plugin handles both manifest transformations and node processing for persona filtering
type Plugin struct {
	writer writers.Writer
	root   *manifest.Node // Document root for processing
}

// New creates a new persona plugin
func New(writer writers.Writer) *Plugin {
	return &Plugin{
		writer: writer,
	}
}

// Name returns the plugin name for identification
func (p *Plugin) Name() string {
	return "persona"
}

// ManifestTransformations returns transformations to apply during manifest parsing
func (p *Plugin) ManifestTransformations() []manifest.NodeTransformation {
	return []manifest.NodeTransformation{p.resolvePersonaFolders}
}

// FinalNodeStructure is called after manifest resolution with the final document structure
func (p *Plugin) FinalNodeStructure(documentNodes []*manifest.Node) error {
	// Store the root for node processing
	if len(documentNodes) > 0 {
		p.root = documentNodes[0]
	}
	return nil
}

// Processor returns the processor name for node processing
func (p *Plugin) Processor() string {
	return "persona"
}

// Process processes a node to generate the JavaScript filtering file
func (p *Plugin) Process(node *manifest.Node) error {
	linkToPersonaMap := p.urlToPersonas(p.root, map[string]string{})
	t, err := template.New("webpage").Parse(jsTemplate)
	if err != nil {
		log.Fatalf("Error parsing template: %v", err)
	}

	var renderedTemplate bytes.Buffer
	err = t.Execute(&renderedTemplate, linkToPersonaMap)
	if err != nil {
		log.Fatalf("Error executing template: %v", err)
	}
	return p.writer.Write(node.Name(), node.Path, renderedTemplate.Bytes(), node, []string{})
}

// ProcessNew processes a node using the new channel-based method
func (p *Plugin) ProcessNew(node *manifest.Node) []chan plugins.Status {
	return nil // Persona plugin only uses synchronous processing
}

// resolvePersonaFolders resolves persona directory structures
func (p *Plugin) resolvePersonaFolders(node *manifest.Node, parent *manifest.Node, _ registry.Interface) (bool, error) {
	if node.Type == "dir" && (node.Dir == "development" || node.Dir == "operations" || node.Dir == "usage") {
		for _, child := range node.Structure {
			p.addPersonaAliasesForNode(child, node.Dir)
		}
		parent.Structure = append(parent.Structure, node.Structure...)
		manifest.RemoveNodeFromParent(node, parent)
	}
	return true, nil
}

// addPersonaAliasesForNode adds persona metadata to nodes
func (p *Plugin) addPersonaAliasesForNode(node *manifest.Node, personaDir string) {
	var dirToPersona = map[string]string{"usage": "Users", "operations": "Operators", "development": "Developers"}
	if node.Type == "file" {
		if node.Frontmatter == nil {
			node.Frontmatter = map[string]interface{}{}
		}
		node.Frontmatter["persona"] = dirToPersona[personaDir]
	}
	for _, child := range node.Structure {
		p.addPersonaAliasesForNode(child, personaDir)
	}
}

// urlToPersonas builds a map from URL to persona list for the JavaScript template
func (p *Plugin) urlToPersonas(node *manifest.Node, res map[string]string) map[string]string {
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
		p.urlToPersonas(child, res)
	}
	return res
}
