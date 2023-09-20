// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package writers

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gardener/docforge/pkg/manifest"
)

// DryRunWriter is the functional interface for working
// with dry run writers
type DryRunWriter interface {
	// GetWriter creates DryRunWriters writing to the
	// same backend but for different roots (e.g. for
	// resources and docs)
	GetWriter(root string) Writer
	// Flush wraps up dry run writing and flushes
	// results to the underlying writer (e.g. os.Stdout)
	Flush() bool
}

type dryRunWriter struct {
	Writer  io.Writer
	writers []*writer
	files   []*file
	t1      time.Time
}

type file struct {
	path string
}

type writer struct {
	root  string
	files *[]*file
}

// NewDryRunWritersFactory creates factory for DryRunWriters
// writing to the same backend but for different roots (e.g. for
// resources and docs)
func NewDryRunWritersFactory(w io.Writer) DryRunWriter {
	return &dryRunWriter{
		Writer:  w,
		writers: []*writer{},
		files:   []*file{},
		t1:      time.Now(),
	}
}

func (d *dryRunWriter) GetWriter(root string) Writer {
	_w := &writer{
		root:  root,
		files: &d.files,
	}
	if d.writers == nil {
		d.writers = []*writer{_w}
		return _w
	}
	d.writers = append(d.writers, _w)
	return _w
}

func (w *writer) Write(name, path string, docBlob []byte, node *manifest.Node) error {
	if len(docBlob) > 0 && node != nil {
		if !strings.HasSuffix(name, ".md") {
			name = fmt.Sprintf("%s.md", name)
		}
	}
	root := filepath.Clean(w.root)
	path = filepath.Clean(path)
	filePath := fmt.Sprintf("%s/%s/%s", root, path, name)
	filePath = filepath.Clean(filePath)
	f := &file{
		path: filePath,
	}
	*w.files = append(*w.files, f)
	return nil
}

// Flush formats and writes the dry-run result to the
// underlying writer
func (d *dryRunWriter) Flush() bool {
	var (
		buf bytes.Buffer
		b   []byte
		err error
	)

	sort.Slice(d.files, func(i, j int) bool { return d.files[i].path < d.files[j].path })
	format(d.files, &buf)

	elapsedTime := time.Since(d.t1)
	buf.WriteString(fmt.Sprintf("\nBuild finished in %f seconds\n", elapsedTime.Seconds()))

	if b, err = ioutil.ReadAll(&buf); err != nil {
		fmt.Println(err.Error())
		return false
	}
	if _, err := d.Writer.Write(b); err != nil {
		fmt.Println(err.Error())
	}
	return true
}

func format(files []*file, b *bytes.Buffer) {
	var all []string
	for _, f := range files {
		p := f.path
		p = filepath.Clean(p)
		dd := strings.Split(p, string(filepath.Separator))
		indent := 0
		for i, s := range dd {
			if i > 0 {
				b.Write([]byte("  "))
				indent++
			}
			idx := i + 1
			_p := filepath.Join(dd[:idx]...)
			if !any(all, _p) {
				all = append(all, _p)
				b.WriteString(fmt.Sprintf("%s\n", s))
				if i < len(dd)-1 {
					b.Write(bytes.Repeat([]byte("  "), i))
					continue
				}
			}
		}
	}
}

func any(all []string, str string) bool {
	for _, s := range all {
		if s == str {
			return true
		}
	}
	return false
}
