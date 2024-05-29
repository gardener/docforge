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
	"strings"

	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	"gopkg.in/yaml.v2"
)

const sectionFile = "_index.md"

type nodeTransformation func(node *Node, parent *Node, manifest *Node, r registry.Interface) error

func processManifest(f nodeTransformation, node *Node, parent *Node, manifest *Node, r registry.Interface) error {
	if err := f(node, parent, manifest, r); err != nil {
		return err
	}
	manifestNode := manifest
	if node.Manifest != "" {
		manifestNode = node
	}
	for _, nodeChild := range node.Structure {
		if err := processManifest(f, nodeChild, node, manifestNode, r); err != nil {
			if node.Manifest != "" {
				return fmt.Errorf("manifest %s -> %w", node.Manifest, err)
			}
			return err
		}
	}
	return nil
}

func loadManifestStructure(node *Node, parent *Node, manifest *Node, r registry.Interface) error {
	var err error
	loadRepoFrom := func(resourceURL string) error {
		if repositoryhost.IsResourceURL(resourceURL) {
			return r.LoadRepository(context.TODO(), resourceURL)
		}
		return nil
	}
	loadErr := errors.Join(loadRepoFrom(node.FileTree), loadRepoFrom(node.File), loadRepoFrom(node.Source))
	for _, multiSource := range node.MultiSource {
		loadErr = errors.Join(loadErr, loadRepoFrom(multiSource))
	}
	if node.Manifest == "" {
		return nil
	}
	// node.Manifest is a manifest to be loaded
	if repositoryhost.IsRelative(node.Manifest) {
		// manifest.Manifest has already been loaded
		newManifest, err := r.ResolveRelativeLink(manifest.Manifest, node.Manifest)
		if err != nil {
			return fmt.Errorf("can't build manifest node %s absolute URL : %w ", node.Manifest, err)
		}
		node.Manifest = newManifest
	}
	loadErr = errors.Join(loadErr, loadRepoFrom(node.Manifest))
	if loadErr != nil {
		return loadErr
	}
	byteContent, err := r.Read(context.TODO(), node.Manifest)
	if err != nil {
		return fmt.Errorf("can't get manifest file content : %w", err)
	}
	content := string(byteContent)
	if err = yaml.Unmarshal([]byte(content), node); err != nil {
		return fmt.Errorf("can't parse manifest %s yaml content : %w", node.Manifest, err)
	}
	return nil
}

func moveManifestContentIntoTree(node *Node, parent *Node, manifest *Node, r registry.Interface) error {
	if node.Type != "manifest" {
		return nil
	}
	if parent != nil {
		parent.Structure = append(parent.Structure, node.Structure...)
		node.Structure = nil
	}
	return nil
}

func decideNodeType(node *Node, _ *Node, _ *Node, _ registry.Interface) error {
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
		return fmt.Errorf("there is a node \n\n%s\nof no type", node)
	case 1:
		node.Type = candidateType[0]
		return nil
	default:
		return fmt.Errorf("there is a node \n\n%s\ntrying to be %s", node, strings.Join(candidateType, ","))
	}
}

func calculatePath(node *Node, parent *Node, _ *Node, _ registry.Interface) error {
	if parent == nil {
		return nil
	}
	if parent.Path == "" {
		node.Path = "."
		return nil
	}
	switch parent.Type {
	case "dir":
		node.Path = path.Join(parent.Path, parent.Dir)
	case "manifest":
		node.Path = parent.Path
	default:
		return fmt.Errorf("parent node \n\n%s\n is not a dir or manifest", node)
	}
	return nil
}

func resolveRelativeLinks(node *Node, _ *Node, manifest *Node, r registry.Interface) error {
	resolveLink := func(link *string) error {
		if *link == "" {
			return nil
		}
		if repositoryhost.IsResourceURL(*link) {
			if _, err := r.ResourceURL(*link); err == nil {
				return nil
			}
			return fmt.Errorf("%s does not exist", *link)
		}
		newLink, err := r.ResolveRelativeLink(manifest.Manifest, *link)
		if err != nil {
			return fmt.Errorf("cant build node's absolute link %s : %w", *link, err)
		}
		*link = newLink
		return nil
	}

	switch node.Type {
	case "file":
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
	case "fileTree":
		return resolveLink(&node.FileTree)
	}
	return nil
}

func extractFilesFromNode(node *Node, parent *Node, manifest *Node, r registry.Interface) error {
	switch node.Type {
	case "file":
		if !strings.HasSuffix(node.File, ".md") {
			node.File += ".md"
		}
	case "fileTree":
		files, err := r.Tree(node.FileTree)
		if err != nil {
			return err
		}
		if err := constructNodeTree(files, node, parent); err != nil {
			return err
		}
		removeNodeFromParent(node, parent)
	}
	return nil
}

func removeNodeFromParent(node *Node, parent *Node) {
	for i, child := range parent.Structure {
		if child == node {
			size := len(parent.Structure)
			parent.Structure[i] = parent.Structure[size-1]
			parent.Structure = parent.Structure[:size-1]
			return
		}
	}
}

func constructNodeTree(files []string, node *Node, parent *Node) error {
	pathToDirNode := map[string]*Node{}
	pathToDirNode[node.Path] = parent
	for _, file := range files {
		extension := path.Ext(file)
		if extension != ".md" && extension != "" {
			continue
		}
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
			return err
		}
		// url.JoinPath escapes once so we revert it's escape
		source, err = url.PathUnescape(source)
		if err != nil {
			return err
		}
		fileName := path.Base(file)
		if !strings.HasSuffix(fileName, ".md") {
			fileName = fileName + ".md"
		}
		filePath := path.Join(node.Path, path.Dir(file))
		parentNode := getParrentNode(pathToDirNode, filePath)
		parentNode.Structure = append(parentNode.Structure, &Node{
			FileType: FileType{
				File:   fileName,
				Source: source,
			},
			Type: "file",
			Path: filePath,
		})
	}
	return nil
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

func mergeFolders(node *Node, parent *Node, manifest *Node, _ registry.Interface) error {
	nodeNameToNode := map[string]*Node{}
	for _, child := range node.Structure {
		switch child.Type {
		case "dir":
			if mergeIntoNode, ok := nodeNameToNode[child.Dir]; ok {
				mergeIntoNode.Structure = append(mergeIntoNode.Structure, child.Structure...)
				removeNodeFromParent(child, node)
			} else {
				nodeNameToNode[child.Dir] = child
			}
		case "file":
			if _, ok := nodeNameToNode[child.File]; ok {
				if child.Frontmatter != nil && nodeNameToNode[child.File].Frontmatter != nil && child.Frontmatter["persona"] != nodeNameToNode[child.File].Frontmatter["persona"] {
					persona, _ := child.Frontmatter["persona"].(string)
					child.File = strings.ReplaceAll(child.File, ".md", "-"+personaToDir[persona]+".md")
				} else {
					return fmt.Errorf("file \n\n%s\nin manifest %s that will be written in %s causes collision", child, manifest.ManifType.Manifest, child.Path)
				}
			}
			nodeNameToNode[child.File] = child
		}
	}
	return nil
}

var dirToPersona = map[string]string{"usage": "Users", "operations": "Operators", "development": "Developers"}
var personaToDir = map[string]string{"Users": "usage", "Operators": "operations", "Developers": "development"}

func resolvePersonaFolders(node *Node, parent *Node, manifest *Node, _ registry.Interface) error {
	if node.Type == "dir" && (node.Dir == "development" || node.Dir == "operations" || node.Dir == "usage") {
		for _, child := range node.Structure {
			if child.Frontmatter == nil {
				child.Frontmatter = map[string]interface{}{}
			}
			child.Frontmatter["persona"] = dirToPersona[node.Dir]
			finalAlias := strings.TrimSuffix(child.Name(), ".md") + "/"
			if child.Name() == sectionFile {
				finalAlias = ""
			}
			child.Frontmatter["aliases"] = []interface{}{"/" + parent.Path + "/" + node.Dir + "/" + finalAlias}
		}
		parent.Structure = append(parent.Structure, node.Structure...)
		removeNodeFromParent(node, parent)
	}
	return nil
}

func propagateFrontmatter(node *Node, parent *Node, manifest *Node, _ registry.Interface) error {
	if parent != nil {
		newFM := map[string]interface{}{}
		for k, v := range parent.Frontmatter {
			newFM[k] = v
		}
		for k, v := range node.Frontmatter {
			newFM[k] = v
		}
		node.Frontmatter = newFM
	}
	return nil
}

func setParent(node *Node, parent *Node, _ *Node, _ registry.Interface) error {
	node.parent = parent
	return nil
}

// func calculateAliases(node *Node, parent *Node, _ *Node, _ registry.Interface) error {
// 	var (
// 		nodeAliases  []interface{}
// 		childAliases []interface{}
// 		formatted    bool
// 	)
// 	if nodeAliases, formatted = node.Frontmatter["aliases"].([]interface{}); node.Frontmatter != nil && node.Frontmatter["aliases"] != nil && !formatted {
// 		return fmt.Errorf("node X \n\n%s\n has invalid alias format", node)
// 	}
// 	for _, nodeAlias := range nodeAliases {
// 		for _, child := range node.Structure {
// 			if child.Frontmatter == nil {
// 				child.Frontmatter = map[string]interface{}{}
// 			}
// 			if child.Frontmatter["aliases"] == nil {
// 				child.Frontmatter["aliases"] = []interface{}{}
// 			}
// 			if childAliases, formatted = child.Frontmatter["aliases"].([]interface{}); !formatted {
// 				return fmt.Errorf("node \n\n%s\n has invalid alias format", child)
// 			}
// 			finalAlias := strings.TrimSuffix(child.Name(), ".md") + "/"
// 			if child.Name() == "_index.md" {
// 				finalAlias = ""
// 			}
// 			childAliases = append(childAliases, fmt.Sprintf("%s", nodeAlias)+"/"+finalAlias)
// 			child.Frontmatter["aliases"] = childAliases
// 		}
// 	}
// 	return nil
// }

// ResolveManifest collects files in FileCollector from a given url and resourcehandlers.FileSource
func ResolveManifest(url string, r registry.Interface) ([]*Node, error) {
	manifest := Node{
		ManifType: ManifType{
			Manifest: url,
		},
	}
	if err := processManifest(loadManifestStructure, &manifest, nil, &manifest, r); err != nil {
		return nil, err
	}
	if err := processManifest(decideNodeType, &manifest, nil, &manifest, r); err != nil {
		return nil, err
	}
	if err := processManifest(calculatePath, &manifest, nil, &manifest, r); err != nil {
		return nil, err
	}
	if err := processManifest(resolveRelativeLinks, &manifest, nil, &manifest, r); err != nil {
		return nil, err
	}
	if err := processManifest(extractFilesFromNode, &manifest, nil, &manifest, r); err != nil {
		return nil, err
	}
	if err := processManifest(moveManifestContentIntoTree, &manifest, nil, &manifest, r); err != nil {
		return nil, err
	}
	if err := processManifest(mergeFolders, &manifest, nil, &manifest, r); err != nil {
		return nil, err
	}
	if err := processManifest(resolvePersonaFolders, &manifest, nil, &manifest, r); err != nil {
		return nil, err
	}
	// if err := processManifest(calculateAliases, &manifest, nil, &manifest, r); err != nil {
	// 	return nil, err
	// }
	if err := processManifest(calculatePath, &manifest, nil, &manifest, r); err != nil {
		return nil, err
	}
	if err := processManifest(mergeFolders, &manifest, nil, &manifest, r); err != nil {
		return nil, err
	}
	if err := processManifest(calculatePath, &manifest, nil, &manifest, r); err != nil {
		return nil, err
	}
	if err := processManifest(setParent, &manifest, nil, &manifest, r); err != nil {
		return nil, err
	}
	if err := processManifest(propagateFrontmatter, &manifest, nil, &manifest, r); err != nil {
		return nil, err
	}
	return getAllNodes(&manifest), nil
}

// GetAllNodes returns all nodes in a manifest as arrayqgi
func getAllNodes(node *Node) []*Node {
	collected := []*Node{node}
	for _, child := range node.Structure {
		collected = append(collected, getAllNodes(child)...)
	}
	return collected
}
