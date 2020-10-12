package writers

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"sort"
	"strings"
	"time"

	"github.com/gardener/docforge/pkg/api"
)

// DryRunWriter is the functional interface for working
// with dry run writers
type DryRunWriter interface {
	// GetWriter creates DryRunWriters writing to the
	// same backend but for different roots (e.g. for
	// resources and docs)
	GetWriter(root string) Writer
	// Flush wraps up dryrun writing and flushes
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
	path  string
	stats []*api.Stat
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

func (w *writer) Write(name, path string, docBlob []byte, node *api.Node) error {
	if len(docBlob) > 0 && node != nil && !strings.HasSuffix(name, ".md") {
		name = fmt.Sprintf("%s.md", name)
	}
	f := &file{
		path:  fmt.Sprintf("%s/%s/%s", w.root, path, name),
		stats: node.GetStats(),
	}
	*w.files = append(*w.files, f)
	return nil
}

// Flush formats and writes the dry-run result to the
// underlying writer
func (d *dryRunWriter) Flush() bool {
	var (
		b     bytes.Buffer
		bytes []byte
		err   error
	)

	sort.Slice(d.files, func(i, j int) bool { return d.files[i].path < d.files[j].path })
	format(d.files, &b)

	elapsedTime := time.Since(d.t1)
	b.WriteString(fmt.Sprintf("\nBuild finished in %f seconds\n", elapsedTime.Seconds()))

	if bytes, err = ioutil.ReadAll(&b); err != nil {
		fmt.Println(err.Error())
		return false
	}
	if _, err := d.Writer.Write(bytes); err != nil {
		fmt.Println(err.Error())
	}
	return true
}

func format(files []*file, b *bytes.Buffer) {
	all := []string{}
	for _, f := range files {
		p := f.path
		dd := strings.Split(p, "/")
		indent := 0
		for i, s := range dd {
			if i > 0 {
				b.Write([]byte("  "))
				indent++
			}
			idx := i + 1
			_p := strings.Join(dd[:idx], "/")
			if !any(all, _p) {
				all = append(all, _p)
				b.WriteString(fmt.Sprintf("%s\n", s))
				for _, st := range f.stats {
					b.Write([]byte("  "))
					b.Write(bytes.Repeat([]byte("  "), indent))
					b.WriteString(fmt.Sprintf("%s stats: %s\n", st.Title, st.Figures))
					for _, detail := range st.Details {
						b.Write(bytes.Repeat([]byte("  "), indent+2))
						b.WriteString(fmt.Sprintf("%s\n", detail))
					}
				}
			}
		}
	}
}

func any(s []string, str string) bool {
	for _, s := range s {
		if s == str {
			return true
		}
	}
	return false
}
