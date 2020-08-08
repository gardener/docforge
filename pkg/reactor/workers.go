package reactor

import (
	"context"
	"fmt"
	"strings"

	"github.com/gardener/docode/pkg/api"
	"github.com/gardener/docode/pkg/backend"
	"github.com/gardener/docode/pkg/jobs"
	"github.com/gardener/docode/pkg/processors"
	"github.com/gardener/docode/pkg/writers"
)

// Reader reads the bytes data from a given source URI
type Reader interface {
	Read(ctx context.Context, source string) ([]byte, error)
}

// ResourceData holds information for source and target of inlined documentation resources
type ResourceData struct {
	Source string
	Target string
}

// DocumentWorker implements jobs#Worker
type DocumentWorker struct {
	writers.Writer
	Reader
	processors.Processor
	RdCh             chan *ResourceData
	ResourceHandlers backend.ResourceHandlers
}

// DocumentWorkTask implements jobs#Task
type DocumentWorkTask struct {
	Node     *api.Node
	Handlers backend.ResourceHandlers
}

// GenericReader is generic implementation for Reader interface
type GenericReader struct {
	Handlers backend.ResourceHandlers
}

func (g *GenericReader) Read(ctx context.Context, source string) ([]byte, error) {
	if handler := g.Handlers.Get(source); handler != nil {
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

	if len(t.Node.Source) > 0 {
		var docBlob []byte
		// for _, source := range t.Node.Source {
		source := t.Node.Source
		sourceBlob, err := w.Reader.Read(ctx, source)
		if err != nil {
			return jobs.NewWorkerError(err, 0)
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
		// harvest document links
		// TODO: rewrite links as appropriate on this step too.
		links, err := HarvestLinks(sourceBlob)
		if err != nil {
			return jobs.NewWorkerError(err, 0)
		}
		//record document links for postprocessing
		for _, l := range links {
			// Resolve relative addresses beforehand, to ensure identity and avoid multiple processing of same link
			// TODO: implement me
			w.RdCh <- &ResourceData{
				l,
				"",
			}
		}

		// merge document content parts into a single document
		docBlob = append(docBlob, sourceBlob...)
		// }

		if len(docBlob) > 0 {
			// pass docBlob to plugin processors

			var pathSegments []string
			for _, parent := range t.Node.Parents() {
				if parent.Name != "" {
					pathSegments = append(pathSegments, parent.Name)
				}
			}

			// Write the document content
			path := strings.Join(pathSegments, "/")
			if err := w.Writer.Write(t.Node.Name, path, docBlob); err != nil {
				return jobs.NewWorkerError(err, 0)
			}
		}

	}

	return nil
}

// LinkedResourceWorker implements jobs#Worker
type LinkedResourceWorker struct {
	writers.Writer
	Reader
	ResourceHandlers backend.ResourceHandlers
}

// Work reads a single source and writes it to its target
func (r *LinkedResourceWorker) Work(ctx context.Context, task interface{}) *jobs.WorkerError {
	// if t, ok := task.(*ResourceData); ok {
	// 	blob, err := r.Reader.Read(ctx, t.Source)
	// 	if err != nil {
	// 		return jobs.NewWorkerError(err, 0)
	// 	}
	// 	if err = r.Writer.Write("", "", blob); err != nil {
	// 		return jobs.NewWorkerError(err, 0)
	// 	}
	// }

	return nil
}
