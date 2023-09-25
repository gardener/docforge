// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reactor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/gardener/docforge/pkg/jobs"
	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/writers"
	"k8s.io/klog/v2"
)

// GitHubInfo is the functional interface for writing GitHub infos
//
//counterfeiter:generate . GitHubInfo
type GitHubInfo interface {
	// WriteGitHubInfo writes GitHub info for an manifest.Node in a separate goroutine
	// returns true if the task was added for processing, false if it was skipped
	WriteGitHubInfo(node *manifest.Node) bool
}

type gitHubInfo struct {
	queue *jobs.JobQueue
}

// NewGitHubInfo creates GitHubInfo object for writing GitHub infos
func NewGitHubInfo(queue *jobs.JobQueue) GitHubInfo {
	return &gitHubInfo{
		queue: queue,
	}
}

func (g *gitHubInfo) WriteGitHubInfo(node *manifest.Node) bool {
	added := g.queue.AddTask(&GitHubInfoTask{Node: node})
	if !added {
		klog.Warningf("scheduling github info write failed for node %v\n", node)
	}
	return added
}

// GitHubInfoTask wraps the parameters for WriteGitHubInfo
type GitHubInfoTask struct {
	Node *manifest.Node
}

type gitHubInfoWorker struct {
	// reader for GitHub info
	reader Reader
	// writer for GitHub info
	writer writers.Writer
}

// GitHubInfoWork is jobs.WorkerFunc for GitHub infos
func (w *gitHubInfoWorker) GitHubInfoWork(ctx context.Context, task interface{}) error {
	if ghTask, ok := task.(*GitHubInfoTask); ok {
		node := ghTask.Node
		var sources []string
		// append source
		if len(node.Source) > 0 {
			sources = append(sources, node.Source)
		}
		// append multi content
		for _, src := range node.MultiSource {
			sources = append(sources, src)
		}
		var (
			b    bytes.Buffer
			info []byte
			err  error
		)
		if len(sources) == 0 {
			klog.V(6).Infof("skip git info for container node: %v\n", node)
			return nil
		}
		for _, s := range sources {
			klog.V(6).Infof("reading git info for %s\n", s)
			if info, err = w.reader.Read(ctx, s); err != nil {
				if _, ok = err.(resourcehandlers.ErrResourceNotFound); ok {
					// for missing resources just log warning
					klog.Warningf("reading GitHub info for %s fails: %v\n", s, err)
					continue
				}
				return fmt.Errorf("failed to read git info for %s: %v", s, err)
			}
			if info == nil {
				continue
			}
			b.Write(info)
		}
		nodePath := node.Path
		klog.V(6).Infof("writing git info for node %s/%s\n", nodePath, node.Name())
		if err = w.writer.Write(node.Name(), nodePath, b.Bytes(), node); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("incorrect github info task: %T", task)
	}
	return nil
}

// GitHubInfoWorkerFunc returns the GitHubInfoWork worker func
func GitHubInfoWorkerFunc(reader Reader, writer writers.Writer) (jobs.WorkerFunc, error) {
	if reader == nil || reflect.ValueOf(reader).IsNil() {
		return nil, errors.New("invalid argument: reader is nil")
	}
	if writer == nil || reflect.ValueOf(writer).IsNil() {
		return nil, errors.New("invalid argument: writer is nil")
	}
	ghWorker := &gitHubInfoWorker{
		reader: reader,
		writer: writer,
	}
	return ghWorker.GitHubInfoWork, nil
}
