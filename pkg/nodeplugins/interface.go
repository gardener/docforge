package nodeplugins

import "github.com/gardener/docforge/pkg/manifest"

// Status is status
type Status struct {
	err           error
	externalLinks []manifest.ExternalLink
}

// NewStatus creates a new Status with the given error
func NewStatus(err error) Status {
	return Status{err: err, externalLinks: nil}
}

// NewStatusWithLinks creates a new Status with error and external links
func NewStatusWithLinks(err error, links []manifest.ExternalLink) Status {
	return Status{err: err, externalLinks: links}
}

// Error returns the error from the status
func (s Status) Error() error {
	return s.err
}

// ExternalLinks returns the external links collected during processing
func (s Status) ExternalLinks() []manifest.ExternalLink {
	return s.externalLinks
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
