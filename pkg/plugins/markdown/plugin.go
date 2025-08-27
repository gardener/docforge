package markdown

import (
	"context"
	"strings"

	"github.com/gardener/docforge/cmd/hugo"
	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/plugins"
	"github.com/gardener/docforge/pkg/plugins/markdown/linkresolver"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/writers"
)

// Plugin handles both manifest transformations and node processing for markdown files
type Plugin struct {
	registry           registry.Interface
	hugo               hugo.Hugo
	writer             writers.Writer
	skipLinkValidation bool
	documentWorker     *Worker // Created in FinalNodeStructure
}

// New creates a new markdown plugin
func New(registry registry.Interface, hugo hugo.Hugo, writer writers.Writer, skipLinkValidation bool) *Plugin {
	return &Plugin{
		registry:           registry,
		hugo:               hugo,
		writer:             writer,
		skipLinkValidation: skipLinkValidation,
	}
}

// Name returns the plugin name for identification
func (p *Plugin) Name() string {
	return "markdown"
}

// ManifestTransformations returns transformations to apply during manifest parsing
func (p *Plugin) ManifestTransformations() []manifest.NodeTransformation {
	return []manifest.NodeTransformation{
		p.setMarkdownProcessor,
		p.propagateFrontmatter,
		p.propagateSkipValidation,
	}
}

// FinalNodeStructure is called after manifest resolution with the final document structure
func (p *Plugin) FinalNodeStructure(documentNodes []*manifest.Node) error {
	// Create document worker with the final document structure
	lr := linkresolver.New(documentNodes, p.registry, p.hugo)
	p.documentWorker = NewDocumentWorker(lr, p.registry, p.hugo, p.writer, p.skipLinkValidation)
	return nil
}

// Processor returns the processor name for node processing
func (p *Plugin) Processor() string {
	return "markdown"
}

// Process processes a node using the old synchronous method
func (p *Plugin) Process(node *manifest.Node) error {
	// Legacy method - not used since we're using ProcessNew() for channels
	return nil
}

// ProcessNew processes a node using the new channel-based method
func (p *Plugin) ProcessNew(node *manifest.Node) []chan plugins.Status {
	out := make(chan plugins.Status)
	go func() {
		defer close(out)

		// Process document using Worker directly - now returns links
		links, err := p.documentWorker.ProcessNode(context.TODO(), node)
		if err != nil {
			out <- plugins.NewStatus(err)
			return
		}

		// Send status with collected external links
		out <- plugins.NewStatusWithLinks(nil, links) // Success
	}()
	return []chan plugins.Status{out}
}

// setMarkdownProcessor sets the processor for markdown files
func (p *Plugin) setMarkdownProcessor(node *manifest.Node, parent *manifest.Node, _ registry.Interface) (bool, error) {
	if node.Type == "file" && strings.HasSuffix(node.File, ".md") {
		node.Processor = "markdown"
	}
	return false, nil
}

// propagateFrontmatter propagates frontmatter from parent to child nodes
func (p *Plugin) propagateFrontmatter(node *manifest.Node, parent *manifest.Node, _ registry.Interface) (bool, error) {
	if parent != nil {
		newFM := map[string]interface{}{}
		for k, v := range parent.Frontmatter {
			if k != "aliases" {
				newFM[k] = v
			}
		}
		for k, v := range node.Frontmatter {
			newFM[k] = v
		}
		node.Frontmatter = newFM
	}
	return false, nil
}

// propagateSkipValidation propagates skip validation settings
func (p *Plugin) propagateSkipValidation(node *manifest.Node, parent *manifest.Node, _ registry.Interface) (bool, error) {
	if parent != nil && parent.Frontmatter != nil {
		if skipVal := parent.Frontmatter["skip_validation"]; skipVal != nil {
			if node.Frontmatter == nil {
				node.Frontmatter = map[string]interface{}{}
			}
			if node.Frontmatter["skip_validation"] == nil {
				node.Frontmatter["skip_validation"] = skipVal
			}
		}
	}
	return false, nil
}
