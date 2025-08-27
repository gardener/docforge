// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package plugins

import (
	"fmt"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/nodeplugins"
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
		if err := plugin.FinalNodeStructure(documentNodes); err != nil {
			return fmt.Errorf("plugin %s failed to initialize with final node structure: %w", plugin.Name(), err)
		}
	}
	return nil
}

// GetNodeProcessors returns node processor interfaces for plugins that process nodes
func (r *Registry) GetNodeProcessors() []nodeplugins.Interface {
	var processors []nodeplugins.Interface

	for _, plugin := range r.plugins {
		// Only include plugins that have a processor
		if plugin.Processor() != "" {
			processors = append(processors, &nodePluginAdapter{plugin})
		}
	}

	return processors
}

// nodePluginAdapter adapts a unified plugin to the nodeplugins.Interface
type nodePluginAdapter struct {
	plugin Interface
}

func (a *nodePluginAdapter) Processor() string {
	return a.plugin.Processor()
}

func (a *nodePluginAdapter) Process(node *manifest.Node) error {
	return a.plugin.Process(node)
}

func (a *nodePluginAdapter) ProcessNew(node *manifest.Node) []chan nodeplugins.Status {
	return a.plugin.ProcessNew(node)
}
