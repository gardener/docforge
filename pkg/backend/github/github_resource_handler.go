package github

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/gardener/docode/pkg/api"
	"github.com/google/go-github/v32/github"
)

// ResourceType is an enumeration for GitHub resource types
// Supported types are "tree" and "blob"
type ResourceType int

func (s ResourceType) String() string {
	return [...]string{"tree", "blob"}[s]
}

func NewResourceType(resourceTypeString string) (ResourceType, error) {
	switch resourceTypeString {
	case "tree":
		return Tree, nil
	case "blob":
		return Blob, nil
	}
	return 0, fmt.Errorf("Unknonw resource type string %s. Must be one of %v", resourceTypeString, []string{"tree", "blob"})
}

const (
	Tree ResourceType = iota
	Blob
)

// ResourceLocator is an abstraction for GitHub specific Universal Resource Locators (URLs)
// It is an internal structure breaking down the GitHub URLs into more segment types such as
// Repo, Owner or SHA.
// ResourceLocator is a common denominator used to translate between GitHub user-oriented urls
// and API urls
type ResourceLocator struct {
	Host  string
	Owner string
	Repo  string
	SHA   string
	Type  ResourceType
	Path  string
	API   string
}

// String produces a GitHub website link to a resource from a ResourceLocator.
// That's the format used to link GitHub rsource in the documentatin structure and pages.
// Example: https://github.com/gardener/gardener/blob/master/docs/README.md
func (g *ResourceLocator) String() string {
	return fmt.Sprintf("https://%s/%s/%s/%s/%s/%s", g.Host, g.Owner, g.Repo, g.Type, g.SHA, g.Path)
}

//
// Example tree:
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
func TreeEntryToGitHubLocator(treeEntry *github.TreeEntry) *ResourceLocator {
	url, err := url.Parse(treeEntry.GetURL())
	if err != nil {
		panic(fmt.Sprintf("failed to parse url %v: %v", treeEntry.GetURL(), err))
	}
	host := url.Host
	sourceURLSegments := strings.Split(url.Path, "/")
	owner := sourceURLSegments[1]
	repo := sourceURLSegments[2]
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
		treeEntry.GetURL(),
	}
}

// Recursively adds or merges nodes built from tree entries to node.Nodes
func buildNodes(ctx context.Context, node *api.Node, entries map[string]*github.TreeEntry) {
	var nodePath string
	if node.NodeSelector != nil {
		nodePath = node.NodeSelector.Path
	} else if node.Source != nil {
		nodePath = node.Source[0]
	}
	baseSegmentsCount := len(strings.Split(nodePath, "/"))
	for k, v := range entries {
		keySegmentsCount := len(strings.Split(k, "/"))
		// sublevel only
		if ((keySegmentsCount - baseSegmentsCount) == 1) && strings.HasPrefix(k, nodePath) {
			n := &api.Node{
				Source: []string{k},
			}
			if node.Nodes == nil {
				node.Nodes = make([]*api.Node, 0)
			}
			// merge (equality based on source[0]) or append
			merged := false
			for _, subnode := range node.Nodes {
				if len(n.Source) == 0 {
					subnode.Source = n.Source
					merged = true
				}
			}
			if merged == false {
				node.Nodes = append(node.Nodes, n)
			}
			// recursively build subnodes if entry is sub-tree
			if v.GetType() == "tree" {
				buildNodes(ctx, n, entries)
			}
			return
		}
	}
}

// Cache is a hierarchical cache for GitHub TreeEntries indexed by base url and path
// TODO: implement me
type Cache map[string]*ResourceLocator

func (c Cache) Get(path string) *ResourceLocator {
	return c[path]
}

func (c Cache) Set(path string, entry *ResourceLocator) *ResourceLocator {
	c[path] = entry
	return entry
}

type GitHub struct {
	Client *github.Client
	cache  Cache
}

// URLToGitHubLocator produces a ResourceLocator from a GitHub website URL
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
func (gh *GitHub) URLToGitHubLocator(ctx context.Context, urlString string, resolveAPIUrl bool) *ResourceLocator {
	var ghRL *ResourceLocator
	// try cache first
	if ghRL = gh.cache.Get(urlString); ghRL == nil {
		// TODO: error on parse failure or panic
		u, _ := url.Parse(urlString)
		host := u.Host
		sourceURLSegments := strings.Split(u.Path, "/")
		if len(sourceURLSegments) < 5 {
			// TOO: be consistent - throwing error or panic
			panic(fmt.Sprintf("invalid GitHub URL format %q", u))
		}
		owner := sourceURLSegments[1]
		repo := sourceURLSegments[2]
		resourceTypeString := sourceURLSegments[3]
		sha := sourceURLSegments[4]

		// get the github url "path" part
		s := strings.Join([]string{owner, repo, resourceTypeString, sha}, "/")
		var (
			resourceType ResourceType
			path         string
			err          error
		)
		if p := strings.Split(u.Path, s); len(p) > 0 {
			path = strings.TrimLeft(p[1], "/")
		}

		if resourceType, err = NewResourceType(resourceTypeString); err != nil {
			panic(fmt.Sprintf("unexpected resource type in url %s", resourceTypeString))
		}
		ghRL = &ResourceLocator{
			host,
			owner,
			repo,
			sha,
			resourceType,
			path,
			"",
		}
		if resolveAPIUrl {
			// grab the index of this repo
			gitTree, _, err := gh.Client.Git.GetTree(ctx, ghRL.Owner, ghRL.Repo, ghRL.SHA, true)
			if err != nil {
				return nil
			}
			// populate cache
			for _, entry := range gitTree.Entries {
				ghRL := TreeEntryToGitHubLocator(entry)
				gh.cache.Set(ghRL.String(), ghRL)
			}
			ghRL = gh.cache.Get(urlString)
		}
	}
	return ghRL
}

// getTreeEntryIndex retrieves from GitHub the tree (hierarchy) identified by the url as a flat
// index of hierarchy paths.
// url format is: https://api.github.com/repos/{owner}/{repo}/git/trees/{sha}
// sha must identify a commit or a tree.
// The returned index is a set of entries mapping tree entries paths to the TreeEntry objects.
func (gh *GitHub) getTreeEntryIndex(ctx context.Context, url string) (map[string]*github.TreeEntry, error) {
	ghRL := gh.URLToGitHubLocator(ctx, url, false)
	gitTree, response, err := gh.Client.Git.GetTree(ctx, ghRL.Owner, ghRL.Repo, ghRL.SHA, true)
	if err != nil || response.StatusCode != 200 {
		return nil, fmt.Errorf("not 200 status code returned, but %d. Failed to get file tree: %v ", response.StatusCode, err)
	}
	// filter and index git tree entries for blobs and subtrees in the given source path
	entries := make(map[string]*github.TreeEntry)
	for _, entry := range gitTree.Entries {
		entries[entry.GetPath()] = entry
	}
	return entries, nil
}

// Accept implements backend.ResourceHandler#Accept
func (gh *GitHub) Accept(uri string) bool {
	var (
		url *url.URL
		err error
	)
	if url, err = url.Parse(uri); err != nil {
		return false
	}
	if url.Host != "github.com" {
		return false
	}
	return true
}

// ResolveNodeSelector recursively adds nodes built from tree entries to node
func (gh *GitHub) ResolveNodeSelector(ctx context.Context, node *api.Node) error {
	var (
		idx map[string]*github.TreeEntry
		err error
	)
	if idx, err = gh.getTreeEntryIndex(ctx, node.NodeSelector.Path); err != nil {
		return err
	}
	buildNodes(ctx, node, idx)
	return nil
}

// Accept implements backend.ResourceHandler#Read
func (gh *GitHub) Read(ctx context.Context, node *api.Node) []byte {
	ghRL := gh.URLToGitHubLocator(ctx, node.Source[0], false)
	blob, _, _ := gh.Client.Git.GetBlobRaw(ctx, ghRL.Owner, ghRL.Repo, ghRL.SHA)

	// if err != nil {
	// 	return err
	// }
	return blob
}

func (gh *GitHub) Path(uri string) string {
	ghRL := gh.URLToGitHubLocator(nil, uri, false)
	return ghRL.Path
}

func (gh *GitHub) DownloadUrl(uri string) string {
	return gh.URLToGitHubLocator(nil, uri, false).String()
}
