// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reactor

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sync"
	"text/template"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/jobs"
	"github.com/gardener/docforge/pkg/markdown"
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
	templates            map[string]*template.Template
	rwLock               sync.RWMutex
}

// DocumentWorkTask implements jobs#Task
type DocumentWorkTask struct {
	Node *api.Node
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

func (w *DocumentWorker) getTemplate(name string) *template.Template {
	defer w.rwLock.Unlock()
	w.rwLock.Lock()
	if tmpl, ok := w.templates[name]; ok {
		return tmpl
	}
	return nil
}
func (w *DocumentWorker) setTemplate(name string, tmpl *template.Template) {
	defer w.rwLock.Unlock()
	w.rwLock.Lock()
	w.templates[name] = tmpl
}

// Work implements Worker#Work function
func (w *DocumentWorker) Work(ctx context.Context, task interface{}, wq jobs.WorkQueue) *jobs.WorkerError {
	var (
		documentBytes []byte
		err           error
	)
	if task, ok := task.(*DocumentWorkTask); ok {
		if len(task.Node.Nodes) == 0 {
			// Node is considered a `Document Node`
			var bytesBuff bytes.Buffer
			doc := &processors.Document{
				Node: task.Node,
			}
			// TODO: separate the logic of Document Processing out of the DocumentWorker
			if len(task.Node.ContentSelectors) > 0 {
				for _, content := range task.Node.ContentSelectors {
					sourceBlob, err := w.Reader.Read(ctx, content.Source)
					if err != nil {
						return jobs.NewWorkerError(err, 0)
					}
					if len(sourceBlob) == 0 {
						continue
					}
					fm, sourceBlob, err := markdown.StripFrontMatter(sourceBlob)
					if err != nil {
						return jobs.NewWorkerError(err, 0)
					}
					doc.AddFrontMatter(fm)
					if sourceBlob, err = w.NodeContentProcessor.ReconcileLinks(ctx, doc, content.Source, sourceBlob); err != nil {
						return jobs.NewWorkerError(err, 0)
					}
					bytesBuff.Write(sourceBlob)
				}
			}
			if task.Node.Template != nil {
				vars := map[string]string{}
				for varName, content := range task.Node.Template.Sources {
					sourceBlob, err := w.Reader.Read(ctx, content.Source)
					if err != nil {
						return jobs.NewWorkerError(err, 0)
					}
					if len(sourceBlob) == 0 {
						continue
					}
					fm, sourceBlob, err := markdown.StripFrontMatter(sourceBlob)
					if err != nil {
						return jobs.NewWorkerError(err, 0)
					}
					doc.AddFrontMatter(fm)
					if sourceBlob, err = w.NodeContentProcessor.ReconcileLinks(ctx, doc, content.Source, sourceBlob); err != nil {
						return jobs.NewWorkerError(err, 0)
					}
					vars[varName] = string(sourceBlob)
				}
				var (
					templateBlob []byte
					tmpl         *template.Template
					err          error
				)
				if tmpl = w.getTemplate(task.Node.Template.Path); tmpl == nil {
					if templateBlob, err = w.Reader.Read(ctx, task.Node.Template.Path); err != nil {
						return jobs.NewWorkerError(err, 0)
					}
					if tmpl, err = template.New(task.Node.Template.Path).Parse(string(templateBlob)); err != nil {
						return jobs.NewWorkerError(err, 0)
					}
					w.setTemplate(task.Node.Template.Path, tmpl)
				}
				if err := tmpl.Execute(&bytesBuff, vars); err != nil {
					return jobs.NewWorkerError(err, 0)
				}
			}
			if len(task.Node.Source) > 0 {
				sourceBlob, err := w.Reader.Read(ctx, task.Node.Source)
				if err != nil {
					if resourceNotFound, ok := err.(resourcehandlers.ErrResourceNotFound); ok {
						klog.Warningf("reading %s failed: %s\n", task.Node.Source, resourceNotFound)
						return nil
					} else {
						return jobs.NewWorkerError(err, 0)
					}
				}
				if len(sourceBlob) == 0 {
					klog.Warningf("No content read from node %s source %s:", task.Node.Name, task.Node.Source)
					return nil
				}
				fm, sourceBlob, err := markdown.StripFrontMatter(sourceBlob)
				if err != nil {
					return jobs.NewWorkerError(err, 0)
				}
				doc.AddFrontMatter(fm)
				if sourceBlob, err = w.NodeContentProcessor.ReconcileLinks(ctx, doc, task.Node.Source, sourceBlob); err != nil {
					return jobs.NewWorkerError(err, 0)
				}
				bytesBuff.Write(sourceBlob)
			}
			if bytesBuff.Len() == 0 && len(doc.FrontMatter) == 0 {
				klog.Warningf("Document node processing halted: No content assigned to document node %s", task.Node.Name)
				return nil
			}
			documentBytes, err = ioutil.ReadAll(&bytesBuff)
			if err != nil {
				return jobs.NewWorkerError(err, 0)
			}
			doc.Append(documentBytes)
			if w.Processor != nil {
				if err := w.Processor.Process(doc); err != nil {
					return jobs.NewWorkerError(err, 0)
				}
			}
			documentBytes = doc.DocumentBytes
			if documentBytes, err = markdown.InsertFrontMatter(doc.FrontMatter, documentBytes); err != nil {
				return jobs.NewWorkerError(err, 0)
			}
		}

		path := utilnode.Path(task.Node, "/")
		if err := w.Writer.Write(task.Node.Name, path, documentBytes, task.Node); err != nil {
			return jobs.NewWorkerError(err, 0)
		}

		if w.GitHubInfoController != nil && len(documentBytes) > 0 {
			w.GitHubInfoController.WriteGitInfo(ctx, filepath.Join(path, task.Node.Name), task.Node)
		}
	}
	return nil
}
