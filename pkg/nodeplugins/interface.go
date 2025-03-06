package nodeplugins

import "github.com/gardener/docforge/pkg/manifest"

// Interface defines the methods node plugins need to implement
type Interface interface {
	// Processor is the name of the node plugin processor
	Processor() string
	// Process is the function that processes a given node
	Process(*manifest.Node) error
}
