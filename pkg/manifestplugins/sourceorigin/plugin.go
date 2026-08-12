// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package sourceorigin

import (
	"fmt"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/registry"
)

// SourceOrigin is the manifest plugin that writes origin frontmatter to every
// processed file node. By default it writes:
//
//	origin: remote   (source resolved by a remote GitHub host)
//	origin: local    (source resolved by a local resourceMappings host)
//
// When GardenerMapping is true it instead writes:
//
//	managed: true   (remote)
//	local: true     (local)
type SourceOrigin struct {
	GardenerMapping bool
}

// PluginNodeTransformations returns the node transformations for this plugin.
func (s *SourceOrigin) PluginNodeTransformations() []manifest.NodeTransformation {
	return []manifest.NodeTransformation{s.annotateOrigin}
}

func (s *SourceOrigin) annotateOrigin(node *manifest.Node, _ *manifest.Node, r registry.Interface) (bool, error) {
	if node.Type != "file" {
		return false, nil
	}
	// Nodes without a source (e.g. synthetic _index.md without source) have no
	// origin to query.
	if node.Source == "" && len(node.MultiSource) == 0 {
		return false, nil
	}

	src := node.Source
	if src == "" {
		src = node.MultiSource[0]
	}

	remote, err := r.IsRemote(src)
	if err != nil {
		return false, fmt.Errorf("node %s: IsRemote: %w", node, err)
	}

	if node.Frontmatter == nil {
		node.Frontmatter = map[string]interface{}{}
	}

	if s.GardenerMapping {
		if remote {
			node.Frontmatter["managed"] = true
		} else {
			node.Frontmatter["local"] = true
		}
	} else {
		if remote {
			node.Frontmatter["origin"] = "remote"
		} else {
			node.Frontmatter["origin"] = "local"
		}
	}

	return false, nil
}
