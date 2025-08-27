// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package plugins

import (
	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/nodeplugins"
)

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
	ProcessNew(*manifest.Node) []chan nodeplugins.Status
}
