package hugo

import (
	"github.com/gardener/docforge/pkg/processors"
	"github.com/gardener/docforge/pkg/writers"
)

// Options is the configuration options for creating Hugo implementations
// docforge interfaces
type Options struct {
	// PrettyUrls indicates if links will rewritten for Hugo will be
	// formatted for pretty url support or not. Pretty urls in Hugo
	// place built source content in index.html, which resides in a path segment with
	// the name of the file, making request URLs more resource-oriented.
	// Example: (source) sample.md -> (build) sample/index.html -> (runtime) ./sample
	PrettyUrls bool
	// IndexFileNames defines a list of file names that indicate
	// their content can be used as Hugo section files (_index.md).
	IndexFileNames []string
	// Writer is the underlying writer used by hugo#FSWriter to serialize
	// content
	Writer writers.Writer
}

// NewWriter creates a new Hugo Writer implementing writers#Writer
func NewWriter(opts *Options) writers.Writer {
	return &FSWriter{
		opts.Writer,
		opts.IndexFileNames,
	}
}

// NewProcessor creates a new Hugo WriProcess implementing processors#Processor
func NewProcessor(opts *Options) processors.Processor {
	return &Processor{
		opts.PrettyUrls,
		opts.IndexFileNames,
	}
}
