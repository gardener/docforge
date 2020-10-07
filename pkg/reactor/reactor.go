package reactor

import (
	"context"
	"fmt"

	"github.com/gardener/docforge/pkg/processors"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/writers"
)

// Options encapsulates the parameters for creating
// new Reactor objects wiht NewReactor
type Options struct {
	MaxWorkersCount              int
	MinWorkersCount              int
	FailFast                     bool
	DestinationPath              string
	ResourcesPath                string
	ResourceDownloadWorkersCount int
	MarkdownFmt                  bool
	processors.Processor
	ResourceDownloadWriter writers.Writer
	Writer                 writers.Writer
	ResourceHandlers       []resourcehandlers.ResourceHandler
}

// NewReactor creates a Reactor from Options
func NewReactor(o *Options) *Reactor {
	rhRegistry := resourcehandlers.NewRegistry(o.ResourceHandlers...)
	downloadController := NewDownloadController(nil, o.ResourceDownloadWriter, o.ResourceDownloadWorkersCount, o.FailFast, rhRegistry)
	worker := &DocumentWorker{
		Writer:               o.Writer,
		Reader:               &GenericReader{rhRegistry},
		NodeContentProcessor: NewNodeContentProcessor(o.ResourcesPath, nil, downloadController, o.FailFast, o.MarkdownFmt, rhRegistry),
		Processor:            o.Processor,
	}
	docController := NewDocumentController(worker, o.MaxWorkersCount, o.FailFast)
	r := &Reactor{
		FailFast:           o.FailFast,
		ResourceHandlers:   rhRegistry,
		DocController:      docController,
		DownloadController: downloadController,
	}
	return r
}

// Reactor orchestrates the documentation build workflow
type Reactor struct {
	FailFast           bool
	ResourceHandlers   resourcehandlers.Registry
	localityDomain     localityDomain
	DocController      DocumentController
	DownloadController DownloadController
}

// Run starts build operation on docStruct
func (r *Reactor) Run(ctx context.Context, docStruct *api.Documentation, dryRun bool) error {
	var err error
	if err := r.Resolve(ctx, docStruct.Root); err != nil {
		return err
	}

	ld := fromAPI(docStruct.LocalityDomain)
	if ld == nil || len(ld) == 0 {
		if ld, err = setLocalityDomainForNode(docStruct.Root, r.ResourceHandlers); err != nil {
			return err
		}
		r.localityDomain = ld
	}

	if dryRun {
		s, err := api.Serialize(docStruct)
		if err != nil {
			return err
		}
		fmt.Println(s)
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	fmt.Printf("Building documentation structure\n\n")
	if err = r.Build(ctx, docStruct.Root, ld); err != nil {
		return err
	}

	return nil
}

// Resolve builds the subnodes hierarchy of a node based on the natural nodes
// hierarchy and on rules such as those in NodeSelector.
// The node hierarchy is resolved by an appropriate handler selected based
// on the NodeSelector path URI
// The resulting model is the actual flight plan for replicating resources.
func (r *Reactor) Resolve(ctx context.Context, node *api.Node) error {
	node.SetParentsDownwards()
	if node.NodeSelector != nil {
		var handler resourcehandlers.ResourceHandler
		if handler = r.ResourceHandlers.Get(node.NodeSelector.Path); handler == nil {
			return fmt.Errorf("No suitable handler registered for path %s", node.NodeSelector.Path)
		}
		if err := handler.ResolveNodeSelector(ctx, node); err != nil {
			return err
		}
	}
	if len(node.Nodes) > 0 {
		for _, n := range node.Nodes {
			if err := r.Resolve(ctx, n); err != nil {
				return err
			}
		}
	}
	return nil
}
