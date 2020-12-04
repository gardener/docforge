// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reactor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sync"

	"k8s.io/klog/v2"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/git"
	"github.com/gardener/docforge/pkg/jobs"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	nodeutil "github.com/gardener/docforge/pkg/util/node"
	"github.com/gardener/docforge/pkg/writers"
	"github.com/google/go-github/v32/github"
)

// GitInfoController is the functional interface for manageing Git info
type GitInfoController interface {
	jobs.Controller
	WriteGitInfo(ctx context.Context, documentPath string, node *api.Node)
}

// GitInfoTask wraps the parameters for WriteGitInfo
type GitInfoTask struct {
	Node *api.Node
}

// gitInfoController implements reactor#DownloadController
type gitInfoController struct {
	jobs.Controller
	job *jobs.Job
	writers.Writer
	rwLock       sync.RWMutex
	contributors map[string]*github.User
}

type gitInfoWorker struct {
	writers.Writer
	Reader
}

type gitInfoReader struct {
	rhs resourcehandlers.Registry
}

func (r *gitInfoReader) Read(ctx context.Context, source string) ([]byte, error) {
	if handler := r.rhs.Get(source); handler != nil {
		return handler.ReadGitInfo(ctx, source)
	}
	return nil, nil
}

// NewGitInfoController creates Controller object for wokring with Git info
func NewGitInfoController(reader Reader, writer writers.Writer, workersCount int, failFast bool, rhs resourcehandlers.Registry) GitInfoController {
	if reader == nil {
		reader = &gitInfoReader{
			rhs: rhs,
		}
	}
	if writer == nil {
		panic(fmt.Sprint("Invalid argument: writer is nil"))
		//writer = &writers.FSWriter{}
	}

	d := &gitInfoWorker{
		Reader: reader,
		Writer: writer,
	}

	job := &jobs.Job{
		ID:         "GitInfo",
		FailFast:   failFast,
		MaxWorkers: workersCount,
		MinWorkers: workersCount,
		Queue:      jobs.NewWorkQueue(100),
	}
	job.SetIsWorkerExitsOnEmptyQueue(true)

	controller := &gitInfoController{
		Controller:   jobs.NewController(job),
		job:          job,
		Writer:       writer,
		contributors: map[string]*github.User{},
	}
	controller.job.Worker = withGitInfoController(d, controller)
	return controller
}

func withGitInfoController(gitInfoWorker *gitInfoWorker, ctrl *gitInfoController) jobs.WorkerFunc {
	return func(ctx context.Context, task interface{}, wq jobs.WorkQueue) *jobs.WorkerError {
		return gitInfoWorker.Work(ctx, ctrl, task, wq)
	}
}

func (d *gitInfoWorker) Work(ctx context.Context, ctrl *gitInfoController, task interface{}, wq jobs.WorkQueue) *jobs.WorkerError {
	if task, ok := task.(*GitInfoTask); ok {
		node := task.Node
		sources := []string{}
		for _, cs := range node.ContentSelectors {
			sources = append(sources, cs.Source)
		}
		if len(node.Source) > 0 {
			sources = append(sources, node.Source)
		}
		var (
			b               bytes.Buffer
			info, infoBytes []byte
			err             error
		)
		if len(sources) == 0 {
			klog.V(6).Infof("skip git info for container nodes\n")
			return nil
		}
		for _, s := range sources {
			klog.V(6).Infof("reading git info for %s\n", s)
			if info, err = d.Read(ctx, s); err != nil {
				return jobs.NewWorkerError(err, 0)
			}
			if info == nil {
				continue
			}
			b.Write(info)
			if err := ctrl.updateContributors(info); err != nil {
				return jobs.NewWorkerError(err, 0)
			}
		}

		if infoBytes, err = ioutil.ReadAll(&b); err != nil {
			return jobs.NewWorkerError(err, 0)
		}
		nodepath := nodeutil.Path(node, "/")
		klog.V(6).Infof("writing git info for node %s/%s\n", nodepath, node.Name)
		if err = d.Write(node.Name, nodepath, infoBytes, node); err != nil {
			return jobs.NewWorkerError(err, 0)
		}
	}
	return nil
}

func (g *gitInfoController) WriteGitInfo(ctx context.Context, documentPath string, node *api.Node) {
	if !g.Enqueue(ctx, &GitInfoTask{
		Node: node,
	}) {
		klog.Warning("Scheduling git info write failed")
	}
}

func (g *gitInfoController) Stop(shutdownCh chan struct{}) {
	defer g.finalize()
	// Check and exit immediately if nothing in queue and blocked waiting
	if g.job.Queue.Count() == 0 {
		g.Controller.Shutdown()
	}
	g.Controller.Stop(shutdownCh)
}

func (g *gitInfoController) setContributor(email string, user *github.User) {
	defer g.rwLock.Unlock()
	g.rwLock.Lock()
	g.contributors[email] = user
}

func (g *gitInfoController) updateContributors(info []byte) error {
	var contributors []*github.User
	gitInfo := &git.GitInfo{}
	if err := json.Unmarshal(info, &gitInfo); err != nil {
		return err
	}
	if gitInfo.Contributors == nil {
		gitInfo.Contributors = []*github.User{}
	}
	if gitInfo.Author != nil {
		contributors = append(gitInfo.Contributors, gitInfo.Author)
	}
	for _, c := range contributors {
		if len(c.GetEmail()) > 0 {
			g.setContributor(c.GetEmail(), c)
		}
	}
	return nil
}

func (g *gitInfoController) finalize() {
	var (
		blob []byte
		err  error
	)
	defer g.rwLock.Unlock()
	g.rwLock.Lock()
	if blob, err = json.MarshalIndent(g.contributors, "", "  "); err != nil {
		klog.Errorf("writing contributors.json failed: %s", err.Error())
	}
	if err := g.Writer.Write("contributors", "", blob, nil); err != nil {
		klog.Errorf("writing contributors.json failed: %s", err.Error())
	}
}
