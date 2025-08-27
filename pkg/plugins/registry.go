// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package plugins

import (
	"fmt"

	"github.com/gardener/docforge/pkg/core/manifest"
)

// Registry manages the collection of unified plugins
type Registry struct {
	plugins []Interface
}

// NewRegistry creates a new plugin registry
func NewRegistry() *Registry {
	return &Registry{
		plugins: make([]Interface, 0),
	}
}

// Register adds a plugin to the registry
func (r *Registry) Register(plugin Interface) {
	r.plugins = append(r.plugins, plugin)
}

// GetManifestTransformations returns all manifest transformations from registered plugins
func (r *Registry) GetManifestTransformations() []manifest.NodeTransformation {
	var transformations []manifest.NodeTransformation

	for _, plugin := range r.plugins {
		transformations = append(transformations, plugin.ManifestTransformations()...)
	}

	return transformations
}

// SetFinalNodeStructure calls FinalNodeStructure on all registered plugins
// This should be called after manifest resolution but before node processing
func (r *Registry) SetFinalNodeStructure(documentNodes []*manifest.Node) error {
	for _, plugin := range r.plugins {
		// TODO: pass deep copy to prevent race conditions
		if err := plugin.FinalNodeStructure(documentNodes); err != nil {
			return fmt.Errorf("plugin %s failed to initialize with final node structure: %w", plugin.Name(), err)
		}
	}
	return nil
}

// GetNodeProcessors returns unified plugins that have node processing capabilities
func (r *Registry) GetNodeProcessors() []Interface {
	var processors []Interface

	for _, plugin := range r.plugins {
		// Only include plugins that have a processor
		if plugin.Processor() != "" {
			processors = append(processors, plugin)
		}
	}

	return processors
}
