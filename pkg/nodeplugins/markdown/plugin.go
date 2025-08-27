package markdown

import (
	"context"
	"sync"

	"github.com/gardener/docforge/cmd/hugo"
	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/nodeplugins"
	"github.com/gardener/docforge/pkg/nodeplugins/markdown/document"
	"github.com/gardener/docforge/pkg/nodeplugins/markdown/linkresolver"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/workers/taskqueue"
	"github.com/gardener/docforge/pkg/writers"
)

type plugin struct {
	documentWorker *document.Worker // Direct access for channels
}

// NewPlugin creates a new markdown plugin
func NewPlugin(workerCount int, failFast bool, wg *sync.WaitGroup, structure []*manifest.Node, rhs registry.Interface, hugo hugo.Hugo, writer writers.Writer, skipLinkValidation bool, validationWorkersCount int, hostsToReport []string, resourceDownloadWorkersCount int) (nodeplugins.Interface, []taskqueue.QueueController, error) {
	queues := []taskqueue.QueueController{}

	// No longer creating validator - using deferred validation instead
	// validator, validatorTasks, err := linkvalidator.New(validationWorkersCount, failFast, wg, rhs, hostsToReport)
	// if err != nil {
	//	return nil, nil, err
	// }

	// Create document worker directly for channel processing
	lr := linkresolver.New(structure, rhs, hugo)
	documentWorker := document.NewDocumentWorker(lr, rhs, hugo, writer, skipLinkValidation)

	return &plugin{
		documentWorker: documentWorker,
	}, queues, nil
}

func (plugin) Processor() string {
	return "markdown"
}

func (p *plugin) Process(node *manifest.Node) error {
	// Legacy method - not used since we're using ProcessNew() for channels
	// This is kept for interface compatibility but does nothing
	return nil
}

func (p *plugin) ProcessNew(node *manifest.Node) []chan nodeplugins.Status {
	out := make(chan nodeplugins.Status)
	go func() {
		defer close(out)

		// Process document using Worker directly - now returns links
		links, err := p.documentWorker.ProcessNode(context.TODO(), node)
		if err != nil {
			out <- nodeplugins.NewStatus(err)
			return
		}

		// Send status with collected external links
		out <- nodeplugins.NewStatusWithLinks(nil, links) // Success
	}()
	return []chan nodeplugins.Status{out}
}
