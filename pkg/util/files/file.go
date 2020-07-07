package files

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/golang/glog"
	"github.com/howeyc/fsnotify"
)

const (
	watchDebounceDelay = 100 * time.Millisecond
)

// Watcher encapsulates file watch and configuration,
// abstracting form the underlying file watch provider
type Watcher struct {
	Watcher      *fsnotify.Watcher
	WatchedFiles []string
	watched      []string
}

// NewFileWatcher creates Watcher
func NewFileWatcher() *Watcher {
	return &Watcher{
		WatchedFiles: []string{},
		watched:      []string{},
	}
}

// AddToWatch adds files to a WatchedFiles list monitored
// by this webhook's Watcher. The Watcher is created on
// demand if it's nil when the operation is invoked.
func (w *Watcher) AddToWatch(files ...string) error {
	if len(files) == 0 {
		return nil
	}
	if w.Watcher == nil {
		var err error
		w.Watcher, err = fsnotify.NewWatcher()
		if err != nil {
			return err
		}
	}
	w.WatchedFiles = append(w.WatchedFiles, files...)
	return nil
}

// Watch starts monitoring WatchedFiles, until a signal is received on its stop channel. If WatchedFiles
// or Watcher are not initialized, Watch returns immediately. The eventHandler function is invoked upon
// Modify/Create file events raised by the watched files.
func (w *Watcher) Watch(stop <-chan struct{}, eventHandler func() error) error {
	var started bool
	if w.Watcher == nil || w.WatchedFiles == nil || len(w.WatchedFiles) == 0 {
		return nil
	}
	defer func(started bool) {
		w.Watcher.Close() // nolint: errcheck
		if started {
			glog.V(6).Infof("watching files stopped")
		}
	}(started)

	// watch the parent directory of the target files so we can catch
	// symlink updates of k8s ConfigMaps volumes.
	for _, file := range w.WatchedFiles {
		watchDir, _ := filepath.Split(file)
		exists := false
		for _, w := range w.watched {
			if w == watchDir {
				exists = true
				break
			}
		}
		if !exists {
			if err := w.Watcher.Watch(watchDir); err != nil {
				return fmt.Errorf("could not watch %v: %v", file, err)
			}
			glog.V(6).Infof("watching %s", watchDir)
			w.watched = append(w.watched, watchDir)
		}
	}
	started = true
	glog.V(6).Info("watching files started")
	var timerC <-chan time.Time
	for {
		select {
		case <-timerC:
			{
				if eventHandler != nil {
					if err := eventHandler(); err != nil {
						break
					}
				}
			}
		case event := <-w.Watcher.Event:
			{
				// use a timer to debounce configuration updates
				if event.IsModify() || event.IsCreate() {
					timerC = time.After(watchDebounceDelay)
				}
			}
		case err := <-w.Watcher.Error:
			glog.V(10).Infof("watcher error: %v", err)
		case <-stop:
			return nil
		}
	}
}
