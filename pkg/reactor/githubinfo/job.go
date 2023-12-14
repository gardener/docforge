// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package githubinfo

import (
	"context"
	"fmt"
	"sync"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/reactor/jobs"
	"github.com/gardener/docforge/pkg/readers"
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
	*GitHubInfoWorker
	queue *jobs.JobQueue
}

// New creates GitHubInfo object for writing GitHub infos
func New(workerCount int, failFast bool, wg *sync.WaitGroup, reader readers.Reader, writer writers.Writer) (GitHubInfo, jobs.QueueController, error) {
	ghInfoWorker, err := NewGithubWorker(reader, writer)
	if err != nil {
		return nil, nil, err
	}
	queue, err := jobs.NewJobQueue("GitHubInfo", workerCount, ghInfoWorker.execute, failFast, wg)
	if err != nil {
		return nil, nil, err
	}
	ghInfo := &gitHubInfo{
		ghInfoWorker,
		queue,
	}
	return ghInfo, queue, nil
}

func (g *gitHubInfo) WriteGitHubInfo(node *manifest.Node) bool {
	added := g.queue.AddTask(&gitHubInfoTask{Node: node})
	if !added {
		klog.Warningf("scheduling github info write failed for node %v\n", node)
	}
	return added
}

// GitHubInfoTask wraps the parameters for WriteGitHubInfo
type gitHubInfoTask struct {
	Node *manifest.Node
}

// GitHubInfoWork is jobs.WorkerFunc for GitHub infos
func (w *GitHubInfoWorker) execute(ctx context.Context, task interface{}) error {
	ghTask, ok := task.(*gitHubInfoTask)
	if !ok {
		return fmt.Errorf("incorrect github info task: %T", task)
	}
	return w.WriteGithubInfo(ctx, ghTask.Node)
}
