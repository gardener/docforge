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

//parse with tags and bytes as read
//fsHandled used to display warning
//uri used to get the proper main branch and versions
func ParseWithMetadata(tags []string, b []byte, fsHandled bool, uri string, defaultBranch *string) (*Documentation, error) {
	var (
		nTags      int
		err        error
		mainBranch string
		ok         bool
	)
	//setting main branch
	if mainBranch, ok = mainBranches[uri]; !ok {
		if mainBranch, ok = mainBranches["default"]; !ok {
			if fsHandled {
				mainBranch = "master"
			} else {
				mainBranch = *defaultBranch
			}
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
