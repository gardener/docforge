package reactor

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gardener/docforge/pkg/processors"
	"github.com/google/uuid"
	"k8s.io/klog/v2"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/writers"
)

// Options encapsulates the parameters for creating
// new Reactor objects with NewReactor
type Options struct {
	MaxWorkersCount              int
	MinWorkersCount              int
	FailFast                     bool
	DestinationPath              string
	ResourcesPath                string
	ResourceDownloadWorkersCount int
	MarkdownFmt                  bool
	RewriteEmbedded              bool
	processors.Processor
	ResourceDownloadWriter writers.Writer
	GitInfoWriter          writers.Writer
	Writer                 writers.Writer
	ResourceHandlers       []resourcehandlers.ResourceHandler
	DryRunWriter           writers.DryRunWriter
	Resolve                bool
	GlobalLinksConfig      *api.Links
}

// NewReactor creates a Reactor from Options
func NewReactor(o *Options) *Reactor {
	var gitInfoController GitInfoController
	rhRegistry := resourcehandlers.NewRegistry(o.ResourceHandlers...)
	downloadController := NewDownloadController(nil, o.ResourceDownloadWriter, o.ResourceDownloadWorkersCount, o.FailFast, rhRegistry)
	if o.GitInfoWriter != nil {
		gitInfoController = NewGitInfoController(nil, o.GitInfoWriter, o.ResourceDownloadWorkersCount, o.FailFast, rhRegistry)
	}
	worker := &DocumentWorker{
		Writer:               o.Writer,
		Reader:               &GenericReader{rhRegistry},
		NodeContentProcessor: NewNodeContentProcessor(o.ResourcesPath, o.GlobalLinksConfig, downloadController, o.FailFast, o.MarkdownFmt, o.RewriteEmbedded, rhRegistry),
		Processor:            o.Processor,
		GitHubInfoController: gitInfoController,
	}
	docController := NewDocumentController(worker, o.MaxWorkersCount, o.FailFast)
	r := &Reactor{
		FailFast:           o.FailFast,
		ResourceHandlers:   rhRegistry,
		DocController:      docController,
		DownloadController: downloadController,
		GitInfoController:  gitInfoController,
		DryRunWriter:       o.DryRunWriter,
		Resolve:            o.Resolve,
	}
	return r
}

// Reactor orchestrates the documentation build workflow
type Reactor struct {
	FailFast           bool
	ResourceHandlers   resourcehandlers.Registry
	DocController      DocumentController
	DownloadController DownloadController
	GitInfoController  GitInfoController
	DryRunWriter       writers.DryRunWriter
	Resolve            bool
}

// Run starts build operation on documentation
func (r *Reactor) Run(ctx context.Context, manifest *api.Documentation, dryRun bool) error {
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		if r.Resolve {
			if err := printResolved(ctx, manifest, os.Stdout); err != nil {
				klog.Errorf("failed to print resolved manifest: %s", err.Error())
			}
		}
		cancel()
		if dryRun {
			r.DryRunWriter.Flush()
		}
	}()

	if err := r.ResolveManifest(ctx, manifest); err != nil {
		return err
	}

	klog.V(4).Info("Building documentation structure\n\n")
	if err := r.Build(ctx, manifest.Structure); err != nil {
		return err
	}

	return nil
}

// ResolveManifest resolves the manifests into buildable model
func (r *Reactor) ResolveManifest(ctx context.Context, manifest *api.Documentation) error {
	var (
		structure []*api.Node
		err       error
	)
	if manifest.NodeSelector != nil {
		if structure, err = r.resolveManifestNodeSelector(ctx, manifest.NodeSelector); err != nil {
			return err
		}
	}
	if structure == nil {
		structure = manifest.Structure
	} else {
		// TODO: this should be rather merge than append
		structure = append(manifest.Structure, structure...)
	}

	if structure == nil {
		return fmt.Errorf("document structure resolved to nil")
	}

	if err = r.resolveStructure(ctx, structure, manifest.Links); err != nil {
		return err
	}

	manifest.Structure = structure
	return nil
}

func printResolved(ctx context.Context, manifest *api.Documentation, writer io.Writer) error {
	// for _, node := range manifest.Structure {
	// 	if links := resolveNodeLinks(node, manifest.Links); len(links) > 0 {
	// 		for _, l := range links {
	// 			l := mergeLinks(node.ResolvedLinks, l)
	// 			node.ResolvedLinks = l
	// 		}
	// 	}
	// 	// remove resolved links for container nodes
	// 	if node.Nodes != nil {
	// 		node.ResolvedLinks = nil
	// 	}
	// }
	s, err := api.Serialize(manifest)
	if err != nil {
		return err
	}
	writer.Write([]byte(s))
	writer.Write([]byte("\n\n"))
	return nil
}

// ResolveStructure resolves the following in a structure model:
// - Node name variables
// - NodeSelectors
// The resulting model is the actual flight plan for replicating resources.
func (r *Reactor) resolveStructure(ctx context.Context, nodes []*api.Node, globalLinksConfig *api.Links) error {
	var handler resourcehandlers.ResourceHandler
	for _, node := range nodes {
		node.SetParentsDownwards()
		if len(node.Source) > 0 {
			if handler = r.ResourceHandlers.Get(node.Source); handler == nil {
				return fmt.Errorf("No suitable handler registered for URL %s", node.Source)
			}
			if len(node.Name) == 0 {
				node.Name = "$name"
			}
			name, ext := handler.ResourceName(node.Source)
			id := uuid.New().String()
			node.Name = strings.ReplaceAll(node.Name, "$name", name)
			node.Name = strings.ReplaceAll(node.Name, "$uuid", id)
			node.Name = strings.ReplaceAll(node.Name, "$ext", fmt.Sprintf(".%s", ext))
		}
		if node.NodeSelector != nil {
			if handler = r.ResourceHandlers.Get(node.NodeSelector.Path); handler == nil {
				return fmt.Errorf("No suitable handler registered for path %s", node.NodeSelector.Path)
			}
			if err := handler.ResolveNodeSelector(ctx, node, node.NodeSelector.ExcludePaths, node.NodeSelector.FrontMatter, node.NodeSelector.ExcludeFrontMatter, node.NodeSelector.Depth); err != nil {
				return err
			}
			// remove node selectors after resolution
			node.NodeSelector = nil
		}
		if len(node.Nodes) > 0 {
			if err := r.resolveStructure(ctx, node.Nodes, globalLinksConfig); err != nil {
				return err
			}
		}
	}
	return nil
}

// ResolveNodeSelector returns resolved nodeSelector nodes structure
func (r *Reactor) resolveManifestNodeSelector(ctx context.Context, nodeSelector *api.NodeSelector) ([]*api.Node, error) {
	var handler resourcehandlers.ResourceHandler
	if nodeSelector != nil {
		node := &api.Node{
			NodeSelector: nodeSelector,
		}
		if handler = r.ResourceHandlers.Get(nodeSelector.Path); handler == nil {
			return nil, fmt.Errorf("No suitable handler registered for path %s", nodeSelector.Path)
		}
		if err := handler.ResolveNodeSelector(ctx, node, nodeSelector.ExcludePaths, nodeSelector.ExcludeFrontMatter, nodeSelector.FrontMatter, nodeSelector.Depth); err != nil {
			return nil, err
		}
		return node.Nodes, nil
	}
	return nil, nil
}
