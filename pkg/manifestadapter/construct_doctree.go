package manifestadapter

import (
	"fmt"
	"path"
	"strings"

	"github.com/gardener/docforge/pkg/manifest"
)

// ConstructDocTree construct a node document from file list
func ConstructDocTree(files []*manifest.Node) (*Node, error) {
	pathToNode := map[string]*Node{}
	for _, file := range files {
		switch file.Type {
		case "file":
			newNode := Node{
				Name:        file.File,
				Source:      file.Source,
				MultiSource: file.MultiSource,
				Properties:  file.Properties,
			}
			pathToNode[path.Join(file.Path, file.File)] = &newNode
			_ = constructParentChain(file.Path, &newNode, pathToNode)
		}
	}
	for _, dir := range files {
		switch dir.Type {
		case "dir":
			n := path.Join(dir.Path, dir.Dir)
			if dir.Properties != nil && pathToNode[n] != nil {
				pathToNode[n].Properties, _ = getFrontmatter(pathToNode[n].Properties["frontmatter"], dir.Properties, "")
			}
		}
	}
	return pathToNode["."], nil
}

///////////// frontmatter processor ////////

func getFrontmatter(category interface{}, frontmatter interface{}, nodepath string) (map[string]interface{}, error) {
	output := map[string]interface{}{}
	switch frontmatter.(type) {
	case map[string]interface{}:
		output, _ = frontmatter.(map[string]interface{})
	case map[interface{}]interface{}:
		iimap, _ := frontmatter.(map[interface{}]interface{})
		for key, value := range iimap {
			output[fmt.Sprintf("%v", key)] = value
		}
	default:
		return nil, fmt.Errorf("invalid frontmatter properties for node: %s", nodepath)
	}
	if c, ok := category.(map[string]interface{}); ok {
		if converted, ok := output["frontmatter"].(map[interface{}]interface{}); ok {
			converted["persona"] = c["persona"]
		}
	}
	return output, nil
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

var (
	translate map[string]string = map[string]string{"usage": "Users", "operations": "Operators", "development": "Developers"}
)

// here we will intercept user, ops, dev
func constructParentChain(dirPath string, node *Node, pathToNode map[string]*Node) *Node {
	if node.Name == "." {
		return nil
	}
	dir := path.Base(dirPath)
	if dir == "usage" || dir == "operations" || dir == "development" {
		if node.Properties == nil {
			node.Properties = map[string]interface{}{}
		}
		//TODO: merge maps
		//	if node.Properties["frontmatter"] != nil && node.Properties["frontmatter"]["categories"] == nil {
		node.Properties["frontmatter"] = MergeStringMaps(node.Properties, map[string]interface{}{"persona": translate[dir]})
		//	}
		return constructParentChain(parentDirPath(dirPath), node, pathToNode)
	}

	dirNode, ok := pathToNode[dirPath]
	if !ok {
		dirNode = &Node{
			Name: dir,
		}
		pathToNode[dirPath] = dirNode
		constructParentChain(parentDirPath(dirPath), dirNode, pathToNode)
	}
	node.SetParent(dirNode)
	dirNode.Nodes = append(dirNode.Nodes, node)
	return dirNode
}

func parentDirPath(fullPath string) string {
	return path.Dir(strings.TrimSuffix(fullPath, "/"))
}
