package reactor

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/gardener/docode/pkg/api"
	"github.com/gardener/docode/pkg/backend"
	"github.com/gardener/docode/pkg/jobs"
	"gopkg.in/yaml.v3"
)

// Worker implements jobs.Worker
type Worker struct {
	ResourceHandlers backend.ResourceHandlers
}

// Task implements jobs.Task
type Task struct {
	node      *api.Node
	localPath string
}

// Work implements Worker#Work function
func (w *Worker) Work(ctx context.Context, task interface{}) *jobs.WorkerError {
	if task, ok := task.(*Task); ok {
		if task.node.Source != nil {
			blobs := make([]byte, 0)
			for _, s := range task.node.Source {
				if handler := w.ResourceHandlers.Get(s); handler != nil {
					blob, err := handler.Read(ctx, task.node)
					if err != nil {
						return jobs.NewWorkerError(err, 0)
					}
					blobs = append(blobs, blob...)
				}
			}
			if len(blobs) > 0 {
				// TODO : should apply only to Markdown documents
				if task.node.Properties != nil {
					b, err := yaml.Marshal(task.node.Properties)
					if err != nil {
						return jobs.NewWorkerError(err, 0)
					}
					annotatedDocument := fmt.Sprintf("---\n%s\n---\n%s", b, blobs)
					blobs = []byte(annotatedDocument)
				}
				parents := filepath.Dir(task.localPath)
				if _, err := os.Stat(parents); os.IsNotExist(err) {
					if err = os.MkdirAll(parents, os.ModePerm); err != nil {
						return jobs.NewWorkerError(err, 0)
					}
				}
				if err := ioutil.WriteFile(task.localPath, blobs, 0644); err != nil {
					return jobs.NewWorkerError(err, 0)
				}
			}
		}
	}
	return nil
}
