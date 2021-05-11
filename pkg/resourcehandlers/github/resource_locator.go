// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package github

import (
	"fmt"
	"strings"
)

// ResourceType is an enumeration for GitHub resource types
// Supported types are "tree", "blob" and "wiki"
type ResourceType int

func (s ResourceType) String() string {
	return [...]string{"tree", "blob", "raw", "wiki", "releases", "issues", "issue", "pulls", "pull", "commit", "commits"}[s]
}

// NewResourceType creates a ResourceType enum from string
func NewResourceType(resourceTypeString string) (ResourceType, error) {
	switch resourceTypeString {
	case "tree":
		return Tree, nil
	case "blob":
		return Blob, nil
	case "raw":
		return Raw, nil
	case "wiki":
		return Wiki, nil
	case "releases":
		return Releases, nil
	case "issues":
		return Issues, nil
	case "issue":
		return Issue, nil
	case "pulls":
		return Pulls, nil
	case "pull":
		return Pull, nil
	case "commit":
		return Commit, nil
	case "commits":
		return Commit, nil
	}
	return 0, fmt.Errorf("unknown resource type string '%s'. Must be one of %v", resourceTypeString, []string{"tree", "blob", "raw", "wiki", "releases", "issues", "issue", "pulls", "pull", "commit", "commits"})
}

const (
	// Tree is GitHub tree objects resource type
	Tree ResourceType = iota
	// Blob is GitHub blob objects resource type
	Blob
	// Raw is GitHub raw resource type aw blob content
	Raw
	// Wiki is GitHub wiki resource type
	Wiki
	// Releases is GitHub releases resource type
	Releases
	// Issues is GitHub issues resource type
	Issues
	// Issue is GitHub issue resource type
	Issue
	// Pulls is GitHub pulls resource type
	Pulls
	// Pull is GitHub pull resource type
	Pull
	// Commit is GitHub commit resource type
	Commit
	// Commits is GitHub commits resource type
	Commits
)

// ResourceLocator is an abstraction for GitHub specific Universal Resource Locators (URLs)
// It is an internal structure breaking down the GitHub URLs into more segment types such as
// Repo, Owner or SHA.
// ResourceLocator is a common denominator used to translate between GitHub user-oriented urls
// and API urls
type ResourceLocator struct {
	Scheme string
	Host   string
	Owner  string
	Repo   string
	SHA    string
	Type   ResourceType
	Path   string
	// branch name (master), tag (v1.2.3), commit hash (1j4h4jh...)
	SHAAlias string
	// IsRawAPI is used to determine if the located resource has to be transformed to
	// a url of type https://host/raw/example/example during .String()
	IsRawAPI bool
}

// String produces a GitHub website link to a resource from a ResourceLocator.
// That's the format used to link а GitHub resource in the documentation structure and pages.
// Example: https://github.com/gardener/gardener/blob/master/docs/README.md
func (r *ResourceLocator) String() string {
	s := fmt.Sprintf("%s://%s", r.Scheme, r.Host)
	if r.IsRawAPI {
		return fmt.Sprintf("%s/raw/%s/%s/%s/%s", s, r.Owner, r.Repo, r.SHAAlias, r.Path)
	}
	// example: https://github.com/gardener
	s = fmt.Sprintf("%s/%s", s, r.Owner)
	if len(r.Repo) == 0 {
		return s
	}
	// example: https://raw.githubusercontent.com/gardener/gardener/master/logo/gardener-large.png
	if strings.HasPrefix(r.Host, "raw.") {
		return fmt.Sprintf("%s/%s/%s/%s", s, r.Repo, r.SHAAlias, r.Path)
	}

	// example: https://github.com/gardener/gardener
	if r.Type < 0 {
		return fmt.Sprintf("%s/%s", s, r.Repo)
	}
	s = fmt.Sprintf("%s/%s/%s", s, r.Repo, r.Type.String())
	if len(r.SHAAlias) > 0 && len(r.Path) > 0 {
		// example: https://github.com/gardener/gardener/blob/master/README.md
		// example: https://github.com/gardener/gardener/raw/master/logo/gardener-large.png
		return fmt.Sprintf("%s/%s/%s", s, r.SHAAlias, r.Path)
	}
	// example: https://github.com/gardener/gardener/releases/tag/v1.10.0
	if len(r.Path) > 0 {
		return fmt.Sprintf("%s/%s", s, r.Path)
	}
	// example: https://github.com/gardener/gardener/pulls
	return s
}

// GetRaw returns the raw content URL for this ResourceLocator if applicable.
// Only bloband raw resource locators qualify. An empty string is returned for all
// other resource type
func (r *ResourceLocator) GetRaw() string {
	switch r.Type {
	case Raw:
		return r.String()
	case Blob:
		{
			r.Type = Raw
			return r.String()
		}
	}
	return ""
}

// GetName returns the Name segment of a resource URL path
func (r *ResourceLocator) GetName() string {
	if len(r.Path) == 0 {
		return ""
	}
	p := strings.Split(r.Path, "/")
	return p[len(p)-1]
}
