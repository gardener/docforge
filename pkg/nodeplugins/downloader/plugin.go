package downloader

import (
	"context"
	"sync"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/nodeplugins"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/writers"
)

type plugin struct {
	dWorkerd ResourceDownloadWorker
}

// NewPlugin creates a new downloader plugin
func NewPlugin(workerCount int, failFast bool, wg *sync.WaitGroup, registry registry.Interface, writer writers.Writer) (nodeplugins.Interface, error) {
	dWorkerd, err := NewDownloader(registry, writer)
	return &plugin{dWorkerd: *dWorkerd}, err
}

func (plugin) Processor() string {
	return "downloader"
}

func (p *plugin) Process(node *manifest.Node) error {
	return nil
}

func (p *plugin) ProcessNew(node *manifest.Node) []chan nodeplugins.Status {
	out := make(chan nodeplugins.Status)
	go func() {
		defer close(out)
		err := p.dWorkerd.Download(context.TODO(), node.Source, node.NodePath())
		out <- nodeplugins.NewStatus(err)
	}()
	return []chan nodeplugins.Status{out}
}
