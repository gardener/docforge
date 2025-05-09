// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"slices"
	"strings"

	"github.com/gardener/docforge/pkg/internal/link"
	"github.com/gardener/docforge/pkg/internal/must"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	"gopkg.in/yaml.v2"
)

const sectionFile = "_index.md"

// NodeTransformation is the way plugins can contribute to the node tree processing
type NodeTransformation func(node *Node, parent *Node, r registry.Interface) (runTreeChangeProcedure bool, err error)

type manifestToNodeTreeTransfromation func(node *Node, parent *Node, manifest *Node, r registry.Interface) error

func manifestToNodeTree(manifest *Node, r registry.Interface, functions ...manifestToNodeTreeTransfromation) error {
	for i := range functions {
		if err := processManifestToNodeTreeTransfromation(functions[i], manifest, nil, manifest, r); err != nil {
			return err
		}
	}
	return nil
}

func processManifestToNodeTreeTransfromation(f manifestToNodeTreeTransfromation, node *Node, parent *Node, manifest *Node, r registry.Interface) error {
	if err := f(node, parent, manifest, r); err != nil {
		return err
	}
	manifestNode := manifest
	if node.Manifest != "" {
		manifestNode = node
	}
	for _, nodeChild := range node.Structure {
		if err := processManifestToNodeTreeTransfromation(f, nodeChild, node, manifestNode, r); err != nil {
			if node.Manifest != "" {
				return fmt.Errorf("manifest %s -> %w", node.Manifest, err)
			}
			return err
		}
	}
	return nil
}

func processNodeTree(manifest *Node, r registry.Interface, shouldRemoveNilNodes bool, functions ...NodeTransformation) error {
	for i := range functions {
		runTreeChangeProcedure, err := processTransformation(functions[i], manifest, nil, r)
		if err != nil {
			return err
		}
		if runTreeChangeProcedure {
			runTCP, err := processTransformation(calculatePath, manifest, nil, r)
			if err != nil {
				return err
			}
			must.BeFalse(runTCP)
			runTCP, err = processTransformation(mergeFolders, manifest, nil, r)
			if err != nil {
				return err
			}
			must.BeFalse(runTCP)
			runTCP, err = processTransformation(calculatePath, manifest, nil, r)
			if err != nil {
				return err
			}
			must.BeFalse(runTCP)
			runTCP, err = processTransformation(setParent, manifest, nil, r)
			if err != nil {
				return err
			}
			must.BeFalse(runTCP)
		}
		if shouldRemoveNilNodes {
			// remove nil nodes after each nodeTransformation
			if err := removeNilNodes(manifest); err != nil {
				return err
			}
		}
	}
	return nil
}

func removeNilNodes(node *Node) error {
	node.Structure = slices.DeleteFunc(node.Structure, func(child *Node) bool {
		return child == nil
	})

	// TODO: implement this logic after manifests have been cleaned up from empty dirs
	// if node.Type == "dir" && len(node.Structure) == 0 {
	// 	return fmt.Errorf("there is an empty directory with path %s", node.NodePath())
	// }

	for _, child := range node.Structure {
		if err := removeNilNodes(child); err != nil {
			return err
		}
	}
	return nil
}

func processTransformation(f NodeTransformation, node *Node, parent *Node, r registry.Interface) (bool, error) {
	runTreeChangeProcedure, err := f(node, parent, r)
	if err != nil {
		return runTreeChangeProcedure, err
	}
	for _, child := range node.Structure {
		childRunTreeChangeProcedure, err := processTransformation(f, child, node, r)
		if err != nil {
			if node.Manifest != "" {
				return runTreeChangeProcedure, fmt.Errorf("manifest %s -> %w", node.Manifest, err)
			}
			return runTreeChangeProcedure, err
		}
		runTreeChangeProcedure = runTreeChangeProcedure || childRunTreeChangeProcedure
	}
	return runTreeChangeProcedure, nil
}

func loadRepositoriesOfResources(node *Node, parent *Node, _ *Node, r registry.Interface) error {
	loadRepoFrom := func(resourceURL string) error {
		if repositoryhost.IsResourceURL(resourceURL) {
			return r.LoadRepository(context.TODO(), resourceURL)
		}
		return nil
	}
	loadErr := errors.Join(loadRepoFrom(node.File), loadRepoFrom(node.Source), loadRepoFrom(node.FileTree), loadRepoFrom(node.Manifest))
	for _, multiSource := range node.MultiSource {
		loadErr = errors.Join(loadErr, loadRepoFrom(multiSource))
	}
	return loadErr
}

func readManifestContents(node *Node, parent *Node, manifest *Node, r registry.Interface) error {
	// skip non-manifest nodes
	if node.Manifest == "" {
		return nil
	}
	// node.Manifest is a manifest to be loaded
	if repositoryhost.IsRelative(node.Manifest) {
		// manifest.Manifest has already been loaded into registry
		manifestResourceURL, err := r.ResolveRelativeLink(manifest.Manifest, node.Manifest)
		if err != nil {
			return fmt.Errorf("can't build manifest node %s absolute URL : %w ", node.Manifest, err)
		}
		node.Manifest = manifestResourceURL
	}
	// load for the read to succeed
	if err := r.LoadRepository(context.TODO(), node.Manifest); err != nil {
		return err
	}
	byteContent, err := r.Read(context.TODO(), node.Manifest)
	if err != nil {
		return fmt.Errorf("can't get manifest file content : %w", err)
	}
	if err = yaml.Unmarshal(byteContent, node); err != nil {
		return fmt.Errorf("can't parse manifest %s yaml content : %w", node.Manifest, err)
	}
	return nil
}

func removeManifestNodes(node *Node, parent *Node, _ *Node, r registry.Interface) error {
	if node.Type != "manifest" || parent == nil {
		return nil
	}
	parent.Structure = append(parent.Structure, node.Structure...)
	node.Structure = nil
	RemoveNodeFromParent(node, parent)
	return nil
}

func decideNodeType(node *Node, _ *Node, _ registry.Interface) (bool, error) {
	node.Type = ""
	candidateType := []string{}
	if node.Manifest != "" {
		candidateType = append(candidateType, "manifest")
	}
	if node.File != "" {
		candidateType = append(candidateType, "file")
	}
	if node.Dir != "" {
		candidateType = append(candidateType, "dir")
	}
	if node.FileTree != "" {
		candidateType = append(candidateType, "fileTree")
	}
	switch len(candidateType) {
	case 0:
		return false, fmt.Errorf("there is a node \n\n%s\nof no type", node)
	case 1:
		node.Type = candidateType[0]
		return false, nil
	default:
		return false, fmt.Errorf("there is a node \n\n%s\ntrying to be %s", node, strings.Join(candidateType, ","))
	}
}

func validateTreeAfterManifestToNodeTree(_ *Node, _ *Node, _ registry.Interface) (bool, error) {
	return true, nil
}

func calculatePath(node *Node, parent *Node, _ registry.Interface) (bool, error) {
	if parent == nil {
		return false, nil
	}
	if parent.Path == "" {
		node.Path = "."
		return false, nil
	}
	switch parent.Type {
	case "dir":
		node.Path = path.Join(parent.Path, parent.Dir)
	case "manifest":
		node.Path = parent.Path
	default:
		return false, fmt.Errorf("parent node \n\n%s\n is not a dir or manifest", node)
	}
	return false, nil
}

func resolveManifestLinks(node *Node, _ *Node, manifest *Node, r registry.Interface) error {
	resolveLink := func(link *string) error {
		if *link == "" {
			return nil
		}
		if repositoryhost.IsResourceURL(*link) {
			if _, err := r.ResourceURL(*link); err != nil {
				return fmt.Errorf("%s does not exist: %w", *link, err)
			}
			return nil
		}
		newLink, err := r.ResolveRelativeLink(manifest.Manifest, *link)
		if err != nil {
			return fmt.Errorf("cant build node's absolute link %s : %w", *link, err)
		}
		*link = newLink
		return nil
	}

	switch {
	case node.File != "":
		// TODO
		// Don't calculate source for empty _index.md file
		if node.File == sectionFile && node.Source == "" {
			return nil
		}
		if strings.Contains(node.File, "/") {
			node.Source = node.File
			node.File = path.Base(node.File)
		}
		for i := range node.MultiSource {
			if err := resolveLink(&node.MultiSource[i]); err != nil {
				return err
			}
		}
		return resolveLink(&node.Source)
	case node.FileTree != "":
		return resolveLink(&node.FileTree)
	}
	return nil
}

func removeFileTreeNodes(node *Node, parent *Node, r registry.Interface) (bool, error) {
	if node.Type != "fileTree" {
		return false, nil
	}
	files, err := r.Tree(node.FileTree)
	if err != nil {
		return false, err
	}
	changed, err := constructNodeTree(files, node, parent)
	if err != nil {
		return changed, err
	}
	RemoveNodeFromParent(node, parent)
	return changed, nil
}

// RemoveNodeFromParent removes node from its parent node
func RemoveNodeFromParent(node *Node, parent *Node) {
	for i, child := range parent.Structure {
		if child == node {
			size := len(parent.Structure)
			parent.Structure[i] = parent.Structure[size-1]
			parent.Structure = parent.Structure[:size-1]
			return
		}
	}
}

func constructNodeTree(files []string, node *Node, parent *Node) (bool, error) {
	changed := false
	pathToDirNode := map[string]*Node{}
	pathToDirNode[node.Path] = parent
	for _, file := range files {
		shouldExclude := false
		for _, excludeFile := range node.ExcludeFiles {
			if strings.HasPrefix(file, excludeFile) {
				shouldExclude = true
				break
			}
		}
		if shouldExclude {
			continue
		}
		source, err := url.JoinPath(strings.Replace(node.FileTree, "/tree/", "/blob/", 1), file)
		if err != nil {
			return changed, err
		}
		// url.JoinPath escapes once so we revert it's escape
		source, err = url.PathUnescape(source)
		if err != nil {
			return changed, err
		}
		fileName := path.Base(file)
		filePath, err := link.Build(node.Path, path.Dir(file))
		if err != nil {
			return changed, err
		}
		parentNode := getParrentNode(pathToDirNode, filePath)
		parentNode.Structure = append(parentNode.Structure, &Node{
			FileType: FileType{
				File:   fileName,
				Source: source,
			},
			Type: "file",
			Path: filePath,
		})
		changed = true
	}
	return changed, nil
}

func getParrentNode(pathToDirNode map[string]*Node, parentPath string) *Node {
	if parent, ok := pathToDirNode[parentPath]; ok {
		return parent
	}
	// construct parent node
	out := &Node{
		DirType: DirType{
			Dir: path.Base(parentPath),
		},
		Type: "dir",
		Path: parentPath,
	}
	outParent := getParrentNode(pathToDirNode, path.Dir(parentPath))
	outParent.Structure = append(outParent.Structure, out)
	pathToDirNode[parentPath] = out
	return out
}

func mergeFolders(node *Node, parent *Node, _ registry.Interface) (bool, error) {
	nodeNameToNode := map[string]*Node{}
	for _, child := range node.Structure {
		switch child.Type {
		case "dir":
			if mergeIntoNode, ok := nodeNameToNode[child.Dir]; ok {
				if mergeIntoNode.Type == "file" {
					return false, fmt.Errorf("there is a file \n\n%s\n colliding with directory \n\n%s", mergeIntoNode, child)
				}
				mergeIntoNode.Structure = append(mergeIntoNode.Structure, child.Structure...)
				RemoveNodeFromParent(child, node)
				// TODO should be removed?
				if len(child.Frontmatter) > 0 {
					if len(nodeNameToNode[child.Dir].Frontmatter) > 0 {
						return false, fmt.Errorf("there are multiple dirs with name %s and path %s that have frontmatter. Please only use one", child.Dir, child.Path)
					}
					nodeNameToNode[child.Dir].Frontmatter = child.Frontmatter
				}
			} else {
				nodeNameToNode[child.Dir] = child
			}
		case "file":
			if collidedWith, ok := nodeNameToNode[child.File]; ok {
				return false, fmt.Errorf("file \n\n%s\n that will be written in %s causes collision with: \n\n%s", child, child.Path, collidedWith)
			}
			nodeNameToNode[child.File] = child
		}
	}
	return false, nil
}

func setParent(node *Node, parent *Node, _ registry.Interface) (bool, error) {
	node.parent = parent
	return false, nil
}

func setDefaultProcessor(node *Node, parent *Node, _ registry.Interface) (bool, error) {
	if node.Type == "file" && node.Processor == "" {
		node.Processor = "downloader"
	}
	return false, nil
}

// ResolveManifest collects files in FileCollector from a given url and resourcehandlers.FileSource
func ResolveManifest(url string, r registry.Interface, additionalTransformations ...NodeTransformation) ([]*Node, error) {
	manifest := &Node{
		ManifType: ManifType{
			Manifest: url,
		},
	}
	err := manifestToNodeTree(manifest, r,
		readManifestContents,
		// needed for resolveManifestLinks during check if links point to existing resources
		loadRepositoriesOfResources,
		resolveManifestLinks,
		removeManifestNodes,
	)
	if err != nil {
		return nil, err
	}

	err = processNodeTree(manifest, r, false,
		decideNodeType,
		validateTreeAfterManifestToNodeTree,
		removeFileTreeNodes,
		setDefaultProcessor,
	)
	if err != nil {
		return nil, err
	}
	err = processNodeTree(manifest, r, true, additionalTransformations...)
	if err != nil {
		return nil, err
	}
	return getAllNodes(manifest), nil
}

// GetAllNodes returns all nodes in a manifest as arrayqgi
func getAllNodes(node *Node) []*Node {
	collected := []*Node{node}
	for _, child := range node.Structure {
		collected = append(collected, getAllNodes(child)...)
	}
	return collected
}
