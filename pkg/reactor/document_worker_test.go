// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reactor

import (
	"context"
	"fmt"
	"github.com/gardener/docforge/pkg/resourcehandlers/testhandler"
	"reflect"
	"testing"
	"time"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/jobs"
	"github.com/gardener/docforge/pkg/processors"
	"github.com/gardener/docforge/pkg/resourcehandlers"
)

type TestWriter struct {
	output map[string][]byte
}

func (w *TestWriter) Write(name, path string, resourceContent []byte, node *api.Node) error {
	w.output[fmt.Sprintf("%s/%s", path, name)] = resourceContent
	return nil
}

type TestReader struct {
	input map[string][]byte
}

func (r *TestReader) Read(ctx context.Context, source string) ([]byte, error) {
	return r.input[source], nil
}

type TestProcessor struct {
	withArgs func(document *processors.Document) error
}

func (p *TestProcessor) Process(document *processors.Document) error {
	if p.withArgs != nil {
		return p.withArgs(document)
	}
	return nil
}

func TestDocumentWorkerWork(t *testing.T) {
	testOutput := "#Heading 1\n"
	rhRegistry := resourcehandlers.NewRegistry(testhandler.NewTestResourceHandler())
	testworker := &DocumentWorker{
		Writer: &TestWriter{
			make(map[string][]byte),
		},
		Reader: &TestReader{
			make(map[string][]byte),
		},
		Processor: &TestProcessor{
			func(documnet *processors.Document) error {
				return nil
			},
		},
		NodeContentProcessor: &nodeContentProcessor{
			downloadController: NewDownloadController(&TestReader{
				make(map[string][]byte),
			}, &TestWriter{
				make(map[string][]byte),
			}, 1, false, rhRegistry),
			resourceHandlers: rhRegistry,
		},
		GitHubInfoController: nil,
		templates:            nil,
	}

	testCases := []struct {
		name                 string
		tasks                interface{}
		readerInput          map[string][]byte
		processorCb          func(document *processors.Document) error
		expectedWriterOutput map[string][]byte
		expectederror        *jobs.WorkerError
	}{
		{
			name: "it reads source, processes and writes it",
			tasks: &DocumentWorkTask{
				&api.Node{
					Name:             "sourcemd",
					ContentSelectors: []api.ContentSelector{{Source: "testsource"}},
				},
			},
			readerInput: map[string][]byte{
				"testsource": []byte(testOutput),
			},
			processorCb: nil,
			expectedWriterOutput: map[string][]byte{
				"/sourcemd": []byte(testOutput),
			},
			expectederror: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tReader := testworker.Reader.(*TestReader)
			tReader.input = tc.readerInput
			tWriter := testworker.Writer.(*TestWriter)
			tWriter.output = make(map[string][]byte)
			tProcessor := testworker.Processor.(*TestProcessor)
			tProcessor.withArgs = tc.processorCb
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			if err := testworker.Work(ctx, tc.tasks, jobs.NewWorkQueue(5)); err != nil {
				t.Errorf("expected error nil != %v", err)
			}
			if !reflect.DeepEqual(tWriter.output, tc.expectedWriterOutput) {
				t.Errorf("expected writer output %v !=  %v", tc.expectedWriterOutput, tWriter.output)
			}
		})
	}
}
