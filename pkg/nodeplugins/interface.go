package nodeplugins

import "github.com/gardener/docforge/pkg/manifest"

// Status is status
type Status struct {
	err error
}

// NewStatus creates a new Status with the given error
func NewStatus(err error) Status {
	return Status{err: err}
}

// Error returns the error from the status
func (s Status) Error() error {
	return s.err
}

// Interface defines the methods node plugins need to implement
type Interface interface {
	// Processor is the name of the node plugin processor
	Processor() string
	// Process is the function that processes a given node
	Process(*manifest.Node) error

	// Process is the function that processes a given node
	ProcessNew(*manifest.Node) []chan Status
}
