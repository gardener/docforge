package reactor

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/gardener/docode/pkg/api"
	"github.com/gardener/docode/pkg/jobs"
	"github.com/gardener/docode/pkg/resourcehandlers"
)

type TestWriter struct {
	output map[string][]byte
}

func (w *TestWriter) Write(name, path string, resourceContent []byte) error {
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
	withArgs func(documentBlob []byte, node *api.Node) ([]byte, error)
}

func (p *TestProcessor) Process(documentBlob []byte, node *api.Node) ([]byte, error) {
	if p.withArgs != nil {
		return p.withArgs(documentBlob, node)
	}
	return documentBlob, nil
}

func TestDocumentWorkerWork(t *testing.T) {
	testworker := &DocumentWorker{
		&TestWriter{
			make(map[string][]byte),
		},
		&TestReader{
			make(map[string][]byte),
		},
		&TestProcessor{
			func(documentBlob []byte, node *api.Node) ([]byte, error) {
				return documentBlob, nil
			},
		},
		&ContentProcessor{
			resourceAbsLink: make(map[string]string),
		},
		make(chan *ResourceData),
	}

	testCases := []struct {
		name                 string
		tasks                interface{}
		readerInput          map[string][]byte
		processorCb          func(documentBlob []byte, node *api.Node) ([]byte, error)
		expectedWriterOutput map[string][]byte
		expectederror        *jobs.WorkerError
	}{
		{
			"it reads source, processes and writes it",
			&DocumentWorkTask{
				&api.Node{
					Name:             "sourcemd",
					ContentSelectors: []api.ContentSelector{{Source: "testsource"}},
				},
			},
			map[string][]byte{
				"testsource": []byte("#Heading 1"),
			},
			nil,
			map[string][]byte{
				"/sourcemd": []byte("#Heading 1"),
			},
			nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resourcehandlers.Load(&FakeResourceHandler{})
			tReader := testworker.Reader.(*TestReader)
			tReader.input = tc.readerInput
			tWriter := testworker.Writer.(*TestWriter)
			tWriter.output = make(map[string][]byte)
			tProcessor := testworker.Processor.(*TestProcessor)
			tProcessor.withArgs = tc.processorCb
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			if err := testworker.Work(ctx, tc.tasks); err != nil {
				t.Errorf("expected error nil != %v", err)
			}
			if !reflect.DeepEqual(tWriter.output, tc.expectedWriterOutput) {
				t.Errorf("expected writer output %v !=  %v", tc.expectedWriterOutput, tWriter.output)
			}
		})
	}
}
