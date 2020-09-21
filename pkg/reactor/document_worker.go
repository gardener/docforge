package reactor

import (
	"context"
	"fmt"

	"github.com/gardener/docode/pkg/api"
	"github.com/gardener/docode/pkg/jobs"
	"github.com/gardener/docode/pkg/processors"
	"github.com/gardener/docode/pkg/resourcehandlers"
	utilnode "github.com/gardener/docode/pkg/util/node"
	"github.com/gardener/docode/pkg/writers"
)

// Reader reads the bytes data from a given source URI
type Reader interface {
	Read(ctx context.Context, source string) ([]byte, error)
}

// DocumentWorker implements jobs#Worker
type DocumentWorker struct {
	writers.Writer
	Reader
	processors.Processor
	NodeContentProcessor *NodeContentProcessor
	localityDomain       localityDomain
}

// DocumentWorkTask implements jobs#Task
type DocumentWorkTask struct {
	Node *api.Node
	// localityDomain localityDomain
}

// GenericReader is generic implementation for Reader interface
type GenericReader struct {
}

// Read TODO:
func (g *GenericReader) Read(ctx context.Context, source string) ([]byte, error) {
	if handler := resourcehandlers.Get(source); handler != nil {
		return handler.Read(ctx, source)
	}
	return nil, fmt.Errorf("failed to get handler")
}

// Work implements Worker#Work function
func (w *DocumentWorker) Work(ctx context.Context, task interface{}) *jobs.WorkerError {
	var (
		t  *DocumentWorkTask
		ok bool
	)
	if t, ok = task.(*DocumentWorkTask); !ok {
		return jobs.NewWorkerError(fmt.Errorf("cast failed: %v", task), 0)
	}

	if len(t.Node.ContentSelectors) < 1 {
		return nil
	}

	// Write the document content
	blobs := make(map[string][]byte)
	for _, content := range t.Node.ContentSelectors {
		sourceBlob, err := w.Reader.Read(ctx, content.Source)
		if len(sourceBlob) == 0 {
			continue
		}
		if err != nil {
			return jobs.NewWorkerError(err, 0)
		}

		newBlob, err := w.NodeContentProcessor.ReconcileLinks(ctx, t.Node, content.Source, sourceBlob)
		if err != nil {
			return jobs.NewWorkerError(err, 0)
		}
		blobs[content.Source] = newBlob
	}

	if len(blobs) == 0 {
		return nil
	}

	var sourceBlob []byte
	for _, blob := range blobs {
		sourceBlob = append(sourceBlob, blob...)
	}

	var err error
	if w.Processor != nil {
		if sourceBlob, err = w.Processor.Process(sourceBlob, t.Node); err != nil {
			return jobs.NewWorkerError(err, 0)
		}
	}

	path := utilnode.Path(t.Node, "/")
	if err := w.Writer.Write(t.Node.Name, path, sourceBlob); err != nil {
		return jobs.NewWorkerError(err, 0)
	}
	return nil
}
