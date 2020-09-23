package node

import (
	"strings"

	"github.com/gardener/docforge/pkg/api"
)

// Path serializes the node parents path to root
// as string of segments that are the parents names and
// and delimited by separator
func Path(node *api.Node, separator string) string {
	var pathSegments []string
	for _, parent := range node.Parents() {
		if parent.Name != "" {
			pathSegments = append(pathSegments, parent.Name)
		}
	}

	return strings.Join(pathSegments, separator)
}

// GetRootNode returns the root node in the parents path
// for a node object n
func GetRootNode(node *api.Node) *api.Node {
	if node == nil {
		return nil
	}

	parentNodes := node.Parents()
	return parentNodes[len(parentNodes)-1]
}
