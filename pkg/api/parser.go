// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gardener/docforge/pkg/util"
	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	"gopkg.in/yaml.v3"
)

// ParsingOptions Options that are given to the parser in the api package
type ParsingOptions struct {
	ExtractedFilesFormats []string `mapstructure:"extracted-files-formats"`
	Hugo                  bool     `mapstructure:"hugo"`
}

// Parse is a function which construct documentation struct from given byte array
func Parse(b []byte, options ParsingOptions) (*Documentation, error) {
	var err error
	docs := &Documentation{}
	if err = yaml.Unmarshal(b, docs); err != nil {
		return nil, err
	}
	// init parents
	for _, n := range docs.Structure {
		n.SetParentsDownwards()
	}
	if err = validateDocumentation(docs, options); err != nil {
		return nil, err
	}

	return docs, nil
}

func validateDocumentation(d *Documentation, options ParsingOptions) error {
	var errs error
	if d.Structure == nil && d.NodeSelector == nil {
		errs = multierror.Append(errs, fmt.Errorf("the document structure must contains at least one of these properties: structure, nodesSelector"))
	}
	if err := validateSectionFile(d.Structure); err != nil {
		errs = multierror.Append(errs, err)
	}
	for _, n := range d.Structure {
		if err := validateNode(n); err != nil {
			errs = multierror.Append(errs, err)
		}
	}
	if err := CheckForCollisions(d.Structure, options); err != nil {
		return err
	}
	if err := validateNodeSelector(d.NodeSelector, "/"); err != nil {
		errs = multierror.Append(errs, err)
	}
	return errs
}

func validateNodeSelector(selector *NodeSelector, root string) error {
	if selector != nil && selector.Path == "" {
		return fmt.Errorf("nodesSelector under %s must contains a path property", root)
	}
	return nil
}

func validateNode(n *Node) error {
	var errs error
	if n.Source == "" && n.Name == "" {
		errs = multierror.Append(errs, fmt.Errorf("node %s must contains at least one of these properties: source, name", n.FullName("/")))
	}
	if n.Source == "" && n.NodeSelector == nil && n.MultiSource == nil && n.Nodes == nil {
		errs = multierror.Append(errs, fmt.Errorf("node %s must contains at least one of these properties: source, nodesSelector, multiSource, nodes", n.FullName("/")))
	}
	if len(n.Sources()) > 0 && (n.Nodes != nil || n.NodeSelector != nil) {
		errs = multierror.Append(errs, fmt.Errorf("node %s must be categorized as a document or a container, please specify only one of the following groups of properties: %s",
			n.FullName("/"), "(source/multiSource),(nodes,nodesSelector)"))
	}
	if n.NodeSelector != nil {
		if err := validateNodeSelector(n.NodeSelector, n.FullName("/")); err != nil {
			errs = multierror.Append(errs, err)
		}
	}
	if len(n.Nodes) > 0 {
		if err := validateSectionFile(n.Nodes); err != nil {
			errs = multierror.Append(errs, err)
		}
		for _, cn := range n.Nodes {
			if err := validateNode(cn); err != nil {
				errs = multierror.Append(errs, err)
			}
		}
	}
	for i, ms := range n.MultiSource {
		if ms == "" {
			errs = multierror.Append(errs, fmt.Errorf("node %s contains empty multiSource value at position %d", n.FullName("/"), i))
		}
	}

	return errs
}

// validateSectionFile ensures one section file per folder
func validateSectionFile(nodes []*Node) error {
	var errs error
	var idx, names []string
	for _, n := range nodes {
		if n.IsDocument() {
			// check 'index=true' property
			if len(n.Properties) > 0 {
				if val, found := n.Properties["index"]; found {
					if isIdx, ok := val.(bool); ok {
						if isIdx {
							idx = append(idx, n.FullName("/"))
							continue
						}
					}
				}
			}
			// check node name
			if n.Name == "_index.md" || n.Name == "_index" {
				names = append(names, n.FullName("/"))
			}
		}

	}
	if len(idx) > 1 {
		errs = multierror.Append(errs, fmt.Errorf("property index: true defined for multiple peer nodes: %s", strings.Join(idx, ",")))
	}
	if len(names) > 1 {
		errs = multierror.Append(errs, fmt.Errorf("_index.md defined for multiple peer nodes: %s", strings.Join(names, ",")))
	}
	if len(idx) == 1 && len(names) > 0 {
		errs = multierror.Append(errs, fmt.Errorf("index node %s collides with peer nodes: %s", idx[0], strings.Join(names, ",")))
	}
	return errs
}

// Serialize marshals the given documentation and transforms it to string
func Serialize(docs *Documentation) (string, error) {
	var (
		err error
		b   []byte
	)
	if b, err = yaml.Marshal(docs); err != nil {
		return "", err
	}
	return string(b), nil
}

// Collision a data type that represents a collision in parsing
type Collision struct {
	NodeParentPath string
	CollidedNodes  map[string][]string
}

// CheckForCollisions checks if a collision occured
func CheckForCollisions(nodes []*Node, options ParsingOptions) error {
	var (
		collisions []Collision
		err        error
	)

	collisions, err = deepCheckNodesForCollisions(nodes, nil, collisions, options)
	if err != nil {
		return err
	}
	if len(collisions) <= 0 {
		return nil
	}

	var sb strings.Builder
	sb.WriteString("Node collisions detected.")
	for _, collision := range collisions {
		sb.WriteString("\nIn ")
		sb.WriteString(collision.NodeParentPath)
		sb.WriteString(" container node.")
		for node, sources := range collision.CollidedNodes {
			sb.WriteString(" Node with name ")
			sb.WriteString(node)
			sb.WriteString(" appears ")
			sb.WriteString(fmt.Sprint(len(sources)))
			sb.WriteString(" times for sources: ")
			sb.WriteString(strings.Join(sources, ", "))
			sb.WriteString(".")
		}
	}
	return errors.New(sb.String())
}

func deepCheckNodesForCollisions(nodes []*Node, parent *Node, collisions []Collision, options ParsingOptions) ([]Collision, error) {
	var err error
	collisions, err = CheckNodesForCollision(nodes, parent, collisions, options)
	if err != nil {
		return nil, err
	}
	for _, node := range nodes {
		if len(node.Nodes) > 0 {
			collisions, err = deepCheckNodesForCollisions(node.Nodes, node, collisions, options)
			if err != nil {
				return nil, err
			}
		}
	}
	return collisions, nil
}

// CheckNodesForCollision given a set of nodes checks if there is a collision
func CheckNodesForCollision(nodes []*Node, parent *Node, collisions []Collision, options ParsingOptions) ([]Collision, error) {
	if len(nodes) < 2 {
		return collisions, nil
	}
	// It is unlikely to have a collision so keep the detection logic as simple and fast as possible.
	checked := make(map[string]struct{}, len(nodes))
	var collisionsNames []string
	for _, node := range nodes {
		nodeName, err := getNodeName(node, options.Hugo, options.ExtractedFilesFormats)
		if err != nil {
			return nil, err
		}
		if _, ok := checked[nodeName]; !ok {
			checked[nodeName] = struct{}{}
		} else {
			collisionsNames = append(collisionsNames, nodeName)
		}
	}

	if len(collisionsNames) == 0 {
		return collisions, nil
	}
	nodeCollisions, err := BuildNodeCollision(nodes, parent, collisionsNames, options)
	if err != nil {
		return nil, err
	}
	return append(collisions, *nodeCollisions), nil
}

func getNodeName(node *Node, hugoEnabled bool, supportedContentFormats []string) (string, error) {
	if node.IsDocument() {
		name := node.Name
		if node.Source != "" && len(node.MultiSource) > 0 {
			return "", fmt.Errorf("document node %s has a source and multisource property defined at the same time", node.FullName("/"))
		} else if node.Name == "" && len(node.MultiSource) > 0 {
			return "", fmt.Errorf("document node %s can't have a missing name and a defined multisource ", node.FullName("/"))
		} else if node.Name == "" && node.Source != "" {
			name = "$name$ext"
		}
		// name != "" and evaluate name expression
		if strings.IndexByte(name, '$') != -1 {
			info, err := util.BuildResourceInfo(node.Source)
			if err != nil {
				return "", err
			}
			ext := info.GetResourceExt()
			resourceName := strings.TrimSuffix(info.GetResourceName(), ext)
			name = strings.ReplaceAll(name, "$name", resourceName)
			name = strings.ReplaceAll(name, "$uuid", uuid.New().String())
			name = strings.ReplaceAll(name, "$ext", ext)
		}
		//check index=true
		if hugoEnabled && len(node.Properties) > 0 {
			if idxVal, found := node.Properties["index"]; found {
				idx, ok := idxVal.(bool)
				if ok && idx {
					name = "_index.md"
				}
			}
		}
		// ensure markdown suffix
		for _, suffix := range supportedContentFormats {
			if strings.HasSuffix(name, suffix) {
				return name, nil
			}
		}
		return fmt.Sprintf("%s.md", node.Name), nil
	}
	//node is container
	if node.Name == "" {
		return "", fmt.Errorf("container node %s should have a name", node.FullName("/"))
	}
	return node.Name, nil
}

// BuildNodeCollision builds the collision data type
func BuildNodeCollision(nodes []*Node, parent *Node, collisionsNames []string, options ParsingOptions) (*Collision, error) {
	c := Collision{
		NodeParentPath: GetNodeParentPath(parent),
		CollidedNodes:  make(map[string][]string, len(collisionsNames)),
	}

	for _, collisionName := range collisionsNames {
		for _, node := range nodes {
			nodeName, err := getNodeName(node, options.Hugo, options.ExtractedFilesFormats)
			if err != nil {
				return nil, err
			}
			if nodeName == collisionName {
				collidedNodes := c.CollidedNodes[nodeName]
				c.CollidedNodes[nodeName] = append(collidedNodes, node.Source)
			}
		}
	}

	return &c, nil
}

// GetNodeParentPath returns the node's parent path
func GetNodeParentPath(node *Node) string {
	if node == nil {
		return "root"
	}
	parents := node.Parents()
	var sb strings.Builder
	for _, child := range parents {
		sb.WriteString(child.Name)
		sb.WriteRune('.')
	}
	sb.WriteString(node.Name)
	return sb.String()
}
