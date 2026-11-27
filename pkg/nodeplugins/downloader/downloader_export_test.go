// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package downloader

import (
	"context"

	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/writers"
)

// NewDownloaderWithFlag creates a ResourceDownloadWorker with the failOnStripped flag set.
// Only compiled during `go test`.
func NewDownloaderWithFlag(r registry.Interface, w writers.Writer, failOnStripped bool) *ResourceDownloadWorker {
	return &ResourceDownloadWorker{registry: r, writer: w, failOnStripped: failOnStripped}
}

// ExecuteTask calls the unexported execute method with a typed downloadTask.
// Only compiled during `go test`.
func (d *ResourceDownloadWorker) ExecuteTask(ctx context.Context, source, dest string, trusted bool) error {
	return d.execute(ctx, &downloadTask{source, dest, trusted})
}
