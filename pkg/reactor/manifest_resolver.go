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
	"github.com/gardener/docforge/pkg/markdown"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	"github.com/yuin/goldmark/ast"
	"k8s.io/klog/v2"
)

// ResolveManifest resolves the root manifests into buildable model
func (r *Reactor) ResolveManifest(ctx context.Context, manifest *api.Documentation) error {
	// init virtual root node
	root := &api.Node{Nodes: manifest.Structure, NodeSelector: manifest.NodeSelector}
	manifest.NodeSelector = nil
	root.SetParentsDownwards()
	// init visited map
	visited := make(map[string]int)
	visited[r.Options.ManifestPath] = 0
	// resolve manifest structure names
	if err := r.resolveNodeNames(root.Nodes); err != nil {
		return err
	}
	// resolve node selector
	if root.NodeSelector != nil {
		node, err := r.resolveNodeSelector(ctx, root, visited)
		if err != nil {
			return err
		}
		root.NodeSelector = nil
		if err = root.Union(node.Nodes); err != nil {
			return err
		}
	}
	// ensure structure is not empty
	if len(root.Nodes) == 0 {
		return fmt.Errorf("document structure is empty")
	}
	// resolve structure
	if err := r.resolveStructure(ctx, root.Nodes, visited); err != nil {
		return err
	}
	// determine section files
	if r.Options.Hugo.Enabled {
		r.resolveSectionFiles(root)
	}
	// set nil parent for root structure nodes
	for _, n := range root.Nodes {
		n.SetParent(nil)
	}
	manifest.Structure = root.Nodes
	return nil
}

// resolveStructure iterates the child nodes in a structure model and resolve:
// - node selector
// - node children recursively
// The resulting model is the actual flight plan for replicating resources.
func (r *Reactor) resolveStructure(ctx context.Context, nodes []*api.Node, visited map[string]int) error {
	for _, node := range nodes {
		// resolve node selector
		if node.NodeSelector != nil {
			if _, ok := visited[node.NodeSelector.Path]; ok {
				return fmt.Errorf("circular dependency discovered: %s", buildCircularDepMessage(visited, node.NodeSelector.Path))
			}
			visited[node.NodeSelector.Path] = len(visited)
			selected, err := r.resolveNodeSelector(ctx, node, visited)
			if err != nil {
				return err
			}
			node.NodeSelector = nil
			if err = node.Union(selected.Nodes); err != nil {
				return err
			}
		}
		// resolve structure
		if err := r.resolveStructure(ctx, node.Nodes, visited); err != nil {
			return err
		}
	}
	return nil
}

func (r *Reactor) resolveNodeSelector(ctx context.Context, node *api.Node, visited map[string]int) (*api.Node, error) {
	// get resource handler
	rh := r.ResourceHandlers.Get(node.NodeSelector.Path)
	if rh == nil {
		return nil, fmt.Errorf("no suitable handler registered for path %s", node.NodeSelector.Path)
	}
	// if path points to manifest then resolve documentation, otherwise nil will be returned
	manifest, err := rh.ResolveDocumentation(ctx, node.NodeSelector.Path)
	if err != nil {
		err = fmt.Errorf("failed to resolve imported documentation manifest for node %s with path %s: %v", node.FullName("/"), node.NodeSelector.Path, err)
		return nil, err
	}
	if manifest != nil {
		result := &api.Node{Nodes: manifest.Structure, NodeSelector: manifest.NodeSelector}
		manifest.NodeSelector = nil
		result.SetParentsDownwards()
		// resolve manifest structure names
		if err = r.resolveNodeNames(result.Nodes); err != nil {
			return nil, err
		}
		if result.NodeSelector != nil {
			if _, ok := visited[result.NodeSelector.Path]; ok {
				return nil, fmt.Errorf("circular dependency discovered %s", buildCircularDepMessage(visited, result.NodeSelector.Path))
			}
			visited[result.NodeSelector.Path] = len(visited)
			selected, e := r.resolveNodeSelector(ctx, result, visited)
			if e != nil {
				return nil, err
			}
			result.NodeSelector = nil
			if err = result.Union(selected.Nodes); err != nil {
				return nil, err
			}
		}
		return result, nil
	}
	// if path points to directory -> resolve node selector
	nodes, err := rh.ResolveNodeSelector(ctx, node)
	if err != nil {
		return nil, err
	}
	result := &api.Node{Nodes: nodes}
	result.SetParentsDownwards()
	// if frontmatter filters defined in node selector
	if len(node.NodeSelector.ExcludeFrontMatter) > 0 || len(node.NodeSelector.FrontMatter) > 0 {
		err = filterDocuments(ctx, rh, result, node.NodeSelector.ExcludeFrontMatter, node.NodeSelector.FrontMatter)
		if err != nil {
			return nil, err
		}
		// cleanup after filtering
		result.Cleanup()
	}
	return result, nil
}

// resolveNodeNames is applicable to explicitly defined nodes, it will evaluate Node name expression if such is defined
// and in hugo mode it will rename nodes that have property 'index=true' to _index.md
// resolve should happen before merging with other nodes
func (r *Reactor) resolveNodeNames(nodes []*api.Node) error {
	for _, node := range nodes {
		if node.IsDocument() {
			// evaluate name expression
			if len(node.Source) > 0 {
				name := node.Name
				if len(name) == 0 {
					name = "$name"
				}
				if strings.IndexByte(name, '$') != -1 {
					handler := r.ResourceHandlers.Get(node.Source)
					if handler == nil {
						return fmt.Errorf("no suitable handler registered for URL %s", node.Source)
					}
					resourceName, ext := handler.ResourceName(node.Source)
					if len(ext) > 0 {
						name += "$ext"
					}
					name = strings.ReplaceAll(name, "$name", resourceName)
					name = strings.ReplaceAll(name, "$uuid", uuid.New().String())
					name = strings.ReplaceAll(name, "$ext", fmt.Sprintf(".%s", ext))
					// set evaluated
					node.Name = name
				}
			}
			// check 'index=true'
			if r.Options.Hugo.Enabled && len(node.Properties) > 0 {
				if idxVal, found := node.Properties["index"]; found {
					idx, ok := idxVal.(bool)
					if ok && idx {
						node.Name = "_index.md"
					}
				}
			}
			// ensure markdown suffix
			if !strings.HasSuffix(node.Name, ".md") {
				node.Name = fmt.Sprintf("%s.md", node.Name)
			}
		} else {
			if err := r.resolveNodeNames(node.Nodes); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *Reactor) resolveSectionFiles(container *api.Node) {
	var hasSectionFile bool
	for _, node := range container.Nodes {
		if node.IsDocument() && node.Name == "_index.md" {
			hasSectionFile = true
			break
		}
	}
	if !hasSectionFile && len(r.Options.Hugo.IndexFileNames) > 0 {
		// try to find one, priority is the IndexFileNames order
		for _, ifn := range r.Options.Hugo.IndexFileNames {
			for _, node := range container.Nodes {
				if node.IsDocument() && strings.EqualFold(node.Name, ifn) {
					klog.V(6).Infof("renaming %s -> _index.md\n", node.FullName("/"))
					node.Name = "_index.md"
					break
				}
			}
		}
	}
	for _, node := range container.Nodes {
		if !node.IsDocument() {
			r.resolveSectionFiles(node)
		}
	}
}

// buildCircularDepMessage creates a circular dependency cycle, returns empty string if no cycle found
func buildCircularDepMessage(visited map[string]int, path string) string {
	// build visited order
	ordered := make([]string, len(visited)+1)
	for k, v := range visited {
		ordered[v] = k
	}
	// add path at the end
	ordered[len(visited)] = path
	for i, v := range ordered {
		if v == path { //
			return strings.Join(ordered[i:], " -> ")
		}
	}
	// no cycle
	return ""
}

// TODO: on err just continue ... ?? only warning messages or exclude nodes with errors ???
// filterDocuments is applicable only to nodes returned from nodeSelector that points to directory
// all document nodes should have a source property
func filterDocuments(ctx context.Context, rh resourcehandlers.ResourceHandler, node *api.Node, exclude map[string]interface{}, include map[string]interface{}) error {
	var errs error
	forExclusion := make(map[string]bool)
	for _, n := range node.Nodes {
		if n.IsDocument() {
			cnt, err := rh.Read(ctx, n.Source)
			if err != nil {
				if resourceNotFound, ok := err.(resourcehandlers.ErrResourceNotFound); ok {
					klog.Warningf("reading source %s from node %s failed: %s\n", n.Source, n.FullName("/"), resourceNotFound)
				} else {
					errs = multierror.Append(errs, fmt.Errorf("reading source %s from node %s failed: %w", n.Source, n.FullName("/"), err))
					continue
				}
			}
			excl := len(include) > 0 // if include filters defined, initially node should be marked for exclusion
			if len(cnt) > 0 {
				dc := &docContent{docCnt: cnt, docURI: n.Source}
				dc.docAst, err = markdown.Parse(cnt)
				if err != nil {
					errs = multierror.Append(errs, fmt.Errorf("fail to parse source %s from node %s: %w", n.Source, n.FullName("/"), err))
					continue
				}
				// cache the content in node properties map
				if n.Properties == nil {
					n.Properties = make(map[string]interface{})
					n.Properties[api.CachedNodeContent] = dc
				}
				if dc.docAst.Kind() == ast.KindDocument {
					doc := dc.docAst.(*ast.Document)
					fm := doc.Meta()
					if len(fm) > 0 {
						for path, v := range include {
							if matchFrontMatterRule(path, v, fm) {
								excl = false // i.e. include
								break
							}
						}
						if !excl { // perform exclude only if not already excluded
							for path, v := range exclude {
								if matchFrontMatterRule(path, v, fm) {
									excl = true
									break
								}
							}
						}
					}
				}
			} else if err == nil {
				klog.Warningf("no content read from node %s source %s\n", n.FullName("/"), n.Source)
			}
			if excl {
				forExclusion[n.Name] = true
			}
		} else {
			if err := filterDocuments(ctx, rh, n, exclude, include); err != nil {
				errs = multierror.Append(errs, err)
			}
		}
	}
	if len(forExclusion) > 0 {
		var filtered []*api.Node
		for _, n := range node.Nodes {
			if forExclusion[n.Name] && n.IsDocument() {
				continue
			}
			filtered = append(filtered, n)
		}
		node.Nodes = filtered
	}
	return errs
}

// matchFrontMatterRule explores a parsed frontmatter object `data` to matchRule
// `value` at `path` pattern and return true on successfully matchRule or false
// otherwise.
// Path is an expression with a JSONPath-like simplified notation.
// An object in path is modeled as dot (`.`). Paths start with the root object,
// i.e. the most minimal path is `.`.
// An object element value is referenced by its name (key) in the object map:
// `.a.b.c` is path to element `c` in map `b` in map `a` in root object map.
// Element values can be scalar, object maps or arrays.
// An element in an array is referenced by its index: `.a.b[1]` references `b`
// array element with index 1.
// Paths can include up to one wildcard `**` symbol that models *any* path node.
// A `.a.**.c` models any path starting with	`.a.` and ending with `.c`.
func matchFrontMatterRule(path string, val interface{}, data interface{}) bool {
	return matchRule(path, val, nil, data)
}

func matchRule(pathPattern string, val interface{}, path []string, data interface{}) bool {
	if path == nil {
		path = []string{"."}
	}
	p := strings.Join(path, "")
	if matchPath(pathPattern, p) {
		if reflect.DeepEqual(val, data) {
			return true
		}
	}
	switch dt := data.(type) {
	case []interface{}:
		for i, u := range dt {
			_p := append(path, fmt.Sprintf("[%d]", i))
			if ok := matchRule(pathPattern, val, _p, u); ok {
				return true
			}
		}
	case map[string]interface{}:
		for k, u := range dt {
			if path[(len(path))-1] != "." {
				path = append(path, ".")
			}
			_p := append(path, k)
			if ok := matchRule(pathPattern, val, _p, u); ok {
				return true
			}
		}
	}
	return false
}

func matchPath(pathPattern, path string) bool {
	if pathPattern == path {
		return true
	}
	s := strings.Split(pathPattern, "**")
	if len(s) == 2 {
		return strings.HasPrefix(path, s[0]) && strings.HasSuffix(path, s[1])
	}
	return false
}
