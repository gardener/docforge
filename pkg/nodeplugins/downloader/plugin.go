package downloader

import (
	"sync"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/nodeplugins"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/workers/taskqueue"
	"github.com/gardener/docforge/pkg/writers"
)

type plugin struct {
	dScheduler Interface
}

// NewPlugin creates a new downloader plugin
func NewPlugin(workerCount int, failFast bool, wg *sync.WaitGroup, registry registry.Interface, writer writers.Writer) (nodeplugins.Interface, taskqueue.QueueController, error) {
	dScheduler, q, err := New(workerCount, failFast, wg, registry, writer)
	return &plugin{dScheduler}, q, err
}

func (plugin) Processor() string {
	return "downloader"
}

func (p *plugin) Process(node *manifest.Node) error {
	return p.dScheduler.Schedule(node.Source, node.NodePath())
}
