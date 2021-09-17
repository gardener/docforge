// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0
package reactor

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/google/uuid"
	"k8s.io/klog/v2"
)

// ResolveManifest resolves the manifests into buildable model
func ResolveManifest(ctx context.Context, manifest *api.Documentation, rhRegistry resourcehandlers.Registry, manifestAbsPath string, indexFileNames []string) error {
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
		node := &api.Node{Nodes: manifest.Structure}
		// merge structure with node selector result
		if err = node.Union(structure); err != nil {
			return err
		}
		structure = node.Nodes
	}

	if len(structure) == 0 {
		return fmt.Errorf("document structure is empty")
	}

	if err = resolveStructure(ctx, rhRegistry, manifestAbsPath, structure, manifest.Links, make(map[string]bool), indexFileNames); err != nil {
		return err
	}

	manifest.Structure = structure
	return nil
}

// ResolveStructure resolves the following in a structure model:
// - Node name variables
// - NodeSelectors
// The resulting model is the actual flight plan for replicating resources.
func resolveStructure(ctx context.Context, rhRegistry resourcehandlers.Registry, manifestAbsPath string, nodes []*api.Node, globalLinksConfig *api.Links, visited map[string]bool, indexFileNames []string) error {
	for _, node := range nodes {
		node.SetParentsDownwards()
		if len(node.Source) > 0 {
			nodeName, err := resolveNodeName(ctx, rhRegistry, node, indexFileNames)
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
				if node.Parent() != nil {
					if err = node.Union(newNode.Nodes); err != nil {
						return err
					}
					node.Parent().Nodes = node.Nodes
				} else {
					if err = node.Union(newNode.Nodes); err != nil {
						return err
					}
				}
			}

			node.Links = mergeLinks(node.Links, newNode.Links)
			node.NodeSelector = nil
		}
		if len(node.Nodes) > 0 {
			if err := resolveStructure(ctx, rhRegistry, manifestAbsPath, node.Nodes, globalLinksConfig, visited, indexFileNames); err != nil {
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
		return nil, fmt.Errorf("no suitable handler registered for path %s", node.NodeSelector.Path)
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
			if err = newNode.Union(res.Nodes); err != nil {
				return nil, err
			}
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

// resolveNodeName will rename nodes that has property 'index=true' or match the list in reactor#Reactor.IndexFileNames to _index.md on first match, first
// renamed basis to serve as section files.
func resolveNodeName(ctx context.Context, rhRegistry resourcehandlers.Registry, node *api.Node, indexFileNames []string) (string, error) {
	if len(node.Source) == 0 {
		return "", fmt.Errorf("node source not defined for %v", node)
	}
	name := node.Name
	handler := rhRegistry.Get(node.Source)
	if handler == nil {
		return "", fmt.Errorf("no suitable handler registered for URL %s", node.Source)
	}
	resourceName, ext := handler.ResourceName(node.Source)
	if len(node.Name) == 0 {
		name = "$name"
		if len(ext) > 0 {
			name = "$name$ext"
		}
	}
	id := uuid.New().String()
	name = strings.ReplaceAll(name, "$name", resourceName)
	name = strings.ReplaceAll(name, "$uuid", id)
	name = strings.ReplaceAll(name, "$ext", fmt.Sprintf(".%s", ext))
	// --- rename logic from hugo#fswriter ---
	// validate
	if node.Parent() != nil {
		if ns := getIndexNodes(node.Parent().Nodes); len(ns) > 1 {
			names := []string{}
			for _, n := range ns {
				names = append(names, n.Name)
			}
			p := api.Path(node, "/")
			return "", fmt.Errorf("multiple peer nodes with property index: true detected in %s: %s", p, strings.Join(names, ","))
		}
	}

	if hasIndexNode([]*api.Node{node}) {
		name = "_index.md"
	}
	// if IndexFileNames has values and index file has not been
	// identified, try to figure out index file out from node names.
	peerNodes := node.Peers()
	if len(indexFileNames) > 0 && name != "_index" && name != "_index.md" && !hasIndexNode(peerNodes) {
		for _, s := range indexFileNames {
			if strings.EqualFold(name, s) {
				klog.V(6).Infof("Renaming %s -> _index.md\n", filepath.Join(api.Path(node, "/"), name))
				name = "_index.md"
				break
			}
		}
	}
	// --- rename logic from writers#fswriter ---
	if !strings.HasSuffix(name, ".md") {
		name = fmt.Sprintf("%s.md", name)
	}
	return name, nil
}

func hasIndexNode(nodes []*api.Node) bool {
	for _, n := range nodes {
		if n.Properties != nil {
			index := n.Properties["index"]
			if isIndex, ok := index.(bool); ok {
				return isIndex
			}
			if n.Name == "_index" || n.Name == "_index.md" {
				return true
			}
		}
	}
	return false
}

func getIndexNodes(nodes []*api.Node) []*api.Node {
	indexNodes := []*api.Node{}
	for _, n := range nodes {
		if n.Properties != nil {
			index := n.Properties["index"]
			if isIndex, ok := index.(bool); ok {
				if isIndex {
					indexNodes = append(indexNodes, n)
				}
			}
		}
	}
	return indexNodes
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
