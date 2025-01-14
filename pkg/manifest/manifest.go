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

	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	"gopkg.in/yaml.v2"
)

const sectionFile = "_index.md"

type nodeTransformation func(node *Node, parent *Node, manifest *Node, r registry.Interface, contentFileFormats []string) error

func processManifest(node *Node, parent *Node, manifest *Node, r registry.Interface, contentFileFormats []string, functions ...nodeTransformation) error {
	for i := range functions {
		if err := processTransformation(functions[i], node, parent, manifest, r, contentFileFormats); err != nil {
			return err
		}
	}
	return nil
}

func processTransformation(f nodeTransformation, node *Node, parent *Node, manifest *Node, r registry.Interface, contentFileFormats []string) error {
	if err := f(node, parent, manifest, r, contentFileFormats); err != nil {
		return err
	}
	manifestNode := manifest
	if node.Manifest != "" {
		manifestNode = node
	}
	for _, nodeChild := range node.Structure {
		if err := processTransformation(f, nodeChild, node, manifestNode, r, contentFileFormats); err != nil {
			if node.Manifest != "" {
				return fmt.Errorf("manifest %s -> %w", node.Manifest, err)
			}
			return err
		}
	}
	return nil
}

func loadRepositoriesOfResources(node *Node, parent *Node, manifest *Node, r registry.Interface, _ []string) error {
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

func loadManifestStructure(node *Node, parent *Node, manifest *Node, r registry.Interface, _ []string) error {
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

func moveManifestContentIntoTree(node *Node, parent *Node, manifest *Node, r registry.Interface, _ []string) error {
	if node.Type != "manifest" {
		return nil
	}
	if parent != nil {
		parent.Structure = append(parent.Structure, node.Structure...)
		node.Structure = nil
	}
	return nil
}

func decideNodeType(node *Node, _ *Node, _ *Node, _ registry.Interface, _ []string) error {
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

func calculatePath(node *Node, parent *Node, _ *Node, _ registry.Interface, _ []string) error {
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

func resolveRelativeLinks(node *Node, _ *Node, manifest *Node, r registry.Interface, _ []string) error {
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

func checkFileTypeFormats(node *Node, _ *Node, manifest *Node, r registry.Interface, contentFileFormats []string) error {
	if node.Type != "file" {
		return nil
	}
	files := append(node.FileType.MultiSource, node.FileType.Source, node.FileType.File)
	for _, file := range files {
		// we do || file == "" to skip empty fields
		if !slices.ContainsFunc(contentFileFormats, func(fileFormat string) bool { return strings.HasSuffix(file, fileFormat) || file == "" }) {
			return fmt.Errorf("file format of %s isn't supported", file)
		}
	}
	return nil
}

func extractFilesFromNode(node *Node, parent *Node, manifest *Node, r registry.Interface, contentFileFormats []string) error {
	if node.Type != "fileTree" {
		return nil
	}
	files, err := r.Tree(node.FileTree)
	if err != nil {
		return err
	}
	if err := constructNodeTree(files, node, parent, contentFileFormats); err != nil {
		return err
	}
	removeNodeFromParent(node, parent)
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

func constructNodeTree(files []string, node *Node, parent *Node, contentFileFormats []string) error {
	pathToDirNode := map[string]*Node{}
	pathToDirNode[node.Path] = parent
	for _, file := range files {
		if !slices.ContainsFunc(contentFileFormats, func(fileFormat string) bool { return strings.HasSuffix(file, fileFormat) }) {
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
		filePath := path.Join(node.Path, path.Dir(file))
		parentNode := getParrentNode(pathToDirNode, filePath, contentFileFormats)
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

func getParrentNode(pathToDirNode map[string]*Node, parentPath string, contentFileFormats []string) *Node {
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
	outParent := getParrentNode(pathToDirNode, path.Dir(parentPath), contentFileFormats)
	outParent.Structure = append(outParent.Structure, out)
	pathToDirNode[parentPath] = out
	return out
}

func mergeFolders(node *Node, parent *Node, manifest *Node, _ registry.Interface, _ []string) error {
	var personaToDir = map[string]string{"Users": "usage", "Operators": "operations", "Developers": "development"}
	nodeNameToNode := map[string]*Node{}
	for _, child := range node.Structure {
		switch child.Type {
		case "dir":
			if mergeIntoNode, ok := nodeNameToNode[child.Dir]; ok {
				mergeIntoNode.Structure = append(mergeIntoNode.Structure, child.Structure...)
				removeNodeFromParent(child, node)
				if len(child.Frontmatter) > 0 {
					if len(nodeNameToNode[child.Dir].Frontmatter) > 0 {
						return fmt.Errorf("there are multiple dirs with name %s and path %s that have frontmatter. Please only use one", child.Dir, child.Path)
					}
					nodeNameToNode[child.Dir].Frontmatter = child.Frontmatter
				}
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

func resolvePersonaFolders(node *Node, parent *Node, manifest *Node, _ registry.Interface, _ []string) error {
	if node.Type == "dir" && (node.Dir == "development" || node.Dir == "operations" || node.Dir == "usage") {
		for _, child := range node.Structure {
			addPersonaAliasesForNode(child, node.Dir, "/"+node.HugoPrettyPath())
		}
		parent.Structure = append(parent.Structure, node.Structure...)
		removeNodeFromParent(node, parent)
	}
	return nil
}

func addPersonaAliasesForNode(node *Node, personaDir string, parrentAlias string) {
	var dirToPersona = map[string]string{"usage": "Users", "operations": "Operators", "development": "Developers"}
	finalAlias := strings.TrimSuffix(node.Name(), ".md") + "/"
	if node.Name() == sectionFile {
		finalAlias = ""
	}
	childAlias := parrentAlias + finalAlias
	if node.Type == "file" {
		if node.Frontmatter == nil {
			node.Frontmatter = map[string]interface{}{}
		}
		node.Frontmatter["persona"] = dirToPersona[personaDir]
		node.Frontmatter["aliases"] = []interface{}{childAlias}
	}
	for _, child := range node.Structure {
		addPersonaAliasesForNode(child, personaDir, childAlias)
	}
}

func propagateFrontmatter(node *Node, parent *Node, manifest *Node, _ registry.Interface, _ []string) error {
	if parent != nil {
		newFM := map[string]interface{}{}
		for k, v := range parent.Frontmatter {
			if k != "aliases" {
				newFM[k] = v
			}
		}
		for k, v := range node.Frontmatter {
			newFM[k] = v
		}
		node.Frontmatter = newFM
	}
	return nil
}

func propagateSkipValidation(node *Node, parent *Node, manifest *Node, _ registry.Interface, _ []string) error {
	if parent != nil && parent.SkipValidation {
		node.SkipValidation = parent.SkipValidation
	}
	return nil
}

func setParent(node *Node, parent *Node, _ *Node, _ registry.Interface, _ []string) error {
	node.parent = parent
	return nil
}

func calculateAliases(node *Node, parent *Node, _ *Node, _ registry.Interface, _ []string) error {
	var (
		nodeAliases  []interface{}
		childAliases []interface{}
		formatted    bool
	)
	if nodeAliases, formatted = node.Frontmatter["aliases"].([]interface{}); node.Frontmatter != nil && node.Frontmatter["aliases"] != nil && !formatted {
		return fmt.Errorf("node X \n\n%s\n has invalid alias format", node)
	}
	for _, nodeAliasI := range nodeAliases {
		for _, child := range node.Structure {
			if child.Frontmatter == nil {
				child.Frontmatter = map[string]interface{}{}
			}
			if child.Frontmatter["aliases"] == nil {
				child.Frontmatter["aliases"] = []interface{}{}
			}
			if childAliases, formatted = child.Frontmatter["aliases"].([]interface{}); !formatted {
				return fmt.Errorf("node \n\n%s\n has invalid alias format", child)
			}
			childAliasSuffix := strings.TrimSuffix(child.Name(), ".md")
			if child.Name() == "_index.md" {
				childAliasSuffix = ""
			}
			nodeAlias := fmt.Sprintf("%s", nodeAliasI)
			if !strings.HasPrefix(nodeAlias, "/") {
				return fmt.Errorf("there is a node with name %s that has an relative alias %s", node.Name(), nodeAlias)
			}
			childAliases = append(childAliases, path.Join(nodeAlias, childAliasSuffix)+"/")
			child.Frontmatter["aliases"] = childAliases
		}
	}
	return nil
}

// ResolveManifest collects files in FileCollector from a given url and resourcehandlers.FileSource
func ResolveManifest(url string, r registry.Interface, contentFileFormats []string) ([]*Node, error) {
	manifest := Node{
		ManifType: ManifType{
			Manifest: url,
		},
	}
	err := processManifest(&manifest, nil, &manifest, r, contentFileFormats,
		loadManifestStructure,
		loadRepositoriesOfResources,
		decideNodeType,
		calculatePath,
		resolveRelativeLinks,
		checkFileTypeFormats,
		extractFilesFromNode,
		moveManifestContentIntoTree,
		mergeFolders,
		calculatePath,
		resolvePersonaFolders,
		calculatePath,
		mergeFolders,
		calculatePath,
		setParent,
		propagateFrontmatter,
		propagateSkipValidation,
		calculateAliases,
	)
	if err != nil {
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
