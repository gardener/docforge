// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reactor

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"sync"
	"text/template"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/markdown"
	"github.com/gardener/docforge/pkg/processors"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/writers"
	"k8s.io/klog/v2"
)

// DocumentWorker defines a structure for processing api.Node document content
type DocumentWorker struct {
	reader Reader
	writer writers.Writer
	processors.Processor
	NodeContentProcessor NodeContentProcessor
	gitHubInfo           GitHubInfo
	templates            map[string]*template.Template
	rwLock               sync.RWMutex
}

// DocumentWorkTask implements jobs#Task
type DocumentWorkTask struct {
	Node *api.Node
}

// Work implements jobs.WorkerFunc
func (w *DocumentWorker) Work(ctx context.Context, task interface{}) error {
	var (
		documentBytes []byte
	)
	if dwTask, ok := task.(*DocumentWorkTask); ok {
		if len(dwTask.Node.Nodes) == 0 {
			// Node is considered a `Document Node`
			var bytesBuff bytes.Buffer
			doc := &processors.Document{
				Node: dwTask.Node,
			}
			// TODO: separate the logic of Document Processing out of the DocumentWorker
			if len(dwTask.Node.ContentSelectors) > 0 {
				for _, content := range dwTask.Node.ContentSelectors {
					sourceBlob, err := w.reader.Read(ctx, content.Source)
					if err != nil {
						return err
					}
					if len(sourceBlob) == 0 {
						continue
					}
					fm, sourceBlob, err := markdown.StripFrontMatter(sourceBlob)
					if err != nil {
						return err
					}
					doc.AddFrontMatter(fm)
					if sourceBlob, err = w.NodeContentProcessor.ReconcileLinks(ctx, doc, content.Source, sourceBlob); err != nil {
						return err
					}
					bytesBuff.Write(sourceBlob)
				}
			}
			if dwTask.Node.Template != nil {
				vars := map[string]string{}
				for varName, content := range dwTask.Node.Template.Sources {
					sourceBlob, err := w.reader.Read(ctx, content.Source)
					if err != nil {
						return err
					}
					if len(sourceBlob) == 0 {
						continue
					}
					fm, sourceBlob, err := markdown.StripFrontMatter(sourceBlob)
					if err != nil {
						return err
					}
					doc.AddFrontMatter(fm)
					if sourceBlob, err = w.NodeContentProcessor.ReconcileLinks(ctx, doc, content.Source, sourceBlob); err != nil {
						return err
					}
					vars[varName] = string(sourceBlob)
				}
				var (
					templateBlob []byte
					tmpl         *template.Template
					err          error
				)
				if tmpl = w.getTemplate(dwTask.Node.Template.Path); tmpl == nil {
					if templateBlob, err = w.reader.Read(ctx, dwTask.Node.Template.Path); err != nil {
						return err
					}
					if tmpl, err = template.New(dwTask.Node.Template.Path).Parse(string(templateBlob)); err != nil {
						return err
					}
					w.setTemplate(dwTask.Node.Template.Path, tmpl)
				}
				if err := tmpl.Execute(&bytesBuff, vars); err != nil {
					return err
				}
			}
			if len(dwTask.Node.Source) > 0 {
				sourceBlob, err := w.reader.Read(ctx, dwTask.Node.Source)
				if err != nil {
					if resourceNotFound, ok := err.(resourcehandlers.ErrResourceNotFound); ok {
						klog.Warningf("reading %s failed: %s\n", dwTask.Node.Source, resourceNotFound)
						return nil
					}
					return err
				}
				if len(sourceBlob) == 0 {
					klog.Warningf("No content read from node %s source %s:", dwTask.Node.Name, dwTask.Node.Source)
					return nil
				}
				fm, sourceBlob, err := markdown.StripFrontMatter(sourceBlob)
				if err != nil {
					return err
				}
				doc.AddFrontMatter(fm)
				if sourceBlob, err = w.NodeContentProcessor.ReconcileLinks(ctx, doc, dwTask.Node.Source, sourceBlob); err != nil {
					return err
				}
				bytesBuff.Write(sourceBlob)
			}
			if bytesBuff.Len() == 0 && len(doc.FrontMatter) == 0 {
				klog.Warningf("Document node processing halted: No content assigned to document node %s", dwTask.Node.Name)
				return nil
			}
			var err error
			documentBytes, err = ioutil.ReadAll(&bytesBuff)
			if err != nil {
				return err
			}
			doc.Append(documentBytes)
			if w.Processor != nil {
				if err := w.Processor.Process(doc); err != nil {
					return err
				}
			}
			documentBytes = doc.DocumentBytes
			if documentBytes, err = markdown.InsertFrontMatter(doc.FrontMatter, documentBytes); err != nil {
				return err
			}
		}

		path := api.Path(dwTask.Node, "/")
		if err := w.writer.Write(dwTask.Node.Name, path, documentBytes, dwTask.Node); err != nil {
			return err
		}
		if w.gitHubInfo != nil && len(documentBytes) > 0 {
			w.gitHubInfo.WriteGitHubInfo(dwTask.Node)
		}
	} else {
		return fmt.Errorf("incorrect document work task: %T", task)
	}
	return nil
}

func (w *DocumentWorker) getTemplate(name string) *template.Template {
	w.rwLock.RLock()
	defer w.rwLock.RUnlock()
	if tmpl, ok := w.templates[name]; ok {
		return tmpl
	}
	return nil
}

func (w *DocumentWorker) setTemplate(name string, tmpl *template.Template) {
	w.rwLock.Lock()
	defer w.rwLock.Unlock()
	w.templates[name] = tmpl
}
