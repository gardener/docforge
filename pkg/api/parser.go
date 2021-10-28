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

// SetVersions sets the mapping of repo uri to last n versions to be iterated over
func SetNVersions(flagNVersions map[string]int, configNVersions map[string]int) {
	flagVersionsMap = flagNVersions
	configVersionsMap = configNVersions
}

// SetMainBranches sets the mappinf of repo uri to name of the default branch
func SetDefaultBranches(flagBranches map[string]string, configBranches map[string]string) {
	flagBranchesMap = flagBranches
	configBranchesMap = configBranches
}

// ChoodeDefaultBranch chooses the default branch of the uri based on command variable, config file and repo default branch setup
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
	if tags, err = getLastNVersions(allTags, nTags); err != nil {
		return nil, err
	}
	versionList := make([]string, 0)
	versionList = append(versionList, targetBranch)
	versionList = append(versionList, tags...)

	versions := strings.Join(versionList, ",")
	flagsVars["versions"] = versions
	return Parse(b)
}

func getLastNVersions(tags []string, n int) ([]string, error) {
	if n < 0 {
		return nil, fmt.Errorf("n can't be negative")
	}
	versions := make([]*semver.Version, len(tags))
	latestVersions := make([]string, 0)
	//convert strings to versions
	for i, tag := range tags {
		version, err := semver.NewVersion(tag)
		if err != nil {
			return nil, fmt.Errorf("Error parsing version: %s", tag)
		}
		versions[i] = version
	}
	sort.Sort(semver.Collection(versions))
	//get last patch
	for i := 0; i < len(versions); i++ {
		upperBound, err := semver.NewConstraint("~" + versions[i].String())
		if err != nil {
			return nil, err
		}
		for i < len(versions) && upperBound.Check(versions[i]) {
			i++
		}
		latestVersions = append(latestVersions, versions[i-1].Original())
		i--
	}
	if n > len(latestVersions) {
		return nil, fmt.Errorf("number of tags is greater than the actual number of tags with latest patch:requested %d actual %d", n, len(latestVersions))
	}
	return latestVersions[len(latestVersions)-n:], nil
}

// Parse is ...
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

// Serialize is ...
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
	if tmpl, err = template.New("").Funcs(tplFuncMap).Parse(string(manifestContent)); err != nil {
		return nil, err
	}
	if err = tmpl.Execute(&b, vars); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
