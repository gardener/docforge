// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package downloader

import (
	"context"
	"path"

	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/writers"
)

// ResourceDownloadWorker is the structure that processes downloads
type ResourceDownloadWorker struct {
	registry registry.Interface
	writer   writers.Writer
}

// NewDownloader creates new downloader
func NewDownloader(registry registry.Interface, writer writers.Writer) (*ResourceDownloadWorker, error) {
	return &ResourceDownloadWorker{
		registry: registry,
		writer:   writer,
	}, nil
}

// Download downloads source in destinationPath
func (d *ResourceDownloadWorker) Download(ctx context.Context, source string, destinationPath string) error {
	reosurceURL, err := d.registry.ResourceURL(source)
	if err != nil {
		return err
	}
	blob, err := d.registry.Read(ctx, reosurceURL.ResourceURL())
	if err != nil {
		return err
	}

	if err = d.writer.Write(path.Base(destinationPath), path.Dir(destinationPath), blob, nil, nil); err != nil {
		return err
	}
	return nil
}
