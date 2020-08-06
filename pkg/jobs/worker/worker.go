package worker

import (
	"context"
	"io/ioutil"
	"os"
	"strings"

	"github.com/gardener/docode/pkg/api"
	"github.com/gardener/docode/pkg/backend"
	"github.com/gardener/docode/pkg/jobs"
)

// New creates a Worker of type FileSystem
func New(workerType string, params Params) jobs.Worker {
	switch workerType {
	case FileSystemWorker:
		if rootDir, ok := params[FSWorkerRootParam]; ok {
			targetRootDir, ok := rootDir.(string)
			if !ok {
				return nil
			}

			return &FSWorker{
				string(targetRootDir),
			}

		}
	default:
		return nil
	}
	return nil
}

// Params is the type used to provide parameters to Workers
type Params map[string]interface{}

// Task implements jobs.Task
type Task struct {
	Node     *api.Node
	Handlers backend.ResourceHandlers
}

// FSWorker serializes given task to file system hierarchy
type FSWorker struct {
	root string
}

// FSWorkerRootParam is parameter for defining target root directory for FileSystem Workers
var FSWorkerRootParam string = "targetRootDir"
var FileSystemWorker string = "fileSystem"

// Work implements Worker#Work function
func (f *FSWorker) Work(ctx context.Context, task interface{}) *jobs.WorkerError {
	if task, ok := task.(*Task); ok {
		if task.Node.Source != nil {
			blobs := make([]byte, 0)
			for _, s := range task.Node.Source {
				handler := task.Handlers.Get(s)
				if handler != nil {
					blob, err := handler.Read(ctx, s)
					if err != nil {
						return jobs.NewWorkerError(err, 0)
					}
					blobs = append(blobs, blob...)
				}
			}
			if len(blobs) > 0 {
				paths := []string{f.root}
				for _, parent := range task.Node.Parents() {
					paths = append(paths, parent.Name)
				}
				path := strings.Join(paths, "/")
				if _, err := os.Stat(path); os.IsNotExist(err) {
					if err = os.MkdirAll(path, os.ModePerm); err != nil {
						return jobs.NewWorkerError(err, 0)
					}
				}
				if len(task.Node.Nodes) == 0 {
					filePath := path + "/" + task.Node.Name
					if err := ioutil.WriteFile(filePath, blobs, 0644); err != nil {
						return jobs.NewWorkerError(err, 0)
					}
				}
			}
		}
	}
	return nil
}
