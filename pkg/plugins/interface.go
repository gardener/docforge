// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package plugins

import (
	"github.com/gardener/docforge/pkg/core/manifest"
)

// Status represents processing status with error and external links
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

// Interface defines the unified plugin interface that combines both
// manifest transformation and node processing capabilities
type Interface interface {
	// Name returns the plugin name for identification
	Name() string

	// ManifestTransformations returns transformations to apply during manifest parsing
	// Return empty slice if plugin doesn't need manifest transformations
	ManifestTransformations() []manifest.NodeTransformation

	// FinalNodeStructure is called after manifest resolution with the final document structure
	// This allows plugins to prepare for node processing with knowledge of the complete structure
	// Return error if plugin cannot initialize with the given structure
	FinalNodeStructure(documentNodes []*manifest.Node) error

	// Processor returns the processor name for node processing
	// Return empty string if plugin doesn't process nodes
	Processor() string

	// Process processes a node using the old synchronous method
	// Return nil if plugin doesn't use legacy processing
	Process(*manifest.Node) error

	// ProcessNew processes a node using the new channel-based method
	// Return empty slice if plugin doesn't use channel processing
	ProcessNew(*manifest.Node) []chan Status
}
