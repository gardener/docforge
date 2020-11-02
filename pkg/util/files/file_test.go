// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package files

import (
	"context"
	"fmt"
	"github.com/gardener/docforge/pkg/util/tests"
	"github.com/howeyc/fsnotify"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	// test data directory
	testdata = "testdata"
)

func init() {
	tests.SetKlogV(10)
}

func runFileModify(ctx context.Context, t *testing.T, filePath string, period time.Duration) {
	ticker := time.NewTicker(period)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			{
				var f *os.File
				f, err := os.OpenFile(filePath, os.O_WRONLY, 0600)
				if err != nil {
					t.Errorf("%v", err)
				}
				fi, _ := f.Stat()
				mtime := fmt.Sprintf("%v", fi.ModTime().UTC())
				if _, err = f.WriteString("test " + mtime); err != nil {
					t.Errorf("%v", err)
				}
				f.Close()
			}
		case <-ctx.Done():
			return
		}
	}
}

func TestAddToWatch(t *testing.T) {
	actual := NewFileWatcher()

	err := actual.AddToWatch("/etc/watched0", "/etc/watched1")

	assert.Nil(t, err)
	assert.Equal(t, []string{"/etc/watched0", "/etc/watched1"}, actual.WatchedFiles)
}

func TestWatchDebounce(t *testing.T) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Errorf("%v", err)
		return
	}
	watchedDirPath := filepath.Join(testdata, fmt.Sprintf("dir%d", rand.Intn(1<<32)))
	if err := os.MkdirAll(watchedDirPath, os.ModePerm); err != nil {
		t.Errorf("%v", err)
		return
	}
	defer func() {
		if err := os.RemoveAll(watchedDirPath); err != nil {
			t.Errorf("%v", err)
		}
		watcher.Close()
	}()
	watchedFilePath := filepath.Join(watchedDirPath, fmt.Sprintf("watched%d", rand.Intn(1<<32)))
	var f *os.File
	if f, err = os.Create(watchedFilePath); err != nil {
		t.Errorf("%v", err)
		return
	}
	f.Close()
	wh := &Watcher{
		WatchedFiles: []string{watchedFilePath},
		Watcher:      watcher,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	invocations := 0
	go func() {
		err = wh.Watch(ctx.Done(), func() error {
			invocations++
			return nil
		})
	}()
	// trigger high-rate Modify events and stop before the watch duration - debounce period
	// to give debouncer a chance to invoke the func after its debounce period
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 350*time.Millisecond)
		defer cancel()
		runFileModify(ctx, t, watchedFilePath, 50*time.Millisecond)
		<-ctx.Done()
	}()

	<-ctx.Done()

	assert.Equal(t, 1, invocations)
}

func TestWatchNoWatcher(t *testing.T) {
	wh := &Watcher{
		WatchedFiles: []string{"watched"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	invocations := 0
	err := wh.Watch(ctx.Done(), func() error {
		invocations++
		return nil
	})

	assert.Nil(t, err)
	assert.Equal(t, 0, invocations)
}

func TestWatchNoWatchedFiles(t *testing.T) {
	watcher, err := fsnotify.NewWatcher()
	defer watcher.Close()
	wh := &Watcher{
		WatchedFiles: []string{},
		Watcher:      watcher,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	invocations := 0
	err = wh.Watch(ctx.Done(), func() error {
		invocations++
		return nil
	})

	assert.Nil(t, err)
	assert.Equal(t, 0, invocations)
}
