// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"

	"github.com/hashicorp/go-multierror"
	"gopkg.in/yaml.v3"
)

// ParseWithMetadata parses a document's byte content given some other metainformation
func ParseWithMetadata(b []byte, targetBranch string, flagsVars map[string]string) (*Documentation, error) {
	versionList := make([]string, 0)
	versionList = append(versionList, targetBranch)

	versions := strings.Join(versionList, ",")
	flagsVars["versions"] = versions
	return Parse(b, flagsVars)
}

// Parse is a function which construct documentation struct from given byte array
func Parse(b []byte, flagsVars map[string]string) (*Documentation, error) {
	blob, err := resolveVariables(b, flagsVars)
	if err != nil {
		return nil, err
	}
	var docs = &Documentation{}
	if err = yaml.Unmarshal(blob, docs); err != nil {
		return nil, err
	}
	// init parents
	for _, n := range docs.Structure {
		n.SetParentsDownwards()
	}
	if err = validateDocumentation(docs); err != nil {
		return nil, err
	}

	return docs, nil
}

// TODO: CHECK FOR COLLISIONS
func validateDocumentation(d *Documentation) error {
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
	if n.IsDocument() && n.Source == "" && n.Name == "" { // TODO: Apply this check on container nodes as well, once all manifests are fixed
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
	// TODO: this is workaround to move child nodes in parent if this node name is empty, delete it once manifests are fixed !!!
	if n.Name == "" && !n.IsDocument() && n.Parent() != nil && n.Parent().NodeSelector == nil && n.NodeSelector != nil && len(n.Nodes) == 0 {
		n.Parent().NodeSelector = n.NodeSelector
		var pNodes []*Node
		for _, pn := range n.Parent().Nodes {
			if pn != n {
				pNodes = append(pNodes, pn)
			}
		}
		n.Parent().Nodes = pNodes
	}
	// TODO: end !!!
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

func resolveVariables(manifestContent []byte, vars map[string]string) ([]byte, error) {
	var (
		tmpl *template.Template
		err  error
		b    bytes.Buffer
	)
	tplFuncMap := make(template.FuncMap)
	tplFuncMap["Split"] = strings.Split
	tplFuncMap["Add"] = func(a, b int) int { return a + b }
	if tmpl, err = template.New("").Funcs(tplFuncMap).Parse(string(manifestContent)); err != nil {
		return nil, err
	}
	if err = tmpl.Execute(&b, vars); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
