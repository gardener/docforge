package downloader

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/gardener/docforge/pkg/readers"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/writers"
	"k8s.io/klog/v2"
)

type DownloadWorker struct {
	reader readers.Reader
	writer writers.Writer
	// lock for accessing the downloadedResources map
	mux sync.Mutex
	// map with downloaded resources
	downloadedResources map[string]struct{}
}

func NewDownloader(reader readers.Reader, writer writers.Writer) (*DownloadWorker, error) {
	if reader == nil || reflect.ValueOf(reader).IsNil() {
		return nil, errors.New("invalid argument: reader is nil")
	}
	if writer == nil || reflect.ValueOf(writer).IsNil() {
		return nil, errors.New("invalid argument: writer is nil")
	}
	return &DownloadWorker{
		reader:              reader,
		writer:              writer,
		downloadedResources: make(map[string]struct{}),
	}, nil
}

func (d *DownloadWorker) Download(ctx context.Context, Source string, Target string, Referer string, Reference string) error {
	if !d.shouldDownload(Source) {
		return nil
	}
	if err := d.download(ctx, Source, Target); err != nil {
		dErr := fmt.Errorf("downloading %s as %s and reference %s from referer %s failed: %v", Source, Target, Reference, Referer, err)
		if _, ok := err.(resourcehandlers.ErrResourceNotFound); ok {
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
	blob, err := d.reader.Read(ctx, Source)
	if err != nil {
		return err
	}
	if err = d.writer.Write(Target, "", blob, nil); err != nil {
		return err
	}
	return nil
}
