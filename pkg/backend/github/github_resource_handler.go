package github

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
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

// GetName returns the Name segment of a resource URL path
func (g *ResourceLocator) GetName() string {
	if len(g.Path) == 0 {
		return ""
	}
	p := strings.Split(g.Path, "/")
	return p[len(p)-1]
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
		shaAlias,
		resourceType,
		treeEntry.GetPath(),
		treeEntry.GetURL(),
	}
}

// Recursively adds or merges nodes built from flat ResourceLocators list to node.Nodes
func buildNodes(node *api.Node, childResourceLocators []*ResourceLocator, cache Cache) {
	var nodePath string
	if node.NodeSelector != nil {
		nodePath = node.NodeSelector.Path
	} else if node.Source != nil {
		nodePath = node.Source[0]
	}
	nodeResourceLocator := cache.Get(nodePath)
	nodePathSegmentsCount := len(strings.Split(nodeResourceLocator.Path, "/"))
	for _, childResourceLocator := range childResourceLocators {
		if !strings.HasPrefix(childResourceLocator.Path, nodeResourceLocator.Path) {
			continue
		}
		childPathSegmentsCount := len(strings.Split(childResourceLocator.Path, "/"))
		childName := childResourceLocator.GetName()
		// 1 sublevel only
		if (childPathSegmentsCount - nodePathSegmentsCount) == 1 {
			// .md files only
			if childResourceLocator.Type == Blob && !strings.HasSuffix(strings.ToLower(childName), ".md") {
				continue
			}
			n := &api.Node{
				parent:= node
				Source: []string{childResourceLocator.String()},
				Name:   childName,
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
			if childResourceLocator.Type == Tree {
				childResourceLocators = cache.GetSubset(childResourceLocator.String())
				buildNodes(n, childResourceLocators, cache)
			}
		}
	}
}

// Cache is a hierarchical cache for GitHub TreeEntries indexed by base url and path
// TODO: implement me
type Cache map[string]*ResourceLocator

func (c Cache) Get(path string) *ResourceLocator {
	return c[path]
}

func (c Cache) GetSubset(pathPrefix string) []*ResourceLocator {
	var entries = make([]*ResourceLocator, 0)
	// The resource type in the URL prefix {tree|blob} changes according to the resource
	// To keep the prefix valid it should alternate this path segment too.
	reStrBlob := regexp.MustCompile(fmt.Sprintf("^(.*?)%s(.*)$", "tree"))
	repStr := "${1}blob$2"
	for k, v := range c {
		pathPrefixAsBlob := pathPrefix
		pathPrefixAsBlob = reStrBlob.ReplaceAllString(pathPrefixAsBlob, repStr)
		if k == pathPrefix {
			continue
		}
		if strings.HasPrefix(k, pathPrefix) || strings.HasPrefix(k, pathPrefixAsBlob) {
			entries = append(entries, v)
		}
	}
	return entries
}

func (c Cache) Set(path string, entry *ResourceLocator) *ResourceLocator {
	c[path] = entry
	return entry
}

type GitHub struct {
	Client *github.Client
	cache  Cache
}

// Parse a GitHub URL into an incomplete ResourceLocator, without
// the APIUrl property.
func parse(urlString string) *ResourceLocator {
	// TODO: error on parse failure or panic
	u, _ := url.Parse(urlString)
	host := u.Host
	sourceURLSegments := strings.Split(u.Path, "/")
	if len(sourceURLSegments) < 5 {
		// TODO: be consistent - throwing error or panic
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
	ghRL := &ResourceLocator{
		host,
		owner,
		repo,
		sha,
		resourceType,
		path,
		"",
	}
	return ghRL
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
func (gh *GitHub) URLToGitHubLocator(ctx context.Context, urlString string, resolveAPIUrl bool) *ResourceLocator {
	var ghRL *ResourceLocator
	// try cache first
	if ghRL = gh.cache.Get(urlString); ghRL == nil {
		ghRL = parse(urlString)
		if resolveAPIUrl {
			// grab the index of this repo
			gitTree, _, err := gh.Client.Git.GetTree(ctx, ghRL.Owner, ghRL.Repo, ghRL.SHA, true)
			if err != nil {
				return nil
			}
			// populate cache wth this tree entries
			for _, entry := range gitTree.Entries {
				rl := TreeEntryToGitHubLocator(entry, ghRL.SHA)
				gh.cache.Set(rl.String(), rl)
			}
			ghRL = gh.cache.Get(urlString)
		}
	}
	return ghRL
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
	// Get ResourceLocator for this node's NodeSelector path and cache its URL's repo
	// tree entries
	ghRL := gh.URLToGitHubLocator(ctx, node.NodeSelector.Path, true)
	// build node subnodes hierarchy from cache
	childResourceLocators := gh.cache.GetSubset(ghRL.String())
	buildNodes(node, childResourceLocators, gh.cache)
	return nil
}

// Accept implements backend.ResourceHandler#Read
func (gh *GitHub) Read(ctx context.Context, node *api.Node) ([]byte, error) {
	ghRL := gh.URLToGitHubLocator(ctx, node.Source[0], false)
	blob, _, err := gh.Client.Git.GetBlobRaw(ctx, ghRL.Owner, ghRL.Repo, ghRL.SHA)
	return blob, err
}

func (gh *GitHub) Path(uri string) string {
	ghRL := gh.URLToGitHubLocator(nil, uri, false)
	return ghRL.Path
}
