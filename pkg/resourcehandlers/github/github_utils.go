// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package github

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/markdown"
	"github.com/gardener/docforge/pkg/resourcehandlers"

	"github.com/gardener/docforge/pkg/util/urls"

	ghclient "github.com/google/go-github/v32/github"
)

// ReadGitInfo implements resourcehandlers/ResourceHandler#ReadGitInfo
func ReadGitInfo(ctx context.Context, uri string, client *ghclient.Client) ([]byte, error) {
	var (
		rl      *ResourceLocator
		commits []*ghclient.RepositoryCommit
		err     error
		blob    []byte
	)
	if rl, err = Parse(uri); err != nil {
		return nil, err
	}
	opts := &ghclient.CommitsListOptions{
		Path: rl.Path,
		SHA:  rl.SHAAlias,
	}
	if commits, _, err = client.Repositories.ListCommits(ctx, rl.Owner, rl.Repo, opts); err != nil {
		return nil, err
	}
	if commits != nil {
		gitInfo := Transform(commits)
		if gitInfo == nil {
			return nil, nil
		}
		if len(rl.SHA) > 0 {
			gitInfo.SHA = &rl.SHA
		}
		if len(rl.SHAAlias) > 0 {
			gitInfo.SHAAlias = &rl.SHAAlias
		}
		if len(rl.Path) > 0 {
			gitInfo.Path = &rl.Path
		}
		if blob, err = MarshallGitInfo(gitInfo); err != nil {
			return nil, err
		}
	}

	return blob, nil
}

// Parse a GitHub URL into an incomplete ResourceLocator, without
// the SHA property.
func Parse(urlString string) (*ResourceLocator, error) {
	var (
		resourceType       ResourceType = -1
		repo               string
		path               string
		err                error
		resourceTypeString string
		shaAlias           string
		u                  *urls.URL
	)

	if u, err = urls.Parse(urlString); err != nil {
		return nil, err
	}

	host := u.Host
	sourceURLPathSegments := []string{}
	if len(u.Path) > 0 {
		// leading/trailing slashes
		_p := strings.TrimSuffix(u.Path[1:], "/")
		sourceURLPathSegments = strings.Split(_p, "/")
	}

	if len(sourceURLPathSegments) < 1 {
		return nil, fmt.Errorf("unsupported GitHub URL: %s. Need at least host and organization|owner", urlString)
	}

	var isRawAPI bool
	if "raw" == sourceURLPathSegments[0] {
		sourceURLPathSegments = sourceURLPathSegments[1:]
		isRawAPI = true
	}

	owner := sourceURLPathSegments[0]
	if len(sourceURLPathSegments) > 1 {
		repo = sourceURLPathSegments[1]
	}
	if len(sourceURLPathSegments) > 2 {
		// is this a raw.host content GitHub link?
		if isRawURL(u.URL) {
			resourceTypeString = "raw"
		} else {
			resourceTypeString = sourceURLPathSegments[2]
		}
		// {blob|tree|wiki|...}
		if resourceType, err = NewResourceType(resourceTypeString); err == nil {
			urlPathPrefix := strings.Join([]string{owner, repo, resourceTypeString}, "/")
			if isRawURL(u.URL) {
				// raw.host links have no resource type path segment
				urlPathPrefix = strings.Join([]string{owner, repo}, "/")
				shaAlias = sourceURLPathSegments[2]
			} else {
				// SHA aliases are defined only for blob/tree/raw objects
				if resourceType == Raw || resourceType == Blob || resourceType == Tree {
					// that would be wrong url but we make up for that
					if len(sourceURLPathSegments) < 4 {
						shaAlias = "master"
					} else {
						shaAlias = sourceURLPathSegments[3]
					}
				}
			}
			if len(shaAlias) > 0 {
				urlPathPrefix = strings.Join([]string{urlPathPrefix, shaAlias}, "/")
			}
			// get the github url "path" part without:
			// - leading "/"
			// - owner, repo, resource type, shaAlias segments if applicable
			if p := strings.Split(u.Path[1:], urlPathPrefix); len(p) > 1 {
				path = strings.TrimPrefix(p[1], "/")
			}
		}
		if err != nil {
			return nil, fmt.Errorf("unsupported GitHub URL: %s . %s", urlString, err.Error())
		}
	}
	if len(u.Fragment) > 0 {
		path = fmt.Sprintf("%s#%s", path, u.Fragment)
	}
	if len(u.RawQuery) > 0 {
		path = fmt.Sprintf("%s?%s", path, u.RawQuery)
	}
	ghRL := &ResourceLocator{
		Scheme:   u.Scheme,
		Host:     host,
		Owner:    owner,
		Repo:     repo,
		Type:     resourceType,
		Path:     path,
		SHAAlias: shaAlias,
		IsRawAPI: isRawAPI,
	}
	return ghRL, nil
}

func isRawURL(u *url.URL) bool {
	return strings.HasPrefix(u.Host, "raw.") || strings.HasPrefix(u.Path, "/raw")
}

// CleanupNodeTree cleanups node tree:
// - remove contentSources that reference tree objects. They are used
//   internally to build the structure but are not a valid contentSource
// - remove empty nodes that do not contain markdown. The build algorithm
//   is blind for the content of a node and leaves nodes that are folders
//   containing for example images only and thus irrelevant to the
//   documentation structure
func CleanupNodeTree(node *api.Node) {
	if len(node.Source) > 0 {
		source := node.Source
		if rl, _ := Parse(source); rl.Type == Tree {
			node.SetSourceLocation(node.Source)
			node.Source = ""
		}
	}
	for _, n := range node.Nodes {
		// skip nested unresolved nodeSelector nodes from cleanup
		if n.NodeSelector != nil && len(n.Nodes) == 0 {
			continue
		}
		CleanupNodeTree(n)
	}
	children := node.Nodes[:0]
	for _, n := range node.Nodes {
		if len(n.Nodes) != 0 || n.NodeSelector != nil || len(n.Source) != 0 {
			children = append(children, n)
		}
	}
	node.Nodes = children
}

var (
	defaultBranches map[string]string
	mux             sync.Mutex
)

// ClearDefaultBranchesCache used primary when testing
func ClearDefaultBranchesCache() {
	defaultBranches = nil
}

// GetDefaultBranch gets the default branch from a given recource handler
func GetDefaultBranch(ctx context.Context, client *ghclient.Client, rl *ResourceLocator) (string, error) {
	mux.Lock()
	defer mux.Unlock()

	if defaultBranches == nil {
		defaultBranches = make(map[string]string)
	}
	strRL := rl.String()
	if defaultBranch, ok := defaultBranches[strRL]; ok {
		return defaultBranch, nil
	}
	repo, _, err := client.Repositories.Get(ctx, rl.Owner, rl.Repo)
	if err != nil {
		return "", err
	}
	defaultBranch := repo.GetDefaultBranch()
	defaultBranches[strRL] = defaultBranch
	return defaultBranch, nil
}

// BaseResolveNodeSelector is the base function used when resolving node selectors
func BaseResolveNodeSelector(ctx context.Context, rl *ResourceLocator, rh resourcehandlers.ResourceHandler, cache *Cache, node *api.Node, excludePaths []string, frontMatter map[string]interface{}, excludeFrontMatter map[string]interface{}, depth int32) ([]*api.Node, error) {
	childResourceLocators, err := cache.GetSubsetWithInit(ctx, rl.String())
	if err != nil {
		return nil, err
	}

	childNodes, err := buildNodes(ctx, rh, cache, node, excludePaths, frontMatter, excludeFrontMatter, depth, childResourceLocators, 0)
	if err != nil {
		return nil, err
	}
	// finally, cleanup folder entries from contentSelectors
	for _, child := range childNodes {
		CleanupNodeTree(child)
	}
	if childNodes == nil {
		return []*api.Node{}, nil
	}

	return childNodes, nil
}

func buildNodes(ctx context.Context, rh resourcehandlers.ResourceHandler, cache *Cache, node *api.Node, excludePaths []string, frontMatter map[string]interface{}, excludeFrontMatter map[string]interface{}, depth int32, childResourceLocators []*ResourceLocator, currentDepth int32) ([]*api.Node, error) {
	var (
		nodesResult []*api.Node
		nodePath    string
	)
	if node.NodeSelector != nil {
		nodePath = node.NodeSelector.Path
	} else if len(node.Source) > 0 {
		nodePath = node.Source
	}
	nodePathRL, err := Parse(nodePath)
	if err != nil {
		return nil, err
	}
	//reformated
	nodeResourceLocator, err := cache.GetWithInit(ctx, nodePathRL)
	if nodeResourceLocator == nil || err != nil {
		panic(fmt.Sprintf("Node is not available as ResourceLocator %v: %v", nodePath, err))
	}
	nodePathSegmentsCount := len(strings.Split(nodeResourceLocator.Path, "/"))
	for _, childResourceLocator := range childResourceLocators {
		if !hasPathPrefix(childResourceLocator.Path, nodeResourceLocator.Path) {
			// invalid child. Why is it here?
			continue
		}
		// check if this resource path has to be excluded
		exclude := false
		for _, excludePath := range excludePaths {
			regex, err := regexp.Compile(excludePath)
			if err != nil {
				return nil, fmt.Errorf("invalid path exclude expression %s: %w", excludePath, err)
			}
			urlString := childResourceLocator.String()
			if regex.Match([]byte(urlString)) {
				exclude = true
				break
			}
		}
		if !exclude {
			childPathSegmentsCount := len(strings.Split(childResourceLocator.Path, "/"))
			// 1 sublevel only
			if (childPathSegmentsCount - nodePathSegmentsCount) == 1 {
				// folders and .md files only
				childName := childResourceLocator.GetName()
				if childResourceLocator.Type == Blob {
					if !strings.HasSuffix(strings.ToLower(childName), ".md") {
						//not a md file
						continue
					}
					// check for frontMatter filter compliance
					if frontMatter != nil || excludeFrontMatter != nil {
						// TODO: cache and reuse to avoid redundant reads when the structure nodes are processed
						b, err := rh.Read(ctx, childResourceLocator.String())
						if err != nil {
							return nil, err
						}
						selected, err := markdown.MatchFrontMatterRules(b, frontMatter, excludeFrontMatter)
						if err != nil {
							return nil, err
						}
						if !selected {
							continue
						}
					}
				}
				// creating new node
				nextNodeChild := &api.Node{
					Name:   childName,
					Source: childResourceLocator.String(),
				}
				nextNodeChild.SetParent(node)
				// recursively build subnodes if entry is sub-tree
				if childResourceLocator.Type == Tree {
					if depth > 0 && depth == currentDepth {
						continue
					}
					currentDepth++
					if childResourceLocators, err = cache.GetSubsetWithInit(ctx, childResourceLocator.String()); err != nil {
						return nil, err
					}
					childNodes, err := buildNodes(ctx, rh, cache, nextNodeChild, excludePaths, frontMatter, excludeFrontMatter, depth, childResourceLocators, currentDepth)
					if err != nil {
						return nil, err
					}
					if nextNodeChild.Nodes == nil {
						nextNodeChild.Nodes = make([]*api.Node, 0)
					}
					nextNodeChild.Nodes = append(nextNodeChild.Nodes, childNodes...)
					currentDepth--
				}
				nodesResult = append(nodesResult, nextNodeChild)
			}
		}
	}
	return nodesResult, nil
}
