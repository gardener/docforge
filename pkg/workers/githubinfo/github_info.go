// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package githubinfo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/writers"
	"k8s.io/klog/v2"
)

// Worker github info worker
type Worker struct {
	registry registry.Interface
	writer   writers.Writer
}

// NewGithubWorker creates new Worker object
func NewGithubWorker(registry registry.Interface, writer writers.Writer) (*Worker, error) {
	if registry == nil || reflect.ValueOf(registry).IsNil() {
		return nil, errors.New("invalid argument: reader is nil")
	}
	if writer == nil || reflect.ValueOf(writer).IsNil() {
		return nil, errors.New("invalid argument: writer is nil")
	}
	return &Worker{
		registry,
		writer,
	}, nil
}

// WriteGithubInfo writes github info to writer for a given node
func (w *Worker) WriteGithubInfo(ctx context.Context, node *manifest.Node) error {
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
		// read github info
		if info, err = w.registry.ReadGitInfo(ctx, s); err != nil {
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
