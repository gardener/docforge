package githubinfo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/readers"
	resourcehandlers "github.com/gardener/docforge/pkg/readers/repositoryhosts"
	"github.com/gardener/docforge/pkg/writers"
	"k8s.io/klog/v2"
)

type GitHubInfoWorker struct {
	reader readers.Reader
	writer writers.Writer
}

func NewGithubWorker(reader readers.Reader, writer writers.Writer) (*GitHubInfoWorker, error) {
	if reader == nil || reflect.ValueOf(reader).IsNil() {
		return nil, errors.New("invalid argument: reader is nil")
	}
	if writer == nil || reflect.ValueOf(writer).IsNil() {
		return nil, errors.New("invalid argument: writer is nil")
	}
	return &GitHubInfoWorker{
		reader: reader,
		writer: writer,
	}, nil
}

func (w *GitHubInfoWorker) WriteGithubInfo(ctx context.Context, node *manifest.Node) error {
	var (
		b       bytes.Buffer
		info    []byte
		err     error
		sources []string
	)
	if len(node.Source) > 0 {
		sources = append(sources, node.Source)
	}
	sources = append(sources, node.MultiSource...)

	if len(sources) == 0 {
		klog.V(6).Infof("skip git info for container node: %v\n", node)
		return nil
	}
	for _, s := range sources {
		klog.V(6).Infof("reading git info for %s\n", s)
		if info, err = w.reader.Read(ctx, s); err != nil {
			if _, ok := err.(resourcehandlers.ErrResourceNotFound); ok {
				klog.Warningf("reading GitHub info for %s fails: %v\n", s, err)
				continue
			}
			return fmt.Errorf("failed to read git info for %s: %v", s, err)
		}
		if info != nil {
			b.Write(info)
		}
	}
	nodePath := node.Path
	klog.V(6).Infof("writing git info for node %s/%s\n", nodePath, node.Name())
	if err = w.writer.Write(node.Name(), nodePath, b.Bytes(), node); err != nil {
		return err
	}
	return nil
}
