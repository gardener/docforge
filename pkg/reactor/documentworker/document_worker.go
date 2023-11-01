// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package documentworker

import (
	"bytes"
	"context"
	"fmt"
	"sync"

	"github.com/gardener/docforge/pkg/document"
	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/reactor/githubinfo"
	"github.com/gardener/docforge/pkg/reactor/jobs"
	"github.com/gardener/docforge/pkg/readers"
	"github.com/gardener/docforge/pkg/writers"
	"k8s.io/klog/v2"
)

var (
	// pool with reusable buffers
	bufPool = sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
		},
	}
)

// DocumentWorker defines a structure for processing manifest.Node document content
type DocumentWorker struct {
	reader               readers.Reader
	writer               writers.Writer
	NodeContentProcessor document.NodeContentProcessor
	gitHubInfo           githubinfo.GitHubInfo
}

func New(workerCount int, failFast bool, wg *sync.WaitGroup, reader readers.Reader, writer writers.Writer, ncp document.NodeContentProcessor, gitHubInfo githubinfo.GitHubInfo) (*DocumentWorker, *jobs.JobQueue, error) {
	worker := &DocumentWorker{
		writer:               writer,
		reader:               reader,
		NodeContentProcessor: ncp,
		gitHubInfo:           gitHubInfo,
	}
	queue, err := jobs.NewJobQueue("Document", workerCount, worker.execute, failFast, wg)
	if err != nil {
		return nil, nil, err
	}
	return worker, queue, nil
}

// Work implements jobs.WorkerFunc
func (w *DocumentWorker) execute(ctx context.Context, task interface{}) error {
	node, ok := task.(*manifest.Node)
	if !ok {
		return fmt.Errorf("incorrect document work task: %T", task)
	}
	return w.Work(ctx, node)
}

func (w *DocumentWorker) Work(ctx context.Context, node *manifest.Node) error {
	var cnt []byte
	path := node.Path
	if node.IsDocument() { // Node is considered a `Document Node`
		// Process the node
		bytesBuff := bufPool.Get().(*bytes.Buffer)
		defer bufPool.Put(bytesBuff)
		bytesBuff.Reset()
		if err := w.NodeContentProcessor.Process(ctx, bytesBuff, w.reader, node); err != nil {
			return err
		}
		if bytesBuff.Len() == 0 {
			klog.Warningf("document node processing halted: no content assigned to document node %s/%s", path, node.Name())
			return nil
		}
		cnt = bytesBuff.Bytes()
	}

	if err := w.writer.Write(node.Name(), path, cnt, node); err != nil {
		return err
	}
	if w.gitHubInfo != nil && len(cnt) > 0 {
		w.gitHubInfo.WriteGitHubInfo(node)
	}
	return nil
}
