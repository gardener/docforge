// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reactor

import (
	"context"
	"github.com/gardener/docforge/pkg/api"
	"github.com/hashicorp/go-multierror"
	"k8s.io/klog/v2"
	"net/url"
	"time"
)

func tasks(nodes []*api.Node, t *[]interface{}) {
	for _, node := range nodes {
		*t = append(*t, &DocumentWorkTask{
			Node: node,
		})
		if node.Nodes != nil {
			tasks(node.Nodes, t)
		}
	}
}

// Build starts the build operation for a document structure root
// in a locality domain
func (r *Reactor) Build(ctx context.Context, documentationStructure []*api.Node) error {
	var errors *multierror.Error

	klog.V(6).Infoln("Starting download tasks")
	r.DownloadTasks.Start(ctx)
	klog.V(6).Infoln("Starting validator tasks")
	r.ValidatorTasks.Start(ctx)
	if r.GitHubInfoTasks != nil {
		klog.V(6).Infoln("Starting GitHub info tasks")
		r.GitHubInfoTasks.Start(ctx)
	}
	// start document tasks
	klog.V(6).Infoln("Starting document tasks")
	r.DocumentTasks.Start(ctx)

	// Enqueue tasks for document controller
	documentPullTasks := make([]interface{}, 0)
	tasks(documentationStructure, &documentPullTasks)
	for _, task := range documentPullTasks {
		r.DocumentTasks.AddTask(task)
	}
	klog.V(6).Infoln("Tasks for document controller enqueued")

	// waiting all tasks to be processed
	r.reactorWaitGroup.Wait()

	r.DocumentTasks.Stop()
	if r.GitHubInfoTasks != nil {
		r.GitHubInfoTasks.Stop()
	}
	r.ValidatorTasks.Stop()
	r.DownloadTasks.Stop()

	klog.Infof("Document tasks processed: %d\n", r.DocumentTasks.GetProcessedTasksCount())
	klog.Infof("Download tasks processed: %d\n", r.DownloadTasks.GetProcessedTasksCount())
	if r.GitHubInfoTasks != nil {
		klog.Infof("GitHub info tasks processed: %d\n", r.GitHubInfoTasks.GetProcessedTasksCount())
	}
	klog.Infof("Validation tasks processed: %d\n", r.ValidatorTasks.GetProcessedTasksCount())

	for _, rhHost := range []string{"https://github.com/gardener", "https://github.tools.sap/kubernetes", "https://github.wdf.sap.corp/kubernetes"} {
		rh := r.ResourceHandlers.Get(rhHost)
		u, _ := url.Parse(rhHost)
		if rh != nil {
			l, rr, rt, err := rh.GetRateLimit(ctx)
			if err != nil {
				klog.Warningf("Error getting RateLimit for %s: %v\n", u.Host, err)
			} else if l > 0 && rr > 0 {
				klog.Infof("%s RateLimit: %d requests per hour, Remaining: %d, Reset after: %s\n", u.Host, l, rr, rt.Sub(time.Now()).Round(time.Second))
			}
		}
	}

	errList := r.DocumentTasks.GetErrorList()
	if errList != nil {
		errors = multierror.Append(errors, errList)
	}
	errList = r.DownloadTasks.GetErrorList()
	if errList != nil {
		errors = multierror.Append(errors, errList)
	}
	if r.GitHubInfoTasks != nil {
		errList = r.GitHubInfoTasks.GetErrorList()
		if errList != nil {
			errors = multierror.Append(errors, errList)
		}
	}
	errList = r.ValidatorTasks.GetErrorList()
	if errList != nil {
		errors = multierror.Append(errors, errList)
	}

	return errors.ErrorOrNil()
}
