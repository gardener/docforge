package reactor

import (
	"context"
	"fmt"
	"sync"

	"github.com/gardener/docode/pkg/api"
	"github.com/gardener/docode/pkg/jobs"
	"github.com/gardener/docode/pkg/processors"
	"github.com/gardener/docode/pkg/resourcehandlers"
	utilnode "github.com/gardener/docode/pkg/util/node"
	"github.com/gardener/docode/pkg/writers"
	"github.com/prometheus/common/log"
)

// Reader reads the bytes data from a given source URI
type Reader interface {
	Read(ctx context.Context, source string) ([]byte, error)
}

// ResourceData holds information for source and target of inlined documentation resources
type ResourceData struct {
	Source         string
	NodeTargetPath string
	OriginalPath   string
	FileName       string
}

// DocumentWorker implements jobs#Worker
type DocumentWorker struct {
	writers.Writer
	Reader
	processors.Processor
	contentProcessor *ContentProcessor
	RdCh             chan *ResourceData
}

// DocumentWorkTask implements jobs#Task
type DocumentWorkTask struct {
	Node *api.Node
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

	if len(t.Node.ContentSelectors) <= 0 {
		return nil
	}

	// pass docBlob to plugin processors
	path := utilnode.NodePath(t.Node, "/")

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

		newBlob, err := HarvestLinks(t.Node, content.Source, path, sourceBlob, w.RdCh, w.contentProcessor)
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

	// TODO: delete
	t.Node.Properties = map[string]interface{}{
		"name": t.Node.Name,
	}

	var err error
	if sourceBlob, err = w.Processor.Process(sourceBlob, t.Node); err != nil {
		return jobs.NewWorkerError(err, 0)
	}

	if err := w.Writer.Write(t.Node.Name, path, sourceBlob); err != nil {
		return jobs.NewWorkerError(err, 0)
	}
	return nil
}

// LinkedResourceWorker implements jobs#Worker
type LinkedResourceWorker struct {
	writers.Writer
	Reader

	downloadedResources map[string]struct{}
	rwlock              sync.RWMutex
}

// Work reads a single source and writes it to its target
func (r *LinkedResourceWorker) Work(ctx context.Context, rd *ResourceData) *jobs.WorkerError {
	if r.downloaded(rd) {
		return nil
	}

	blob, err := r.Reader.Read(ctx, rd.Source)
	if err != nil {
		log.Error(err)
		return jobs.NewWorkerError(err, 1)
	}

	// p := strings.Split(rd.OriginalPath, "/")
	// fileName := p[len(p)-1]
	// filepath := strings.Join(p[:len(p)-1], "/")
	// filepath = rd.NodeTargetPath + "/" + filepath
	if err := r.Writer.Write(rd.NodeTargetPath, "", blob); err != nil {
		log.Error(err)
		return jobs.NewWorkerError(err, 1)
	}

	r.rwlock.Lock()
	defer r.rwlock.Unlock()
	if r.downloadedResources == nil {
		r.downloadedResources = make(map[string]struct{})
	}
	r.downloadedResources[rd.Source] = struct{}{}
	return nil
}

func (r *LinkedResourceWorker) downloaded(rd *ResourceData) bool {
	r.rwlock.Lock()
	defer r.rwlock.Unlock()
	_, ok := r.downloadedResources[rd.Source]
	return ok
}
