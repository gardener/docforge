// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reactor

import (
	"bytes"
	"context"
	"fmt"
	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/writers"
	"k8s.io/klog/v2"
)

// DocumentWorker defines a structure for processing api.Node document content
type DocumentWorker struct {
	reader               Reader
	writer               writers.Writer
	NodeContentProcessor NodeContentProcessor
	gitHubInfo           GitHubInfo
}

// DocumentWorkTask implements jobs#Task
type DocumentWorkTask struct {
	Node *api.Node
}

// Work implements jobs.WorkerFunc
func (w *DocumentWorker) Work(ctx context.Context, task interface{}) error {
	if dwTask, ok := task.(*DocumentWorkTask); ok {
		var cnt []byte
		path := api.Path(dwTask.Node, "/")
		if len(dwTask.Node.Nodes) == 0 { // Node is considered a `Document Node`
			// Process the node
			bytesBuff := bufPool.Get().(*bytes.Buffer)
			defer bufPool.Put(bytesBuff)
			bytesBuff.Reset()
			if err := w.NodeContentProcessor.Process(ctx, bytesBuff, w.reader, dwTask.Node); err != nil {
				return err
			}
			if bytesBuff.Len() == 0 {
				klog.Warningf("document node processing halted: no content assigned to document node %s/%s", path, dwTask.Node.Name)
				return nil
			}
			cnt = bytesBuff.Bytes()
		}

		if err := w.writer.Write(dwTask.Node.Name, path, cnt, dwTask.Node); err != nil {
			return err
		}
		if w.gitHubInfo != nil && len(cnt) > 0 {
			w.gitHubInfo.WriteGitHubInfo(dwTask.Node)
		}
	} else {
		return fmt.Errorf("incorrect document work task: %T", task)
	}
	return nil
}
