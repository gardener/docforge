package reactor

import (
	"context"
	"fmt"
	"strings"

	"github.com/gardener/docode/pkg/api"
	"github.com/gardener/docode/pkg/jobs"
	"github.com/gardener/docode/pkg/processors"
	"github.com/gardener/docode/pkg/resourcehandlers"
	"github.com/gardener/docode/pkg/writers"
)

// Reader reads the bytes data from a given source URI
type Reader interface {
	Read(ctx context.Context, source string) ([]byte, error)
}

// ResourceData holds information for source and target of inlined documentation resources
type ResourceData struct {
	Node   *api.Node
	Source string
}

// DocumentWorker implements jobs#Worker
type DocumentWorker struct {
	writers.Writer
	Reader
	processors.Processor
	RdCh chan *ResourceData
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

	blobs := make(map[string][]byte)
	links := make([]string, 0)
	for _, content := range t.Node.ContentSelectors {
		sourceBlob, err := w.Reader.Read(ctx, content.Source)
		if len(sourceBlob) == 0 {
			continue
		}
		if err != nil {
			return jobs.NewWorkerError(err, 0)
		}
		blobs[content.Source] = sourceBlob
		blobLinks, err := HarvestLinks(content.Source, sourceBlob)
		if err != nil {
			return jobs.NewWorkerError(err, 0)
		}
		links = append(links, blobLinks...)
	}

	if len(blobs) == 0 {
		return nil
	}

	// process document blob
	// if h := w.ResourceHandlers.Get(source); h != nil {
	// 	// filter document content if there is content selector
	// 	if expr := h.GetContentSelector(source); len(expr) > 0 {
	// 		var err error
	// 		sourceBlob, err = SelectContent(sourceBlob, expr)
	// 		if err != nil {
	// 			return jobs.NewWorkerError(err, 0)
	// 		}
	// 	}
	// }

	// TODO: would it be a good idea if we set the channel as argument to HarvestLinks and let
	// it send the links
	// if err != nil {
	// 	return jobs.NewWorkerError(err, 0)
	// }

	//record document links for postprocessing
	for _, l := range links {
		// Resolve relative addresses beforehand, to ensure identity and avoid multiple processing of same link
		// TODO: implement me
		w.RdCh <- &ResourceData{
			Source: l,
		}
	}

	var sourceBlob []byte
	for _, blob := range blobs {
		sourceBlob = append(sourceBlob, blob...)
	}
	// set hardcode prop
	t.Node.Properties = map[string]interface{}{
		"name": t.Node.Name,
	}
	var err error
	if sourceBlob, err = w.Processor.Process(sourceBlob, t.Node); err != nil {
		return jobs.NewWorkerError(err, 0)
	}

	// pass docBlob to plugin processors
	var pathSegments []string
	for _, parent := range t.Node.Parents() {
		if parent.Name != "" {
			pathSegments = append(pathSegments, parent.Name)
		}
	}

	// Write the document content
	path := strings.Join(pathSegments, "/")
	if err := w.Writer.Write(t.Node.Name, path, sourceBlob); err != nil {
		return jobs.NewWorkerError(err, 0)
	}
	return nil
}

// LinkedResourceWorker implements jobs#Worker
type LinkedResourceWorker struct {
	writers.Writer
	Reader
}

// Work reads a single source and writes it to its target
func (r *LinkedResourceWorker) Work(ctx context.Context, task interface{}) *jobs.WorkerError {
	if t, ok := task.(*ResourceData); ok {
		fmt.Println("ResouceData: ", t.Source)
	}
	return nil
}
