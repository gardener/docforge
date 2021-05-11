// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package github

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"k8s.io/klog/v2"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/markdown"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/util/urls"
	"github.com/google/go-github/v32/github"
)

var (
	reHasTree = regexp.MustCompile("^(.*?)tree(.*)$")
)

// TreeEntryToGitHubLocator creates a ResourceLocator from a github.TreeEntry and shaAlias.
// The shaAlias is a ref i.e. the name of e.g. a branch or a tag that should resolve to this resource
// in the git database. It binds the formats of a GitHub website URLs to the GitHub API URLs.
//
// Example tree entries:
//{
//	"path": "docs",
//	"mode": "040000",
//	"type": "tree",
//	"sha": "5e11bda664b234920d85db5ca10055916c11e35d",
//	"url": "https://api.github.com/repos/gardener/gardener/git/trees/5e11bda664b234920d85db5ca10055916c11e35d"
//}
// Example blob:
//{
//	"path": "docs/README.md",
//	"mode": "100644",
//	"type": "blob",
//  "size": "6260"
//	"sha": "91776959202ec10db883c5cfc05c51e78403f02c",
//	"url": "https://api.github.com/repos/gardener/gardener/git/blobs/91776959202ec10db883c5cfc05c51e78403f02c"
//}
func TreeEntryToGitHubLocator(treeEntry *github.TreeEntry, shaAlias string) *ResourceLocator {
	// Tree entries such as (submodule) `commit` objects do not have URL
	// and cannot be converted to ResourceLocator
	if treeEntry.URL == nil {
		return nil
	}
	url, err := url.Parse(treeEntry.GetURL())
	if err != nil {
		panic(fmt.Sprintf("failed to parse url %v: %v", treeEntry.GetURL(), err))
	}

	sourceURLSegments := strings.Split(url.Path, "/")
	owner := sourceURLSegments[2]
	repo := sourceURLSegments[3]
	host := url.Host
	if host != "api.github.com" {
		owner = sourceURLSegments[4]
		repo = sourceURLSegments[5]
	} else {
		host = "github.com"
	}

	resourceType, err := NewResourceType(treeEntry.GetType())
	if err != nil {
		panic(fmt.Sprintf("unexpected resource type %v: %v", treeEntry.GetType(), err))
	}
	return &ResourceLocator{
		Scheme:   url.Scheme,
		Host:     host,
		Owner:    owner,
		Path:     treeEntry.GetPath(),
		Type:     resourceType,
		Repo:     repo,
		SHA:      treeEntry.GetSHA(),
		SHAAlias: shaAlias,
	}
}

// Recursively adds or merges nodes built from flat ResourceLocators list to node.Nodes
func (gh *GitHub) buildNodes(ctx context.Context, node *api.Node, excludePaths []string, frontMatter map[string]interface{}, excludeFrontMatter map[string]interface{}, depth int32, childResourceLocators []*ResourceLocator, currentDepth int32) ([]*api.Node, error) {
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
	nodeResourceLocator, err := gh.cache.Get(nodePathRL)
	if nodeResourceLocator == nil || err != nil {
		panic(fmt.Sprintf("Node is not available as ResourceLocator %v: %v", nodePath, err))
	}
	nodePathSegmentsCount := len(strings.Split(nodeResourceLocator.Path, "/"))
	for _, childResourceLocator := range childResourceLocators {
		if !strings.HasPrefix(childResourceLocator.Path, nodeResourceLocator.Path) {
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
			childName := childResourceLocator.GetName()
			// 1 sublevel only
			if (childPathSegmentsCount - nodePathSegmentsCount) == 1 {
				// folders and .md files only
				if childResourceLocator.Type == Blob {
					if !strings.HasSuffix(strings.ToLower(childName), ".md") {
						continue
					}
					// check for frontMatter filter compliance
					if frontMatter != nil || excludeFrontMatter != nil {
						// TODO: cache and reuse to avoid redundant reads when the structure nodes are processed
						b, err := gh.Read(ctx, childResourceLocator.String())
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
				n := &api.Node{
					Name:   childName,
					Source: childResourceLocator.String(),
				}
				n.SetParent(node)
				// recursively build subnodes if entry is sub-tree
				if childResourceLocator.Type == Tree {
					if depth > 0 && depth == currentDepth {
						continue
					}
					currentDepth++
					if childResourceLocators, err = gh.cache.GetSubset(childResourceLocator.String()); err != nil {
						return nil, err
					}
					childNodes, err := gh.buildNodes(ctx, n, excludePaths, frontMatter, excludeFrontMatter, depth, childResourceLocators, currentDepth)
					if err != nil {
						return nil, err
					}
					if n.Nodes == nil {
						n.Nodes = make([]*api.Node, 0)
					}
					n.Nodes = append(n.Nodes, childNodes...)
					currentDepth--
				}
				nodesResult = append(nodesResult, n)
			}
		}
	}
	return nodesResult, nil
}

// - remove contentSources that reference tree objects. They are used
//   internally to build the structure but are not a valid contentSource
// - remove empty nodes that do not contain markdown. The build algorithm
//   is blind for the content of a node and leaves nodes that are folders
//   containing for example images only and thus irrelevant to the
//   documentation structure
func cleanupNodeTree(node *api.Node) {
	if len(node.Source) > 0 {
		source := node.Source
		if rl, _ := Parse(source); rl.Type == Tree {
			node.Source = ""
		}
	}
	for _, n := range node.Nodes {
		// skip nested unresolved nodeSelector nodes from cleanup
		if n.NodeSelector != nil && len(n.Nodes) == 0 {
			continue
		}
		cleanupNodeTree(n)
	}
	children := node.Nodes[:0]
	for _, n := range node.Nodes {
		if len(n.Nodes) != 0 || n.NodeSelector != nil || len(n.Source) != 0 {
			children = append(children, n)
		}
	}
	node.Nodes = children
}

// HasURLPrefix returns true if pathPrefix tests true as prefix for path,
// either with tree or blob in its resource type segment
// The resource type in the URL prefix {tree|blob} changes according to the resource
// To keep the prefix valid it should alternate this path segment too.
func HasURLPrefix(path, pathPrefix string) bool {
	repStr := "${1}blob$2"
	pathPrefixAsBlob := pathPrefix
	pathPrefixAsBlob = reHasTree.ReplaceAllString(pathPrefixAsBlob, repStr)
	return strings.HasPrefix(path, pathPrefix) || strings.HasPrefix(path, pathPrefixAsBlob)
}

// GitHub implements resourcehandlers/ResourceHandler
type GitHub struct {
	Client               *github.Client
	cache                *Cache
	acceptedHosts        []string
	rawusercontentClient *http.Client
}

// NewResourceHandler creates new GitHub ResourceHandler objects
func NewResourceHandler(client *github.Client, acceptedHosts []string) resourcehandlers.ResourceHandler {
	tr := &http.Transport{
		MaxIdleConns:       10,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: true,
	}

	return &GitHub{
		Client: client,
		cache: &Cache{
			cache: map[string]*ResourceLocator{},
		},
		acceptedHosts:        acceptedHosts,
		rawusercontentClient: &http.Client{Transport: tr},
	}
}

// URLToGitHubLocator produces a ResourceLocator from a GitHub website URL.
// ResourceLocator is the internal format used to dereference GitHub website
// links from documentation structure specification and documents.
//
// Examples:
// - https://github.com/gardener/gardener/tree/master/docs
// - https://github.com/gardener/gardener/blob/master/docs/README.md
// - https://raw.githubusercontent.com/gardener/docforge/master/README.md
// - https://github.com/gardener/docforge/blob/master/README.md?raw=true
// - https://github.enterprise/org/repo/blob/master/docs/img/image.png?raw=true
// - https://github.enterprise/raw/org/repo/master/docs/img/image.png
// - https://raw.github.enterprise/org/repo/master/docs/img/img.png
// If resolveAPIUrl is true, GitHub API will be queried to populate the API URL for
// that resource (its SHA cannot be inferred from the url). If it's false the APIUrl
// property will be nil. In this case ctx can be omitted too.
func (gh *GitHub) URLToGitHubLocator(ctx context.Context, urlString string, resolveAPIUrl bool) (*ResourceLocator, error) {
	var (
		ghRL *ResourceLocator
		err  error
	)
	if ghRL, err = Parse(urlString); err != nil {
		return nil, err
	}
	if ghRL.Type == Wiki || len(ghRL.SHAAlias) == 0 {
		return ghRL, nil
	}
	cachedRL, err := gh.cache.Get(ghRL)
	if err != nil {
		return nil, err
	}
	if cachedRL != nil {
		return cachedRL, nil
	}
	if resolveAPIUrl {
		// grab the index of this repo
		gitTree, resp, err := gh.Client.Git.GetTree(ctx, ghRL.Owner, ghRL.Repo, ghRL.SHAAlias, true)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode > 399 {
			return nil, fmt.Errorf("request for %s failed: %s", urlString, resp.Status)
		}
		// populate cache with this tree entries
		for _, entry := range gitTree.Entries {
			rl := TreeEntryToGitHubLocator(entry, ghRL.SHAAlias)
			gh.cache.Set(rl)
		}
		cachedRL, err := gh.cache.Get(ghRL)
		if err != nil {
			return nil, err
		}
		if cachedRL == nil {
			return nil, resourcehandlers.ErrResourceNotFound(urlString)
		}
		return cachedRL, nil
	}
	return ghRL, nil
}

// Accept implements resourcehandlers/ResourceHandler#Accept
func (gh *GitHub) Accept(uri string) bool {
	var (
		url *url.URL
		err error
	)
	if gh.acceptedHosts == nil {
		return false
	}
	// Quick sanity check, preventing panic when trying to
	// resolve relative paths in url.Parse
	if !strings.HasPrefix(uri, "http") {
		return false
	}
	if url, err = url.Parse(uri); err != nil {
		return false
	}
	// check if this is a GitHub URL
	if rl, err := Parse(uri); rl == nil || err != nil {
		return false
	}
	for _, s := range gh.acceptedHosts {
		if url.Host == s {
			return true
		}
	}
	return false
}

// ResolveNodeSelector recursively adds nodes built from tree entries to node
// ResolveNodeSelector implements resourcehandlers/ResourceHandler#ResolveNodeSelector
func (gh *GitHub) ResolveNodeSelector(ctx context.Context, node *api.Node, excludePaths []string, frontMatter map[string]interface{}, excludeFrontMatter map[string]interface{}, depth int32) ([]*api.Node, error) {
	rl, err := gh.URLToGitHubLocator(ctx, node.NodeSelector.Path, true)
	if err != nil {
		if _, ok := err.(resourcehandlers.ErrResourceNotFound); ok {
			return []*api.Node{}, nil
		}
		return nil, err
	}

	childResourceLocators, err := gh.cache.GetSubset(rl.String())
	if err != nil {
		return nil, err
	}
	childNodes, err := gh.buildNodes(ctx, node, excludePaths, frontMatter, excludeFrontMatter, depth, childResourceLocators, 0)
	if err != nil {
		return nil, err
	}
	// finally cleanup folder entries from contentSelectors
	for _, child := range childNodes {
		cleanupNodeTree(child)
	}
	if childNodes == nil {
		return []*api.Node{}, nil
	}

	return childNodes, nil
}

// ResolveDocumentation for a given path and return it as a *api.Documentation
func (gh *GitHub) ResolveDocumentation(ctx context.Context, path string) (*api.Documentation, error) {
	rl, err := gh.URLToGitHubLocator(ctx, path, true)
	if err != nil {
		return nil, err
	}
	// TODO: In cases where nodesSelector.Path is set to an url poiting to a resource with .md extension, it's
	// considered as invalid. This is to avoid downloading the resource twice. Contemplate logic that caches
	// the resource once read for later downloads.
	if !(rl.Type == Blob || rl.Type == Raw) || urls.Ext(rl.String()) == ".md" {
		return nil, nil
	}

	blob, err := gh.Read(ctx, rl.String())
	if err != nil {
		return nil, err
	}

	return api.Parse(blob)
}

// Read implements resourcehandlers/ResourceHandler#Read
func (gh *GitHub) Read(ctx context.Context, uri string) ([]byte, error) {
	var (
		blob []byte
		rl   *ResourceLocator
		err  error
	)
	if rl, err = gh.URLToGitHubLocator(ctx, uri, true); err != nil {
		return nil, err
	}
	if rl != nil {
		switch rl.Type {
		case Blob:
			{
				blob, _, err = gh.Client.Git.GetBlobRaw(ctx, rl.Owner, rl.Repo, rl.SHA)
				if err != nil {
					return nil, err
				}
			}
		case Wiki:
			{
				wikiPage := rl.String()
				if !strings.HasSuffix(wikiPage, ".md") {
					wikiPage = fmt.Sprintf("%s.%s", wikiPage, "md")
				}
				resp, err := gh.rawusercontentClient.Get(wikiPage)
				if err != nil {
					return nil, err
				}
				defer resp.Body.Close()
				var hasContentTypeRaw bool
				for _, ct := range resp.Header["Content-Type"] {
					if strings.Contains(ct, "text/plain") {
						hasContentTypeRaw = true
						break
					}
				}
				if !hasContentTypeRaw {
					return nil, fmt.Errorf("Request for resource content to %s returned unexpected content type for wiki raw content: %s", rl.String(), resp.Header["Content-Type"])
				}
				return ioutil.ReadAll(resp.Body)
			}
		case Tree:
			{
				klog.Warningf("Attempted to read tree object from GitHub: %s. Only wiki pages and blob URLs are supported", rl.String())
			}
		}
	}
	return blob, err
}

// ReadGitInfo implements resourcehandlers/ResourceHandler#ReadGitInfo
func (gh *GitHub) ReadGitInfo(ctx context.Context, uri string) ([]byte, error) {
	var (
		rl      *ResourceLocator
		commits []*github.RepositoryCommit
		err     error
		blob    []byte
	)
	if rl, err = Parse(uri); err != nil {
		return nil, err
	}
	opts := &github.CommitsListOptions{
		Path: rl.Path,
		SHA:  rl.SHAAlias,
	}
	if commits, _, err = gh.Client.Repositories.ListCommits(ctx, rl.Owner, rl.Repo, opts); err != nil {
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

// ResourceName implements resourcehandlers/ResourceHandler#ResourceName
func (gh *GitHub) ResourceName(uri string) (string, string) {
	var (
		rl  *ResourceLocator
		err error
	)
	if rl, err = gh.URLToGitHubLocator(context.TODO(), uri, false); err != nil {
		panic(err)
	}
	if rl == nil {
		panic(errors.New(uri))
	}
	if rl != nil {
		if u, err := urls.Parse(rl.String()); err == nil {
			return u.ResourceName, u.Extension
		}
	}
	return "", ""
}

// BuildAbsLink builds the abs link from the source and the relative path
// Implements resourcehandlers/ResourceHandler#BuildAbsLink
func (gh *GitHub) BuildAbsLink(source, relPath string) (string, error) {
	u, err := url.Parse(relPath)
	if err != nil {
		return "", err
	}
	if u.IsAbs() {
		return relPath, nil
	}

	u, err = url.Parse(source)
	if err != nil {
		return "", err
	}
	u, err = u.Parse(relPath)
	if err != nil {
		return "", err
	}
	return u.String(), err
}

// SetVersion replaces the version segment in the path of GitHub URLs if
// applicable or returns the original URL unchanged if not.
// Implements resourcehandlers/ResourceHandler#SetVersion
func (gh *GitHub) SetVersion(absLink, version string) (string, error) {
	var (
		rl  *ResourceLocator
		err error
	)
	if rl, err = Parse(absLink); err != nil {
		return "", err
	}

	if len(rl.SHAAlias) > 0 {
		rl.SHAAlias = version
		return rl.String(), nil
	}

	return absLink, nil
}

// GetRawFormatLink implements ResourceHandler#GetRawFormatLink
func (gh *GitHub) GetRawFormatLink(absLink string) (string, error) {
	var (
		rl  *ResourceLocator
		err error
	)
	if rl, err = Parse(absLink); err != nil {
		return "", err
	}
	if l := rl.GetRaw(); len(l) > 0 {
		return l, nil
	}
	return absLink, nil
}

func (gh *GitHub) TreeExists(ctx context.Context, absLink string) (bool, error) {
	ghLocator, err := gh.URLToGitHubLocator(ctx, absLink, false)
	if err != nil {
		return false, err
	}

	if ghLocator != nil && ghLocator.Type == Tree {
		ghTrees, response, err := gh.Client.Git.GetTree(ctx, ghLocator.Owner, ghLocator.Repo, ghLocator.SHA, false)
		if err != nil {
			// return the the error if the response object is nil or the status code is different from 401
			if response == nil || response.StatusCode != http.StatusNotFound {
				return false, err
			}
		}
		if response.StatusCode == http.StatusOK && ghTrees != nil {
			return true, nil
		}

	}
	return false, nil
}
