package manifestplugins

import "github.com/gardener/docforge/pkg/manifest"

// Interface should be implemented by plugins
type Interface interface {
	// PluginNodeTransformations is the list of node transformations
	// the plugin would like to apply when constructing the manifest
	PluginNodeTransformations() []manifest.NodeTransformation
}
