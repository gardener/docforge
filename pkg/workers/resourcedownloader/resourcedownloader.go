// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package resourcedownloader

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	"github.com/gardener/docforge/pkg/writers"
	"k8s.io/klog/v2"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate -header ../../../license_prefix.txt

// Interface encapsulates activities for asynchronous and parallel scheduling and download of resources
//
//counterfeiter:generate . Interface
type Interface interface {
	// Schedule is a typesafe wrapper for enqueuing download tasks. An error is returned if scheduling fails.
	Download(ctx context.Context, source string, target string, document string) error
}

// ResourceDownloadWorker is the structure that processes downloads
type ResourceDownloadWorker struct {
	registry registry.Interface
	writer   writers.Writer
	// lock for accessing the downloadedResources map
	mux sync.Mutex
	// map with downloaded resources
	downloadedResources map[string]struct{}
}

// NewDownloader creates new downloader
func NewDownloader(registry registry.Interface, writer writers.Writer) (*ResourceDownloadWorker, error) {
	if registry == nil || reflect.ValueOf(registry).IsNil() {
		return nil, errors.New("invalid argument: reader is nil")
	}
	if writer == nil || reflect.ValueOf(writer).IsNil() {
		return nil, errors.New("invalid argument: writer is nil")
	}
	return &ResourceDownloadWorker{
		registry:            registry,
		writer:              writer,
		downloadedResources: make(map[string]struct{}),
	}, nil
}

// Download downloads source as target
func (d *ResourceDownloadWorker) Download(ctx context.Context, source string, target string, document string) error {
	if !d.shouldDownload(source) {
		return nil
	}
	if err := d.download(ctx, source, target); err != nil {
		dErr := fmt.Errorf("downloading %s as %s from document %s failed: %v", source, target, document, err)
		if _, ok := err.(repositoryhost.ErrResourceNotFound); ok {
			// for missing resources just log warning
			klog.Warning(dErr.Error())
			return nil
		}
		return dErr
	}
	return nil
}

// shouldDownload checks whether a download task for the same Source is being processed
func (d *ResourceDownloadWorker) shouldDownload(Source string) bool {
	d.mux.Lock()
	defer d.mux.Unlock()
	if _, ok := d.downloadedResources[Source]; ok {
		return false
	}
	d.downloadedResources[Source] = struct{}{}
	return true
}

func (d *ResourceDownloadWorker) download(ctx context.Context, Source string, Target string) error {
	reosurceURL, err := d.registry.ResourceURL(Source)
	if err != nil {
		return err
	}
	blob, err := d.registry.Read(ctx, reosurceURL.ResourceURL())
	if err != nil {
		return err
	}
	if err = d.writer.Write(Target, "", blob, nil, nil); err != nil {
		return err
	}
	return nil
}
