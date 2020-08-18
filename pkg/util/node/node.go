package node

import (
	"strings"

	"github.com/gardener/docode/pkg/api"
)

func NodePath(node *api.Node, separator string) string {
	var pathSegments []string
	for _, parent := range node.Parents() {
		if parent.Name != "" {
			pathSegments = append(pathSegments, parent.Name)
		}
	}

	return strings.Join(pathSegments, separator)
}

func GetRootNode(node *api.Node) *api.Node {
	if node == nil {
		return nil
	}

	parentNodes := node.Parents()
	return parentNodes[len(parentNodes)-1]
}
