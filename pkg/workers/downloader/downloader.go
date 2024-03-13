// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package downloader

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"reflect"
	"strings"
	"sync"

	"github.com/gardener/docforge/pkg/readers/repositoryhosts"
	"github.com/gardener/docforge/pkg/readers/resource"
	"github.com/gardener/docforge/pkg/writers"
	"k8s.io/klog/v2"
)

// DownloadWorker is the structure that processes downloads
type DownloadWorker struct {
	registry repositoryhosts.Registry
	writer   writers.Writer
	// lock for accessing the downloadedResources map
	mux sync.Mutex
	// map with downloaded resources
	downloadedResources map[string]struct{}
}

// NewDownloader creates new downloader
func NewDownloader(registry repositoryhosts.Registry, writer writers.Writer) (*DownloadWorker, error) {
	if registry == nil || reflect.ValueOf(registry).IsNil() {
		return nil, errors.New("invalid argument: reader is nil")
	}
	if writer == nil || reflect.ValueOf(writer).IsNil() {
		return nil, errors.New("invalid argument: writer is nil")
	}
	return &DownloadWorker{
		registry:            registry,
		writer:              writer,
		downloadedResources: make(map[string]struct{}),
	}, nil
}

// DownloadResourceName create resource name that will be dowloaded from a resource link
func DownloadResourceName(resource resource.Resource, document string) string {
	resourcePath := resource.String()
	mdsum := md5.Sum([]byte(resourcePath + document))
	ext := path.Ext(resourcePath)
	name := strings.TrimSuffix(path.Base(resourcePath), ext)
	hash := hex.EncodeToString(mdsum[:])[:6]
	return fmt.Sprintf("%s_%s%s", name, hash, ext)

}

// Download downloads source as target
func (d *DownloadWorker) Download(ctx context.Context, source string, target string, document string) error {
	if !d.shouldDownload(source) {
		return nil
	}
	if err := d.download(ctx, source, target); err != nil {
		dErr := fmt.Errorf("downloading %s as %s from document %s failed: %v", source, target, document, err)
		if _, ok := err.(repositoryhosts.ErrResourceNotFound); ok {
			// for missing resources just log warning
			klog.Warning(dErr.Error())
			return nil
		}
		return dErr
	}
	return nil
}

// shouldDownload checks whether a download task for the same Source is being processed
func (d *DownloadWorker) shouldDownload(Source string) bool {
	d.mux.Lock()
	defer d.mux.Unlock()
	if _, ok := d.downloadedResources[Source]; ok {
		return false
	}
	d.downloadedResources[Source] = struct{}{}
	return true
}

func (d *DownloadWorker) download(ctx context.Context, Source string, Target string) error {
	klog.V(6).Infof("downloading %s as %s\n", Source, Target)
	// normal read
	repoHost, err := d.registry.Get(Source)
	if err != nil {
		return err
	}
	blob, err := repoHost.Read(ctx, Source)
	if err != nil {
		return err
	}
	if err = d.writer.Write(Target, "", blob, nil); err != nil {
		return err
	}
	return nil
}
