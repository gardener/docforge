package manifest

import (
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/gardener/docforge/pkg/util/urls"
	"gopkg.in/yaml.v2"
)

type nodeTransformation func(node *Node, parent *Node, manifest *Node, fs FileSource) error

func processManifest(f nodeTransformation, node *Node, parent *Node, manifest *Node, fs FileSource) error {
	if err := f(node, parent, manifest, fs); err != nil {
		return err
	}
	manifestNode := manifest
	if node.Manifest != "" {
		manifestNode = node
	}
	for _, child := range node.Structure {
		if err := processManifest(f, child, node, manifestNode, fs); err != nil {
			if node.Manifest != "" {
				return fmt.Errorf("manifest %s -> %w", node.Manifest, err)
			}
			return err
		}
	}
	return nil
}

func loadManifestStructure(node *Node, parent *Node, manifest *Node, fs FileSource) error {
	var (
		err         error
		content     string
		newManifest string
	)
	if node.Manifest == "" {
		return nil
	}
	if newManifest, err = fs.BuildAbsLink(manifest.Manifest, node.Manifest); err != nil {
		return fmt.Errorf("can't build manifest node %s absolute URL : %w ", node.Manifest, err)
	}
	node.Manifest = newManifest
	if content, err = fs.ManifestFromURL(node.Manifest); err != nil {
		return fmt.Errorf("can't get manifest file content : %w", err)
	}
	if err = yaml.Unmarshal([]byte(content), node); err != nil {
		return fmt.Errorf("can't parse manifest %s yaml content : %w", node.Manifest, err)
	}
	if parent != nil {
		parent.Structure = append(parent.Structure, node.Structure...)
		node.Structure = nil
	}
	return nil
}

func decideNodeType(node *Node, _ *Node, _ *Node, _ FileSource) error {
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

func calculatePath(node *Node, parent *Node, _ *Node, _ FileSource) error {
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

func resolveFileRelativeLinks(node *Node, _ *Node, manifest *Node, fs FileSource) error {
	var (
		err     error
		newLink string
	)
	switch node.Type {
	case "file":
		// Don't calculate source for empty _index.md file
		if node.File == "_index.md" && node.Source == "" {
			return nil
		}
		if strings.Contains(node.File, "/") {
			node.Source = node.File
			node.File = path.Base(node.File)
		}
		if newLink, err = fs.BuildAbsLink(manifest.Manifest, node.Source); err != nil {
			return fmt.Errorf("cant build node's absolute link %s : %w", node.Source, err)
		}
		node.Source = newLink
	case "fileTree":
		if newLink, err = fs.BuildAbsLink(manifest.Manifest, node.FileTree); err != nil {
			return fmt.Errorf("cant build node's absolute link %s : %w", node.FileTree, err)
		}
		node.FileTree = newLink
	}
	return nil
}

func extractFilesFromNode(node *Node, parent *Node, manifest *Node, fs FileSource) error {
	switch node.Type {
	case "file":
		if !strings.HasSuffix(node.File, ".md") {
			node.File += ".md"
		}
	case "fileTree":
		files, err := fs.FileTreeFromURL(node.FileTree)
		if err != nil {
			return nil
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
		extension := urls.Ext(file)
		if extension != "md" && extension != "" {
			continue
		}
		source, err := url.JoinPath(strings.Replace(node.FileTree, "/tree/", "/blob/", 1), file)
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

func mergeFolders(node *Node, parent *Node, manifest *Node, _ FileSource) error {
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
				return fmt.Errorf("file \n\n%s\nin manifest %s that will be written in %s causes collision", child, manifest.ManifType.Manifest, child.Path)
			}
			nodeNameToNode[child.File] = child
		}
	}
	return nil
}

var dirToPersona = map[string]string{"usage": "Users", "operations": "Operators", "development": "Developers"}

func resolvePersonaFolders(node *Node, parent *Node, manifest *Node, _ FileSource) error {
	if node.Type == "dir" {
		if node.Dir == "development" || node.Dir == "operations" || node.Dir == "usage" {
			for _, child := range node.Structure {
				if child.Properties == nil {
					child.Properties = map[string]interface{}{}
				}
				//TODO: merge maps
				//	if node.Properties["frontmatter"] != nil && node.Properties["frontmatter"]["categories"] == nil {
				child.Properties["frontmatter"] = MergeStringMaps(child.Properties, map[string]interface{}{"persona": dirToPersona[node.Dir]})
			}
			parent.Structure = append(parent.Structure, node.Structure...)
			removeNodeFromParent(node, parent)
		}
	}

	return nil
}

// MergeStringMaps merges the content of the newMaps with the oldMap. If a key already exists then
// it gets overwritten by the last value with the same key.
func MergeStringMaps[T any](oldMap map[string]T, newMaps ...map[string]T) map[string]T {
	var out map[string]T

	if oldMap != nil {
		out = make(map[string]T, len(oldMap))
	}
	for k, v := range oldMap {
		out[k] = v
	}

	for _, newMap := range newMaps {
		if newMap != nil && out == nil {
			out = make(map[string]T)
		}

		for k, v := range newMap {
			out[k] = v
		}
	}

	return out
}

func setParent(node *Node, parent *Node, _ *Node, _ FileSource) error {
	node.parent = parent
	return nil
}

// ResolveManifest collects files in FileCollector from a given url and FileSource
func ResolveManifest(url string, fs FileSource) (*Node, error) {
	manifest := Node{
		ManifType: ManifType{
			Manifest: url,
		},
	}
	if err := processManifest(loadManifestStructure, &manifest, nil, &manifest, fs); err != nil {
		return nil, err
	}
	if err := processManifest(decideNodeType, &manifest, nil, &manifest, fs); err != nil {
		return nil, err
	}
	if err := processManifest(calculatePath, &manifest, nil, &manifest, fs); err != nil {
		return nil, err
	}
	if err := processManifest(resolveFileRelativeLinks, &manifest, nil, &manifest, fs); err != nil {
		return nil, err
	}
	if err := processManifest(extractFilesFromNode, &manifest, nil, &manifest, fs); err != nil {
		return nil, err
	}
	if err := processManifest(mergeFolders, &manifest, nil, &manifest, fs); err != nil {
		return nil, err
	}
	if err := processManifest(resolvePersonaFolders, &manifest, nil, &manifest, fs); err != nil {
		return nil, err
	}
	if err := processManifest(calculatePath, &manifest, nil, &manifest, fs); err != nil {
		return nil, err
	}
	if err := processManifest(setParent, &manifest, nil, &manifest, fs); err != nil {
		return nil, err
	}
	return &manifest, nil
}
