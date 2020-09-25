package github

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/google/go-github/v32/github"
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
	host := "github.com" //TODO convert api.domain to domain from url.Host
	sourceURLSegments := strings.Split(url.Path, "/")
	owner := sourceURLSegments[2]
	repo := sourceURLSegments[3]
	resourceType, err := NewResourceType(treeEntry.GetType())
	if err != nil {
		panic(fmt.Sprintf("unexpected resource type %v: %v", treeEntry.GetType(), err))
	}
	return &ResourceLocator{
		host,
		owner,
		repo,
		treeEntry.GetSHA(),
		resourceType,
		treeEntry.GetPath(),
		shaAlias,
	}
}

// Recursively adds or merges nodes built from flat ResourceLocators list to node.Nodes
func buildNodes(node *api.Node, childResourceLocators []*ResourceLocator, cache Cache) {
	var (
		nodePath            string
		nodeResourceLocator *ResourceLocator
	)
	if node.NodeSelector != nil {
		nodePath = node.NodeSelector.Path
	} else if len(node.ContentSelectors) > 0 {
		nodePath = node.ContentSelectors[0].Source
	}
	if nodeResourceLocator = cache.Get(nodePath); nodeResourceLocator == nil {
		panic(fmt.Sprintf("Node is not available as ResourceLocator %v", nodePath))
	}
	nodePathSegmentsCount := len(strings.Split(nodeResourceLocator.Path, "/"))
	for _, childResourceLocator := range childResourceLocators {
		if !strings.HasPrefix(childResourceLocator.Path, nodeResourceLocator.Path) {
			continue
		}
		childPathSegmentsCount := len(strings.Split(childResourceLocator.Path, "/"))
		childName := childResourceLocator.GetName()
		// 1 sublevel only
		if (childPathSegmentsCount - nodePathSegmentsCount) == 1 {
			// folders and .md files only
			if childResourceLocator.Type == Blob && !strings.HasSuffix(strings.ToLower(childName), ".md") {
				continue
			}
			n := &api.Node{
				ContentSelectors: []api.ContentSelector{{Source: childResourceLocator.String()}},
				Name:             childName,
			}
			n.SetParent(node)
			if node.Nodes == nil {
				node.Nodes = make([]*api.Node, 0)
			}

			node.Nodes = append(node.Nodes, n)

			// recursively build subnodes if entry is sub-tree
			if childResourceLocator.Type == Tree {
				childResourceLocators = cache.GetSubset(childResourceLocator.String())
				buildNodes(n, childResourceLocators, cache)
			}
		}
	}
}

// Cache is indexes GitHub TreeEntries by website resource URLs as keys,
// mapping ResourceLocator objects to them.
// TODO: implement me efficently and for parallel use
type Cache map[string]*ResourceLocator

// Get returns a ResourceLocator object mapped to the path (URL)
func (c Cache) Get(path string) *ResourceLocator {
	return c[path]
}

// HasURLPrefix returns true if pathPrefix tests true as prefix for path,
// either with tree or blob in its resource type segment
// The resource type in the URL prefix {tree|blob} changes according to the resource
// To keep the prefix valid it should alternate this path segment too.
func HasURLPrefix(path, pathPrefix string) bool {
	// TODO: make me global
	reStrBlob := regexp.MustCompile(fmt.Sprintf("^(.*?)%s(.*)$", "tree"))
	repStr := "${1}blob$2"
	pathPrefixAsBlob := pathPrefix
	pathPrefixAsBlob = reStrBlob.ReplaceAllString(pathPrefixAsBlob, repStr)
	return strings.HasPrefix(path, pathPrefix) || strings.HasPrefix(path, pathPrefixAsBlob)
}

// GetSubset returns a subset of the ResourceLocator objects mapped to keys
// with this pathPrefix
func (c Cache) GetSubset(pathPrefix string) []*ResourceLocator {
	var entries = make([]*ResourceLocator, 0)
	for k, v := range c {
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
func (c Cache) Set(path string, entry *ResourceLocator) *ResourceLocator {
	c[path] = entry
	return entry
}

// GitHub implements resourcehanlders#ResourceHandler
type GitHub struct {
	Client        *github.Client
	cache         Cache
	acceptedHosts []string
}

// NewResourceHandler creates new GitHub ResourceHandler objects
func NewResourceHandler(client *github.Client, acceptedHosts []string) resourcehandlers.ResourceHandler {
	return &GitHub{
		client,
		Cache{},
		acceptedHosts,
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
// property will be nil. In this case ctx can be omited too.
func (gh *GitHub) URLToGitHubLocator(ctx context.Context, urlString string, resolveAPIUrl bool) (*ResourceLocator, error) {
	var (
		ghRL *ResourceLocator
		err  error
	)
	// try cache first
	//TODO: we probably need lock before getting from the map
	if ghRL = gh.cache.Get(urlString); ghRL == nil {
		if ghRL, err = parse(urlString); err != nil {
			return nil, err
		}
		if resolveAPIUrl {
			_p := strings.Split(ghRL.Path, "/")[0]
			if _, found := nonSHAPathPrefixes[_p]; found {
				return ghRL, nil
			}
			// grab the index of this repo
			gitTree, _, err := gh.Client.Git.GetTree(ctx, ghRL.Owner, ghRL.Repo, ghRL.SHAAlias, true)
			if err != nil {
				return nil, err
			}
			// populate cache wth this tree entries
			for _, entry := range gitTree.Entries {
				rl := TreeEntryToGitHubLocator(entry, ghRL.SHAAlias)
				if HasURLPrefix(rl.String(), urlString) {
					gh.cache.Set(rl.String(), rl)
				}
			}
			ghRL = gh.cache.Get(urlString)
		}
	}
	return ghRL, nil
}

// Accept implements resourcehandlers.ResourceHandler#Accept
func (gh *GitHub) Accept(uri string) bool {
	var (
		url *url.URL
		err error
	)
	if gh.acceptedHosts == nil {
		return false
	}
	if url, err = url.Parse(uri); err != nil {
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
func (gh *GitHub) ResolveNodeSelector(ctx context.Context, node *api.Node) error {
	var (
		rl  *ResourceLocator
		err error
	)
	if rl, err = gh.URLToGitHubLocator(ctx, node.NodeSelector.Path, true); err != nil {
		return err
	}
	if rl != nil {
		// build node subnodes hierarchy from cache (URLToGitHubLocator populates the cache)
		childResourceLocators := gh.cache.GetSubset(rl.String())
		buildNodes(node, childResourceLocators, gh.cache)
	}
	return nil
}

// Accept implements resourcehandlers.ResourceHandler#Read
func (gh *GitHub) Read(ctx context.Context, uri string) ([]byte, error) {
	var (
		blob []byte
		rl   *ResourceLocator
		err  error
	)
	if rl, err = gh.URLToGitHubLocator(ctx, uri, true); err != nil {
		return nil, err
	}
	if rl != nil && rl.Type != Tree {
		blob, _, err = gh.Client.Git.GetBlobRaw(ctx, rl.Owner, rl.Repo, rl.SHA)
		if err != nil {
			return nil, err
		}
	}
	return blob, err
}

// Name implements resourcehandlers.ResourceHandler#Name
func (gh *GitHub) Name(uri string) string {
	var (
		rl  *ResourceLocator
		err error
	)
	if rl, err = gh.URLToGitHubLocator(nil, uri, true); err != nil {
		panic(err)
	}
	if gh != nil {
		return rl.GetName()
	}
	return ""
}

// BuildAbsLink builds the abs link from the source and the relative path
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

// GetLocalityDomainCandidate returns the provided source as locality domain candidate
// parameters suitable for quering reactor/LocalityDomain#PathInLocality
func (gh *GitHub) GetLocalityDomainCandidate(source string) (key, path, version string, err error) {
	var rl *ResourceLocator
	if rl, err = parse(source); rl != nil {
		version = rl.SHAAlias
		if len(rl.Host) > 0 && len(rl.Owner) > 0 && len(rl.Repo) > 0 {
			key = fmt.Sprintf("%s/%s/%s", rl.Host, rl.Owner, rl.Repo)
		}
		if len(rl.Owner) > 0 && len(rl.Repo) > 0 && len(rl.SHAAlias) > 0 {
			path = fmt.Sprintf("%s/%s/%s", rl.Owner, rl.Repo, rl.Path)
		}
	}
	return
}

// SetVersion replaces the version segment in the path of GitHub URLs if
// applicable or returns the original URL unchanged if not.
func (gh *GitHub) SetVersion(absLink, version string) (string, error) {
	var (
		rl  *ResourceLocator
		err error
	)
	if rl, err = parse(absLink); err != nil {
		return "", err
	}

	if len(rl.Path) > 0 {
		pathSegments := strings.Split(rl.Path, "/")
		if _, found := nonSHAPathPrefixes[pathSegments[0]]; found {
			return absLink, nil
		}
		rl.SHAAlias = version
		return rl.String(), nil
	}

	return absLink, nil
}
