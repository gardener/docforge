package reactor

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/jobs"
	"github.com/gardener/docforge/pkg/processors"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	utilnode "github.com/gardener/docforge/pkg/util/node"
	"github.com/gardener/docforge/pkg/writers"
	"k8s.io/klog/v2"
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
	NodeContentProcessor NodeContentProcessor
	GitHubInfoController GitInfoController
}

// DocumentWorkTask implements jobs#Task
type DocumentWorkTask struct {
	Node *api.Node
	// localityDomain localityDomain
}

// GenericReader is generic implementation for Reader interface
type GenericReader struct {
	ResourceHandlers resourcehandlers.Registry
}

// Read reads from the resource at the source URL delegating the
// the actual operation to a suitable resource handler
func (g *GenericReader) Read(ctx context.Context, source string) ([]byte, error) {
	if handler := g.ResourceHandlers.Get(source); handler != nil {
		return handler.Read(ctx, source)
	}
	return nil, fmt.Errorf("failed to get handler to read from %s", source)
}

// Work implements Worker#Work function
func (w *DocumentWorker) Work(ctx context.Context, task interface{}, wq jobs.WorkQueue) *jobs.WorkerError {
	if task, ok := task.(*DocumentWorkTask); ok {

		var (
			b                    bytes.Buffer
			sourceBlob, document []byte
			err                  error
		)

		if len(task.Node.Nodes) == 0 {
			if len(task.Node.ContentSelectors) > 0 {
				for _, content := range task.Node.ContentSelectors {
					if sourceBlob, err = w.Reader.Read(ctx, content.Source); err != nil {
						return jobs.NewWorkerError(err, 0)
					}
					if len(sourceBlob) == 0 {
						continue
					}
					if sourceBlob, err = w.NodeContentProcessor.ReconcileLinks(ctx, task.Node, content.Source, sourceBlob); err != nil {
						return jobs.NewWorkerError(err, 0)
					}
					b.Write(sourceBlob)
				}
			}
			// TODO: implement read by template
			if len(task.Node.Source) > 0 {
				if sourceBlob, err = w.Reader.Read(ctx, task.Node.Source); err != nil {
					return jobs.NewWorkerError(err, 0)
				}
				if len(sourceBlob) == 0 {
					klog.Warningf("No content read from node %s source %s:", task.Node.Name, task.Node.Source)
					return nil
				}
				if sourceBlob, err = w.NodeContentProcessor.ReconcileLinks(ctx, task.Node, task.Node.Source, sourceBlob); err != nil {
					return jobs.NewWorkerError(err, 0)
				}
				b.Write(sourceBlob)
			}
			if b.Len() == 0 {
				klog.Warningf("Document node processing halted: No content assigned to document node %s", task.Node.Name)
				return nil
			}

			if document, err = ioutil.ReadAll(&b); err != nil {
				return jobs.NewWorkerError(err, 0)
			}

			if w.Processor != nil {
				if document, err = w.Processor.Process(document, task.Node); err != nil {
					return jobs.NewWorkerError(err, 0)
				}
			}
		}

		path := utilnode.Path(task.Node, "/")
		if err = w.Writer.Write(task.Node.Name, path, document, task.Node); err != nil {
			return jobs.NewWorkerError(err, 0)
		}

		if w.GitHubInfoController != nil && len(document) > 0 {
			w.GitHubInfoController.WriteGitInfo(ctx, filepath.Join(path, task.Node.Name), task.Node)
		}
	}
	return nil
}
