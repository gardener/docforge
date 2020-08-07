package worker

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"

	"github.com/gardener/docode/pkg/api"
	"github.com/gardener/docode/pkg/backend"
	"github.com/gardener/docode/pkg/jobs"
)

// DocumentationTask is the task that DocWorker works on
type DocumentationTask struct {
	Node     *api.Node
	Handlers backend.ResourceHandlers
}

// DocWorker is a worker that process #DocumentationTask
type DocWorker struct {
	Writer
	Reader
	Processor
	RdCh chan *ResourceData
}

// ResourceData holds information for source and target of inlined documentation resources
type ResourceData struct {
	Source string
	Target string
}

// Reader reads data for a given source and returns it in raw format
type Reader interface {
	Read(ctx context.Context, source string) ([]byte, error)
}

func (g *GenericReader) Read(ctx context.Context, source string) ([]byte, error) {
	handler := g.Handlers.Get(source)
	if handler != nil {
		return handler.Read(ctx, source)
	}
	return nil, fmt.Errorf("failed to get handler")
}

// Writer writes blobs to a given path
type Writer interface {
	Write(name, path string, blobs []byte) error
}

// Processor is used to transform raw data
type Processor interface {
	Process(blobs []byte, node *api.Node) ([]byte, []*ResourceData, error)
}

// EmEmptyProcessor does nothing
type EmptyProcessor struct {
}

func (e *EmptyProcessor) Process(blobs []byte, node *api.Node) ([]byte, []*ResourceData, error) {
	return nil, nil, nil
}

// FSWriter is implementation of Writer interface for writing blobs to the file system
type FSWriter struct {
	Root string
}

// GenericReader is generic implementation for Reader interface
type GenericReader struct {
	Handlers backend.ResourceHandlers
}

func (f *FSWriter) Write(name, s string, docBlob []byte) error {
	p := path.Join(f.Root, s)
	if len(docBlob) < 0 {
		log.Println("skipping document with name", name)
	}

	if _, err := os.Stat(p); os.IsNotExist(err) {
		log.Println("mdkir: ", p)
		if err = os.MkdirAll(p, os.ModePerm); err != nil {
			return jobs.NewWorkerError(err, 0)
		}
	}

	filePath := path.Join(p, name)
	log.Println("writeFile: ", filePath)
	if err := ioutil.WriteFile(filePath, docBlob, 0644); err != nil {
		return jobs.NewWorkerError(err, 0)
	}

	return nil
}

// Work implements Worker#Work function
func (f *DocWorker) Work(ctx context.Context, task interface{}) *jobs.WorkerError {
	var t *DocumentationTask
	t, ok := task.(*DocumentationTask)
	if !ok {
		return jobs.NewWorkerError(fmt.Errorf("cast failed: %v", task), 0)
	}

	if t.Node.Source == nil {
		log.Println("node has nil source:", t.Node)
		return nil
	}

	var docBlob []byte
	for _, source := range t.Node.Source {
		sourceBlob, err := f.Reader.Read(ctx, source)
		if err != nil {
			log.Println(err)
			return jobs.NewWorkerError(err, 0)
		}

		// TODO: When using selector, crop the blob before processing
		processedBlob, resourcesData, err := f.Processor.Process(sourceBlob, t.Node)
		if err != nil {
			log.Println(err)

			return jobs.NewWorkerError(err, 0)
		}
		for _, r := range resourcesData {
			f.RdCh <- r
		}
		docBlob = append(docBlob, processedBlob...)
	}

	// Processing
	// TODO: format blobs to desired state

	var dirNames []string
	for _, parent := range t.Node.Parents() {
		if parent.Name != "" {
			dirNames = append(dirNames, parent.Name)
		}
	}

	path := strings.Join(dirNames, "/")
	if err := f.Writer.Write(t.Node.Name, path, docBlob); err != nil {
		return jobs.NewWorkerError(err, 0)
	}

	return nil
}

// ResourceWorker Reads
type ResourceWorker struct {
	Reader
}

// Work reads a single source and writes it to its target
func (r *ResourceWorker) Work(ctx context.Context, task interface{}) *jobs.WorkerError {
	if t, ok := task.(*ResourceData); ok {
		blob, err := r.Reader.Read(ctx, t.Source)
		if err != nil {
			return jobs.NewWorkerError(err, 0)
		}
		fmt.Println(blob)
	}

	return nil
}
