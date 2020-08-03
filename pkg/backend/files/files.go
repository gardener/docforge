package files

import (
	"context"
)

// FileWorker specializes in processing File resources
type FileWorker struct {
}

// FileTask is a unit of work specification that is processed by Worker
type FileTask struct {
	owner      string
	repository string
	entrySHA   string
	entryPath  string
	parentDir  string
}

// NewFileTask creates task for a Worker to execute
func NewFileTask(parentDir, owner, repository, entrySHA, entryPath string) *FileTask {
	return &FileTask{
		parentDir:  parentDir,
		owner:      owner,
		repository: repository,
		entrySHA:   entrySHA,
		entryPath:  entryPath,
	}
}

// Work implements Worker#Work function
func (b *FileWorker) Work(ctx context.Context, task interface{}) *WorkerError {
	// if task, ok := task.(*FileTask); ok {
	// 	if err := createBlobFromTask(ctx, b.Client, task); err != nil {
	// 		return &WorkerError{
	// 			error: err,
	// 		}
	// 	}
	// }
	return nil
}
