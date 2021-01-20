// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package github

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"k8s.io/klog/v2"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/util/urls"
	"github.com/google/go-github/v32/github"
)

var (
	reHasTree = regexp.MustCompile("^(.*?)tree(.*)$")
)

// TreeEntryToGitHubLocator creates a ResourceLocator from a github.TreeEntry and shaAlias.
// The shaAlias is the name of e.g. a branch or a tag that should resolve to this resource
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
	url, err := url.Parse(treeEntry.GetURL())
	if err != nil {
		panic(fmt.Sprintf("failed to parse url %v: %v", treeEntry.GetURL(), err))
	}

	sourceURLSegments := strings.Split(url.Path, "/")
	owner := sourceURLSegments[2]
	repo := sourceURLSegments[3]

	if url.Host != "api.github.com" {
		owner = sourceURLSegments[4]
		repo = sourceURLSegments[5]
	}

	resourceType, err := NewResourceType(treeEntry.GetType())
	if err != nil {
		panic(fmt.Sprintf("unexpected resource type %v: %v", treeEntry.GetType(), err))
	}
	return &ResourceLocator{
		Scheme:   url.Scheme,
		Host:     url.Host,
		Owner:    owner,
		Path:     treeEntry.GetPath(),
		Type:     resourceType,
		Repo:     repo,
		SHA:      treeEntry.GetSHA(),
		SHAAlias: shaAlias,
	}
}

// Recursively adds or merges nodes built from flat ResourceLocators list to node.Nodes
func buildNodes(node *api.Node, excludePaths []string, frontMatter map[string]interface{}, excludeFrontMatter map[string]interface{}, depth int32, childResourceLocators []*ResourceLocator, cache *Cache, currentDepth int32) ([]*api.Node, error) {
	var (
		nodesResult         []*api.Node
		nodePath            string
		nodeResourceLocator *ResourceLocator
	)
	if node.NodeSelector != nil {
		nodePath = node.NodeSelector.Path
	} else if len(node.Source) > 0 {
		nodePath = node.Source
	}
	if nodeResourceLocator = cache.Get(nodePath); nodeResourceLocator == nil {
		panic(fmt.Sprintf("Node is not available as ResourceLocator %v", nodePath))
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
				if childResourceLocator.Type == Blob && !strings.HasSuffix(strings.ToLower(childName), ".md") {
					continue
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
					childResourceLocators = cache.GetSubset(childResourceLocator.String())
					childNodes, err := buildNodes(n, excludePaths, frontMatter, excludeFrontMatter, depth, childResourceLocators, cache, currentDepth)
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
		if rl, _ := parse(source); rl.Type == Tree {
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

// Cache is indexes GitHub TreeEntries by website resource URLs as keys,
// mapping ResourceLocator objects to them.
// TODO: implement me efficiently and for parallel use
type Cache struct {
	cache map[string]*ResourceLocator
	mux   sync.Mutex
}

// Get returns a ResourceLocator object mapped to the path (URL)
func (c *Cache) Get(path string) *ResourceLocator {
	defer c.mux.Unlock()
	c.mux.Lock()
	return c.cache[path]
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

// GetSubset returns a subset of the ResourceLocator objects mapped to keys
// with this pathPrefix
func (c *Cache) GetSubset(pathPrefix string) []*ResourceLocator {
	defer c.mux.Unlock()
	c.mux.Lock()
	var entries = make([]*ResourceLocator, 0)
	for k, v := range c.cache {
		if k == pathPrefix {
			continue
		}
		if HasURLPrefix(k, pathPrefix) {
			entries = append(entries, v)
		}
	}
	return entries
}

// Set adds a mapping between a path (URL) and a ResourceLocator to the cache
func (c *Cache) Set(path string, entry *ResourceLocator) *ResourceLocator {
	defer c.mux.Unlock()
	c.mux.Lock()
	c.cache[path] = entry
	return entry
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
//
// If resolveAPIUrl is true, GitHub API will be queried to populate the API URL for
// that resource (its SHA cannot be inferred from the url). If it's false the APIUrl
// property will be nil. In this case ctx can be omitted too.
func (gh *GitHub) URLToGitHubLocator(ctx context.Context, urlString string, resolveAPIUrl bool) (*ResourceLocator, error) {
	var (
		ghRL *ResourceLocator
		err  error
	)
	// try cache first
	if ghRL = gh.cache.Get(urlString); ghRL == nil {
		if ghRL, err = parse(urlString); err != nil {
			return nil, err
		}
		if ghRL.Type != Wiki && resolveAPIUrl {
			if len(ghRL.SHAAlias) == 0 {
				return ghRL, nil
			}
			// grab the index of this repo
			gitTree, resp, err := gh.Client.Git.GetTree(ctx, ghRL.Owner, ghRL.Repo, ghRL.SHAAlias, true)
			if err != nil {
				return nil, err
			}
			if resp.StatusCode > 399 {
				return nil, fmt.Errorf("request for %s failed: %s", urlString, resp.Status)
			}
			// populate cache wth this tree entries
			for _, entry := range gitTree.Entries {
				rl := TreeEntryToGitHubLocator(entry, ghRL.SHAAlias)
				rl.Host = ghRL.Host
				rl.IsRawAPI = ghRL.IsRawAPI
				gh.cache.Set(rl.String(), rl)
			}
			ghRL = gh.cache.Get(urlString)
			if ghRL == nil {
				return nil, resourcehandlers.ErrResourceNotFound
			}
		}
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
	if rl, err := parse(uri); rl == nil || err != nil {
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
		if err == resourcehandlers.ErrResourceNotFound {
			return []*api.Node{}, nil
		}
		return nil, err
	}

	childResourceLocators := gh.cache.GetSubset(rl.String())
	childNodes, err := buildNodes(node, excludePaths, frontMatter, excludeFrontMatter, depth, childResourceLocators, gh.cache, 0)
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
	if rl, err = parse(uri); err != nil {
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
		gitInfo := transform(commits)
		if gitInfo == nil {
			return nil, nil
		}
		if blob, err = marshallGitInfo(gitInfo); err != nil {
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
	if rl, err = gh.URLToGitHubLocator(nil, uri, false); err != nil {
		panic(err)
	}
	if gh != nil {
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
	if rl, err = parse(absLink); err != nil {
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
	if rl, err = parse(absLink); err != nil {
		return "", err
	}
	if l := rl.GetRaw(); len(l) > 0 {
		return l, nil
	}
	return absLink, nil
}
