// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"sort"

	"github.com/gardener/docforge/pkg/registry"
	"k8s.io/klog/v2"
)

// weightOf extracts the numeric weight from a node's frontmatter.
// Returns (value, true) for int or float64 weight values; (0, false) otherwise.
func weightOf(n *Node) (int, bool) {
	if n.Frontmatter == nil {
		return 0, false
	}
	raw, ok := n.Frontmatter["weight"]
	if !ok || raw == nil {
		return 0, false
	}
	switch v := raw.(type) {
	case int:
		return v, true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

// resolveOrder sorts node.Structure so that children with a weight frontmatter
// field come first in ascending order, followed by unweighted children in their
// original manifest order. Equal weights preserve manifest order (stable sort).
func resolveOrder(node *Node, _ *Node, _ registry.Interface) (bool, error) {
	if len(node.Structure) < 2 {
		return false, nil
	}

	hasWeight := false
	hasNoWeight := false
	for _, child := range node.Structure {
		if _, ok := weightOf(child); ok {
			hasWeight = true
		} else {
			hasNoWeight = true
		}
	}
	if hasWeight && hasNoWeight {
		klog.Warningf("mixed weight ordering at %s: some children have 'weight' frontmatter and some do not", node.NodePath())
	}

	if !hasWeight {
		return false, nil
	}

	sort.SliceStable(node.Structure, func(i, j int) bool {
		wi, iHasWeight := weightOf(node.Structure[i])
		wj, jHasWeight := weightOf(node.Structure[j])
		if iHasWeight && jHasWeight {
			return wi < wj
		}
		// weighted nodes sort before unweighted
		return iHasWeight
	})

	return false, nil
}
