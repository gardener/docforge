package reactor

import (
	"context"
	"fmt"
	"sync"

	"github.com/gardener/docode/pkg/writers"
)

// DownloadTask holds information for source and target of linked document resources
type DownloadTask struct {
	Source string
	Target string
}

type worker interface {
	download(ctx context.Context, dt *DownloadTask) error
}

type DownloadJob interface {
	Start(ctx context.Context, errCh chan error, shutdownCh chan struct{}, wg *sync.WaitGroup)
	Schedule(ctx context.Context, link, resourceName string)
}

type ResourceDownloadJob struct {
	worker
	downloadCh          chan *DownloadTask
	failFast            bool
	workersCount        int
	rwlock              sync.RWMutex
	downloadedResources map[string]struct{}
}

type downloadWorker struct {
	writers.Writer
	Reader
}

func NewResourceDownloadJob(reader Reader, writer writers.Writer, workersCount int, failFast bool) DownloadJob {
	if reader == nil {
		reader = &GenericReader{}
	}
	if writer == nil {
		panic(fmt.Sprint("Invalid argument: writer is nil"))
		//writer = &writers.FSWriter{}
	}
	downloadCh := make(chan *DownloadTask)
	return &ResourceDownloadJob{
		worker: &downloadWorker{
			Reader: reader,
			Writer: writer,
		},
		downloadCh:          downloadCh,
		failFast:            failFast,
		workersCount:        workersCount,
		downloadedResources: make(map[string]struct{}),
	}
}

// Starts the job with multiple workers, each waiting for download tasks or context termination
func (l *ResourceDownloadJob) Start(ctx context.Context, errCh chan error, shutdownCh chan struct{}, jobWg *sync.WaitGroup) {
	if l.workersCount < 1 {
		panic(fmt.Sprintf("Invalid argument: expected workersCount > 1, was %d", l.workersCount))
	}
	jobWg.Add(1)
	go func() {
		var (
			shutdownChs = []chan struct{}{}
		)
		for i := 0; i < l.workersCount; i++ {
			go func() {
				workerShutdownCh := make(chan struct{})
				shutdownChs = append(shutdownChs, workerShutdownCh)
				l.start(ctx, errCh, workerShutdownCh)
			}()
		}
		fmt.Printf("%d resource download workers started \n", l.workersCount)
		<-shutdownCh
		for _, ch := range shutdownChs {
			ch <- struct{}{}
		}
		jobWg.Done()
	}()

	// select {
	// case <-ctx.Done():
	// 	{
	// 		fmt.Printf("Downloaded resources: %d\n", len(l.downloadedResources))
	// 		return
	// 	}
	// }
}

// worker func
func (l *ResourceDownloadJob) start(ctx context.Context, errCh chan error, shutdownCh chan struct{}) {
	var halt bool
	for {
		select {
		case task, ok := <-l.downloadCh:
			{
				if !ok {
					return
				}
				if !l.isDownloaded(task) {
					if err := l.worker.download(ctx, task); err != nil {
						fmt.Printf("%v\n", err)
						errCh <- err
						break
					}
					l.setDownloaded(task)
				}
			}
		case <-ctx.Done():
			{
				halt = true
			}
		case <-shutdownCh:
			{
				halt = true
			}
		}
		// check if we can shutdown gracefully, i.e.
		// exit when the queue is empty and no new input
		// is expected
		if halt && len(l.downloadCh) < 1 {
			return
		}
	}
}

func (d *downloadWorker) download(ctx context.Context, dt *DownloadTask) error {
	fmt.Printf("Downloading %s as %s\n", dt.Source, dt.Target)
	blob, err := d.Reader.Read(ctx, dt.Source)
	if err != nil {
		return err
	}

	if err := d.Writer.Write(dt.Target, "", blob); err != nil {
		return err
	}
	return nil
}

func (l *ResourceDownloadJob) isDownloaded(dt *DownloadTask) bool {
	l.rwlock.Lock()
	defer l.rwlock.Unlock()
	_, ok := l.downloadedResources[dt.Source]
	return ok
}

func (l *ResourceDownloadJob) setDownloaded(dt *DownloadTask) {
	l.rwlock.Lock()
	defer l.rwlock.Unlock()
	l.downloadedResources[dt.Source] = struct{}{}
}

func (l *ResourceDownloadJob) Schedule(ctx context.Context, link, resourceName string) {
	go func() {
		task := &DownloadTask{
			Source: link,
			Target: resourceName,
		}
		select {
		case l.downloadCh <- task:
			return
		case <-ctx.Done():
			return
		}
	}()
}
