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
	"k8s.io/klog/v2"
)

// flagsVars variables for template resolving
var (
	flagsVars    map[string]string
	versions     map[string]int
	mainBranches map[string]string
)

// SetFlagsVariables initialize flags variables
func SetFlagsVariables(vars map[string]string) {
	flagsVars = vars
}

// SetVersions sets the mapping of repo uri to last n versions to be iterated over
func SetVersions(vers map[string]int) {
	versions = vers
}

// SetMainBranches sets the mappinf of repo uri to name of the default branch
func SetMainBranches(mb map[string]string) {
	mainBranches = mb
}

// ParseWithMetadata parses the byte array and list of tags received as parameters and constructs a documentation structurep
func ParseWithMetadata(tags []string, b []byte, fsHandled bool, uri string) (*Documentation, error) {
	var (
		nTags      int
		err        error
		mainBranch string
		ok         bool
	)
	//setting main branch
	if mainBranch, ok = mainBranches[uri]; !ok {
		if mainBranch, ok = mainBranches["default"]; !ok {
			mainBranch = "master"
		}
	}
	//setting nTags
	if nTags, ok = versions[uri]; !ok {
		if nTags, ok = versions["default"]; !ok {
			nTags = 0
		}
	}
	if nTags > 0 && fsHandled {
		klog.Warningf("There is a yaml file from file system not connected with a repository. Therefore LastNSupportedVersions is set to 0 %s", uri)
		nTags = 0
	}
	if tags, err = getLastNVersions(tags, nTags); err != nil {
		return nil, err
	}
	versionList := make([]string, 0)
	versionList = append(versionList, mainBranch)
	versionList = append(versionList, tags...)
	versions := strings.Join(versionList, ",")
	flagsVars["versions"] = versions
	return Parse(b)
}

func getLastNVersions(tags []string, n int) ([]string, error) {
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
		return nil, fmt.Errorf("number of tags is greater than the actual number of tags with latest patch:requested %d actual %d", n, len(latestVersions))
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
	return docs, nil
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
