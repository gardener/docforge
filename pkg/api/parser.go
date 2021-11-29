// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"fmt"
	"html/template"
	"sort"
	"strings"

	"github.com/Masterminds/semver"
	"gopkg.in/yaml.v3"
)

// flagsVars variables for template resolving
var (
	flagsVars         map[string]string
	flagVersionsMap   map[string]int
	configVersionsMap map[string]int
	flagBranchesMap   map[string]string
	configBranchesMap map[string]string
)

// SetFlagsVariables initialize flags variables
func SetFlagsVariables(vars map[string]string) {
	flagsVars = vars
}

// SetNVersions sets the mapping of repo uri to last n versions to be iterated over
func SetNVersions(flagNVersions map[string]int, configNVersions map[string]int) {
	flagVersionsMap = flagNVersions
	configVersionsMap = configNVersions
}

// SetDefaultBranches sets the mappinf of repo uri to name of the default branch
func SetDefaultBranches(flagBranches map[string]string, configBranches map[string]string) {
	flagBranchesMap = flagBranches
	configBranchesMap = configBranches
}

// ChooseTargetBranch chooses the default branch of the uri based on command variable, config file and repo default branch setup
func ChooseTargetBranch(uri string, repoCurrentBranch string) string {
	var (
		targetBranch string
		ok           bool
	)
	//choosing default branch
	if targetBranch, ok = flagBranchesMap[uri]; !ok {
		if targetBranch, ok = configBranchesMap[uri]; !ok {
			if targetBranch, ok = flagBranchesMap["default"]; !ok {
				targetBranch = repoCurrentBranch
			}
		}
	}
	return targetBranch
}

// ChooseNVersions chooses how many versions to be iterated over given a repo uri
func ChooseNVersions(uri string) int {
	var (
		nTags int
		ok    bool
	)
	//setting nTags
	if nTags, ok = flagVersionsMap[uri]; !ok {
		if nTags, ok = configVersionsMap[uri]; !ok {
			if nTags, ok = flagVersionsMap["default"]; !ok {
				nTags = 0
			}
		}
	}
	return nTags
}

// ParseWithMetadata parses a document's byte content given some other metainformation
func ParseWithMetadata(b []byte, allTags []string, nTags int, targetBranch string) (*Documentation, error) {
	var (
		err  error
		tags []string
	)
	if tags, err = GetLastNVersions(allTags, nTags); err != nil {
		return nil, err
	}
	versionList := make([]string, 0)
	versionList = append(versionList, targetBranch)
	versionList = append(versionList, tags...)

	versions := strings.Join(versionList, ",")
	flagsVars["versions"] = versions
	return Parse(b)
}

// GetLastNVersions returns only the last patch version for each major and minor version
func GetLastNVersions(tags []string, n int) ([]string, error) {
	if n < 0 {
		return nil, fmt.Errorf("n can't be negative")
	} else if n == 0 {
		return []string{}, nil
	}

	if len(tags) == 0 {
		return nil, fmt.Errorf("number of tags is greater than the actual number of all tags: wanted - %d, actual - %d", n, len(tags))
	}

	versions := make([]*semver.Version, len(tags))
	//convert strings to versions
	for i, tag := range tags {
		version, err := semver.NewVersion(tag)
		if err != nil {
			return nil, fmt.Errorf("Error parsing version: %s", tag)
		}
		versions[i] = version
	}
	sort.Sort(sort.Reverse(semver.Collection(versions)))

	//get last patches of the last n major versions
	latestVersions := make([]string, 0)
	firstVersion := versions[0]
	latestVersions = append(latestVersions, firstVersion.Original())

	constaint, err := semver.NewVersion(fmt.Sprintf("%d.%d", firstVersion.Major(), firstVersion.Minor()))
	if err != nil {
		return nil, err
	}
	for i := 1; i < len(versions) && len(latestVersions) < n; i++ {
		if versions[i].LessThan(constaint) {
			latestVersions = append(latestVersions, versions[i].Original())
			if constaint, err = semver.NewVersion(fmt.Sprintf("%d.%d", versions[i].Major(), versions[i].Minor())); err != nil {
				return nil, err
			}
		}
	}
	if n > len(latestVersions) {
		return nil, fmt.Errorf("number of tags is greater than the actual number of all tags: wanted - %d, actual - %d", n, len(latestVersions))
	}
	return latestVersions, nil
}

// Parse is a function which construct documentation struct from given byte array
func Parse(b []byte) (*Documentation, error) {
	blob, err := resolveVariables(b, flagsVars)
	if err != nil {
		return nil, err
	}
	var docs = &Documentation{}
	if err = yaml.Unmarshal(blob, docs); err != nil {
		return nil, err
	}

	if err = validateDocumentation(docs); err != nil {
		return nil, err
	}

	return docs, nil
}

func validateDocumentation(d *Documentation) error {
	var err error

	if d.Structure == nil && d.NodeSelector == nil {
		err = fmt.Errorf("the document structure must contains at least one of these propperties: structure, nodesSelector")
		return err
	}

	if d.NodeSelector != nil && d.NodeSelector.Path == "" {
		err = fmt.Errorf("the document structure must always contains path property in the nodesSelector")
		return err
	}

	allNodes := getAllNodes(d.Structure)
	for _, node := range allNodes {
		if node.isDocument() && node.Source == "" && node.Name == "" {
			err = fmt.Errorf("document node must contains at least one of these properties: source, name. node: %+v", node)
			return err
		}

		if node.Source == "" && node.NodeSelector == nil && node.ContentSelectors == nil && node.Nodes == nil && node.Template == nil {
			err = fmt.Errorf("node must contains at least one of these propperties: source, nodesSelector, contentsSelector, template, nodes. node: %+v", node)
			return err
		}

		if node.isDocument() && (node.Nodes != nil || node.NodeSelector != nil) {
			err = fmt.Errorf("node must be categorized as a document or a container node. Please specify only one of the following groups of propperties: %s, for node: %+v", "(source/contentSelector/Template),(nodes,nodesSelector)", node)
			return err
		}

		if node.NodeSelector != nil && node.NodeSelector.Path == "" {
			err = fmt.Errorf("document nodesSelector %+v must always contains a path property", node.NodeSelector)
			return err
		}

		contentSelectors := make([]ContentSelector, 0)
		if node.ContentSelectors != nil {
			contentSelectors = append(contentSelectors, node.ContentSelectors...)
		}

		if node.Template != nil {
			if node.Template.Path == "" {
				err = fmt.Errorf("node template must always contains a path property. node: %+v", node)
				return err
			}

			for key, cs := range node.Template.Sources {
				if key == "" {
					err = fmt.Errorf("the key of a template selector must not be empty. node: %+v", node)
					return err
				}

				if cs == nil {
					err = fmt.Errorf("template must always contains a map of contentSelectors. node: %+v", node)
					return err
				}

				contentSelectors = append(contentSelectors, *cs)
			}
		}

		for _, cs := range contentSelectors {
			if cs.Source == "" {
				err = fmt.Errorf("contentSelector must always contains a source property. node: %+v", node)
				return err
			}

			if cs.Selector != nil {
				err = fmt.Errorf("selector property is not supported in the ContentSelector. node: %+v", node)
				return err
			}
		}
	}

	return nil
}

func getAllNodes(currentNodes []*Node) []*Node {
	allNodes := make([]*Node, 0)

	for _, node := range currentNodes {
		allNodes = append(allNodes, node)
		allNodes = append(allNodes, getAllNodes(node.Nodes)...)
	}

	return allNodes
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
