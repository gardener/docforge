package manifestplugins

import "github.com/gardener/docforge/pkg/manifest"

// Interface should be implemented by plugins
type Interface interface {
	// PluginNodeTransformations is the list of node transformations
	// the plugin would like to apply when constructing the manifest.
	// Also plugins can provide a no-op transformation to get part of
	// the node tree that will be needed when processing a node on a
	// later stage
	PluginNodeTransformations() []manifest.NodeTransformation
}
