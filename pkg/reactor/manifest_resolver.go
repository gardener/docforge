// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reactor

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/google/uuid"
	"k8s.io/klog/v2"
)

// ResolveManifest resolves the manifests into buildable model
func ResolveManifest(ctx context.Context, manifest *api.Documentation, rhRegistry resourcehandlers.Registry, manifestAbsPath string) error {
	var (
		structure []*api.Node
		err       error
	)
	if manifest.NodeSelector != nil {
		node, err := resolveNodeSelector(ctx, rhRegistry, &api.Node{NodeSelector: manifest.NodeSelector}, make(map[string]bool), manifest.Links)
		if err != nil {
			return err
		}
		manifest.NodeSelector = nil
		structure = node.Nodes
	}
	if structure == nil {
		structure = manifest.Structure
	} else {
		// TODO: this should be rather merge than append
		structure = append(manifest.Structure, structure...)
	}

	if structure == nil {
		return fmt.Errorf("document structure resolved to nil")
	}

	if err = resolveStructure(ctx, rhRegistry, manifestAbsPath, structure, manifest.Links, make(map[string]bool)); err != nil {
		return err
	}

	manifest.Structure = structure
	return nil
}

func resolveStructure(ctx context.Context, rhRegistry resourcehandlers.Registry, manifestAbsPath string, nodes []*api.Node, globalLinksConfig *api.Links, visited map[string]bool) error {
	for _, node := range nodes {
		node.SetParentsDownwards()
		if len(node.Source) > 0 {
			nodeName, err := resolveNodeName(ctx, node, rhRegistry)
			if err != nil {
				return err
			}
			node.Name = nodeName
		}
		if node.NodeSelector != nil {
			if manifestAbsPath == node.NodeSelector.Path {
				klog.Warningf("circular dependency discovered, the node %s specifies the provided documentation manifest with path %s as a dependency: ", node.Name, manifestAbsPath)
				node.NodeSelector = nil
				continue
			}
			if visited[node.NodeSelector.Path] {
				klog.Warning("circular dependency discovered:", buildCircularDepMessage(visited, node.NodeSelector))
				node.NodeSelector = nil
				continue
			}
			visited[node.NodeSelector.Path] = true
			newNode, err := resolveNodeSelector(ctx, rhRegistry, node, visited, globalLinksConfig)
			if err != nil {
				return err
			}
			if len(newNode.Nodes) > 0 {
				node.Nodes = append(node.Nodes, newNode.Nodes...)
			}
			node.Links = mergeLinks(node.Links, newNode.Links)
			node.NodeSelector = nil
		}
		if len(node.Nodes) > 0 {
			if err := resolveStructure(ctx, rhRegistry, manifestAbsPath, node.Nodes, globalLinksConfig, visited); err != nil {
				return err
			}
			visited = map[string]bool{}
		}
		node.SetParentsDownwards()
	}
	return nil
}

func resolveNodeSelector(ctx context.Context, rhRegistry resourcehandlers.Registry, node *api.Node, visited map[string]bool, globalLinksConfig *api.Links) (*api.Node, error) {
	newNode := &api.Node{}
	handler := rhRegistry.Get(node.NodeSelector.Path)
	if handler == nil {
		return nil, fmt.Errorf("No suitable handler registered for path %s", node.NodeSelector.Path)
	}

	moduleDocumentation, err := handler.ResolveDocumentation(ctx, node.NodeSelector.Path)
	if err != nil {
		err = fmt.Errorf("failed to resolve imported documentation manifest for node %s with path %s: %v", node.Name, node.NodeSelector.Path, err)
		return nil, err
	}

	if moduleDocumentation != nil {
		newNode.Nodes = moduleDocumentation.Structure
		if moduleLinks := moduleDocumentation.Links; moduleLinks != nil {
			globalNode := &api.Node{
				Links: globalLinksConfig,
			}
			pruneModuleLinks(moduleLinks.Rewrites, node, getNodeRewrites)
			pruneModuleLinks(moduleLinks.Rewrites, globalNode, getNodeRewrites)
			if moduleLinks.Downloads != nil {
				pruneModuleLinks(moduleLinks.Downloads.Renames, node, getNodeDownloadsRenamesKeys)
				pruneModuleLinks(moduleLinks.Downloads.Renames, globalNode, getNodeDownloadsRenamesKeys)
				pruneModuleLinks(moduleLinks.Downloads.Scope, node, getNodeDownloadsScopeKeys)
				pruneModuleLinks(moduleLinks.Downloads.Scope, globalNode, getNodeDownloadsScopeKeys)
			}
		}
		newNode.Links = moduleDocumentation.Links
		if moduleDocumentation.NodeSelector != nil {
			if visited[moduleDocumentation.NodeSelector.Path] {
				klog.Warning("circular dependency discovered:", buildCircularDepMessage(visited, moduleDocumentation.NodeSelector))
				moduleDocumentation.NodeSelector = nil
				return newNode, nil
			}
			visited[moduleDocumentation.NodeSelector.Path] = true
			childNode := &api.Node{
				NodeSelector: moduleDocumentation.NodeSelector,
			}
			childNode.SetParent(node)
			res, err := resolveNodeSelector(ctx, rhRegistry, childNode, visited, globalLinksConfig)
			if err != nil {
				return nil, err
			}
			for _, n := range res.Nodes {
				n.SetParent(node)
				n.SetParentsDownwards()
			}
			pruneChildNodesLinks(node, res.Nodes, globalLinksConfig)
			newNode.Nodes = append(newNode.Nodes, res.Nodes...)
		}
		return newNode, nil
	}

	nodes, err := handler.ResolveNodeSelector(ctx, node, node.NodeSelector.ExcludePaths, node.NodeSelector.ExcludeFrontMatter, node.NodeSelector.FrontMatter, node.NodeSelector.Depth)
	if err != nil {
		return nil, err
	}

	newNode.Nodes = nodes
	return newNode, nil
}

func buildCircularDepMessage(visited map[string]bool, nodeSelector *api.NodeSelector) string {
	var circularDependency string
	for path := range visited {
		circularDependency = circularDependency + path + " -> "
	}
	circularDependency = circularDependency + nodeSelector.Path
	return circularDependency
}

func resolveNodeName(ctx context.Context, node *api.Node, rhRegistry resourcehandlers.Registry) (string, error) {
	name := node.Name
	handler := rhRegistry.Get(node.Source)
	if handler == nil {
		return "", fmt.Errorf("No suitable handler registered for URL %s", node.Source)
	}
	if len(node.Name) == 0 {
		name = "$name"
	}
	resourceName, ext := handler.ResourceName(node.Source)
	id := uuid.New().String()
	name = strings.ReplaceAll(name, "$name", resourceName)
	name = strings.ReplaceAll(name, "$uuid", id)
	name = strings.ReplaceAll(name, "$ext", fmt.Sprintf(".%s", ext))
	return name, nil
}

func pruneModuleLinks(moduleLinks interface{}, node *api.Node, getParentLinks func(node *api.Node) map[string]struct{}) {
	v := reflect.ValueOf(moduleLinks)
	if v.Kind() != reflect.Map {
		return
	}

	for _, moduleLinkKey := range v.MapKeys() {
		for parent := node; parent != nil; parent = parent.Parent() {
			if parent.Links == nil {
				continue
			}

			if parentLinks := getParentLinks(parent); parentLinks != nil {
				if _, ok := parentLinks[moduleLinkKey.String()]; ok {
					klog.Warningf("Overriding module link %s", moduleLinkKey.String())
					v.SetMapIndex(moduleLinkKey, reflect.Value{})
				}
			}
		}
	}
}

func getNodeRewrites(node *api.Node) map[string]struct{} {
	if node.Links == nil {
		return nil
	}
	keys := make(map[string]struct{})
	for k := range node.Links.Rewrites {
		keys[k] = struct{}{}
	}
	return keys
}

func getNodeDownloadsRenamesKeys(node *api.Node) map[string]struct{} {
	if node.Links == nil {
		return nil
	}
	if node.Links.Downloads == nil {
		return nil
	}

	keys := make(map[string]struct{})
	for k := range node.Links.Downloads.Renames {
		keys[k] = struct{}{}
	}
	return keys
}

func getNodeDownloadsScopeKeys(node *api.Node) map[string]struct{} {
	if node.Links == nil {
		return nil
	}
	if node.Links.Downloads == nil {
		return nil
	}

	keys := make(map[string]struct{})
	for k := range node.Links.Downloads.Scope {
		keys[k] = struct{}{}
	}
	return keys
}

func pruneChildNodesLinks(parentNode *api.Node, nodes []*api.Node, globalLinksConfig *api.Links) {
	for _, node := range nodes {
		if node.Nodes != nil {
			pruneChildNodesLinks(node, node.Nodes, globalLinksConfig)
		}

		if node.Links != nil {
			globalNode := &api.Node{
				Links: globalLinksConfig,
			}
			pruneModuleLinks(node.Links.Rewrites, parentNode, getNodeRewrites)
			pruneModuleLinks(node.Links.Rewrites, globalNode, getNodeRewrites)
			if node.Links.Downloads != nil {
				pruneModuleLinks(node.Links.Downloads.Renames, parentNode, getNodeDownloadsRenamesKeys)
				pruneModuleLinks(node.Links.Downloads.Renames, globalNode, getNodeDownloadsRenamesKeys)
				pruneModuleLinks(node.Links.Downloads.Scope, parentNode, getNodeDownloadsScopeKeys)
				pruneModuleLinks(node.Links.Downloads.Scope, globalNode, getNodeDownloadsScopeKeys)
			}
		}
	}
}
