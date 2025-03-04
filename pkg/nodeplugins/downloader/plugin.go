package downloader

import (
	"sync"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/nodeplugins"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/workers/taskqueue"
	"github.com/gardener/docforge/pkg/writers"
)

type Plugin struct {
	dScheduler Interface
}

func NewPlugin(workerCount int, failFast bool, wg *sync.WaitGroup, registry registry.Interface, writer writers.Writer) (nodeplugins.Interface, taskqueue.QueueController, error) {
	dScheduler, q, err := New(workerCount, failFast, wg, registry, writer)
	return &Plugin{dScheduler}, q, err
}

func (Plugin) Processor() string {
	return "downloader"
}

func (p *Plugin) Process(node *manifest.Node) error {
	return p.dScheduler.Schedule(node.Source, node.NodePath())
}
