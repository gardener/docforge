// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package git

import (
	"context"
	"fmt"
	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/git"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/resourcehandlers/github"
	"github.com/gardener/docforge/pkg/util/httpclient"
	"github.com/gardener/docforge/pkg/util/urls"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	ghclient "github.com/google/go-github/v32/github"
	nethttp "net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// CacheDir is the name of repository cache directory
const CacheDir string = "cache"

var (
	reHasTree = regexp.MustCompile("^(.*?)tree(.*)$")
	repStr    = "${1}blob$2"
)

// FileReader defines interface for reading file attributes and content
type FileReader interface {
	ReadFile(string) ([]byte, error)
	Stat(name string) (os.FileInfo, error)
	IsNotExist(err error) bool
}

type osReader struct{}

func (osR *osReader) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

func (osR *osReader) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

func (osR *osReader) IsNotExist(err error) bool {
	return os.IsNotExist(err)
}

// Git represents a resourcehandlers.ResourceHandler for git repositories
type Git struct {
	client                 *ghclient.Client
	httpClient             *nethttp.Client
	gitAuth                http.AuthMethod
	gitRepositoriesAbsPath string
	acceptedHosts          []string
	localMappings          map[string]string
	git                    git.Git

	preparedRepos map[string]*Repository
	mutex         sync.RWMutex

	fileReader FileReader
}

// NewResourceHandler creates new GitHub ResourceHandler objects
func NewResourceHandler(gitRepositoriesAbsPath string, user *string, oauthToken string, githubOAuthClient *ghclient.Client, httpClient *nethttp.Client, acceptedHosts []string, localMappings map[string]string) resourcehandlers.ResourceHandler {
	return &Git{
		client:                 githubOAuthClient,
		httpClient:             httpClient,
		gitAuth:                buildAuthMethod(user, oauthToken),
		localMappings:          localMappings,
		gitRepositoriesAbsPath: gitRepositoriesAbsPath,
		acceptedHosts:          acceptedHosts,
		git:                    git.NewGit(),
		fileReader:             &osReader{},
	}
}

func buildAuthMethod(user *string, oauthToken string) http.AuthMethod {
	// why BasicAuth - https://stackoverflow.com/a/52219873
	var u string
	if user != nil {
		u = *user
	}

	return &http.BasicAuth{
		Username: u,
		Password: oauthToken,
	}
}

// Accept implements resourcehandlers.ResourceHandler#Accept
func (g *Git) Accept(uri string) bool {
	var (
		url *url.URL
		err error
	)
	if g.acceptedHosts == nil {
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
	if rl, err := github.Parse(uri); rl == nil || err != nil {
		return false
	}
	for _, s := range g.acceptedHosts {
		if url.Host == s {
			return true
		}
	}
	return false
}

// ResolveNodeSelector implements resourcehandlers.ResourceHandler#ResolveNodeSelector
func (g *Git) ResolveNodeSelector(ctx context.Context, node *api.Node, excludePaths []string, frontMatter map[string]interface{}, excludeFrontMatter map[string]interface{}, depth int32) ([]*api.Node, error) {
	rl, err := github.Parse(node.NodeSelector.Path)
	if err != nil {
		return nil, err
	}
	repositoryPath := g.repositoryPathFromResourceLocator(rl)
	if err := g.prepareGitRepository(ctx, repositoryPath, rl); err != nil {
		return nil, err
	}

	nodesSelectorLocalPath := g.getNodeSelectorLocalPath(repositoryPath, rl)
	fileInfo, err := g.fileReader.Stat(nodesSelectorLocalPath)
	if err != nil {
		if g.fileReader.IsNotExist(err) {
			return nil, resourcehandlers.ErrResourceNotFound(node.NodeSelector.Path)
		}
		return nil, err
	}
	if !fileInfo.IsDir() && filepath.Ext(fileInfo.Name()) == ".yaml" {
		return nil, fmt.Errorf("nodeSelector path is neither directory or module")
	}
	_node := &api.Node{
		Nodes: []*api.Node{},
	}
	nb := &nodeBuilder{
		rootNodePath:    nodesSelectorLocalPath,
		someMap:         make(map[string]*api.Node),
		resourceLocator: rl,
	}
	filepath.Walk(nodesSelectorLocalPath, nb.build)
	for path, n := range nb.someMap {
		parentPath := filepath.Dir(path)
		parent, exists := nb.someMap[parentPath]
		if !exists { // If a parent does not exist, this is the root.
			_node = n
		} else {
			n.SetParent(parent)
			n.Parent().Nodes = append(n.Parent().Nodes, n)
		}
	}

	_node.SetParentsDownwards()
	// finally, cleanup folder entries from contentSelectors
	for _, child := range _node.Nodes {
		github.CleanupNodeTree(child)
	}
	if len(_node.Nodes) > 0 {
		return _node.Nodes, nil
	}
	return nil, nil
}

func (g *Git) getNodeSelectorLocalPath(repositoryPath string, rl *github.ResourceLocator) string {
	// first check for provided repository mapping
	mapKey := fmt.Sprintf("%s://%s/%s/%s", rl.Scheme, rl.Host, rl.Owner, rl.Repo)
	path, ok := g.localMappings[mapKey]
	if ok && len(path) > 0 {
		if fi, err := g.fileReader.Stat(path); err == nil && fi.IsDir() {
			return filepath.Join(path, rl.Path)
		}
	}
	return filepath.Join(repositoryPath, rl.Path)
}

type nodeBuilder struct {
	rootNodePath    string
	someMap         map[string]*api.Node
	resourceLocator *github.ResourceLocator
}

func (nb *nodeBuilder) build(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}
	newNode := &api.Node{
		Name: info.Name(),
	}
	if info.IsDir() {
		newNode.Nodes = []*api.Node{}
		source := filepath.ToSlash(strings.TrimPrefix(path, nb.rootNodePath))
		newNode.SetSourceLocation(nb.resourceLocator.String() + source)
	} else {
		if filepath.Ext(info.Name()) != ".md" {
			return nil
		}
		source := filepath.ToSlash(strings.TrimPrefix(path, nb.rootNodePath))

		// Change file types of the tree leafs from tree to blob
		currentPath := nb.resourceLocator.String()
		pathAsBlob := reHasTree.ReplaceAllString(currentPath, repStr)

		newNode.Source = pathAsBlob + source
	}

	nb.someMap[path] = newNode
	return nil
}

func (g *Git) prepareGitRepository(ctx context.Context, repositoryPath string, rl *github.ResourceLocator) error {
	repository := g.getOrInitRepository(repositoryPath, rl)
	return repository.Prepare(ctx, rl.SHAAlias)
}

// System cache structure type/org/repo
func (g *Git) Read(ctx context.Context, uri string) ([]byte, error) {
	var (
		uriPath  string
		fileInfo os.FileInfo
		err      error
	)
	if uriPath, err = g.getGitFilePath(ctx, uri, true); err != nil {
		return nil, err
	}
	fileInfo, err = g.fileReader.Stat(uriPath)
	if err != nil {
		if g.fileReader.IsNotExist(err) {
			return nil, resourcehandlers.ErrResourceNotFound(uri)
		}
		return nil, fmt.Errorf("Git resource handler failed to read file at %s: %v ", uriPath, err)
	}
	if fileInfo.IsDir() {
		return nil, nil
	}
	return g.fileReader.ReadFile(uriPath)
}

func (g *Git) getGitFilePath(ctx context.Context, uri string, initRepo bool) (string, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", fmt.Errorf("unable to parse file uri %s: %v", uri, err)
	}
	// remove query & fragment from uri
	u.RawQuery = ""
	u.Fragment = ""
	u.ForceQuery = false
	rl, err := github.Parse(u.String())
	if err != nil {
		return "", fmt.Errorf("unable to parse file uri %s: %v", uri, err)
	}
	// first check for provided repository mapping
	for k, v := range g.localMappings {
		if strings.HasPrefix(uri, k) {
			fileInfo, err := g.fileReader.Stat(v)
			if err != nil {
				return "", fmt.Errorf("failed to use mapping %s because local path is invalid: %v", k, err)
			}
			if fileInfo.IsDir() {
				mappingResourceLocator, err := github.Parse(k)
				if err != nil {
					return "", err
				}
				mappingPath := strings.TrimPrefix(rl.Path, mappingResourceLocator.Path)
				v = filepath.Join(v, mappingPath)
			}
			return v, nil
		}
	}
	// use git cache folder
	repositoryPath := g.repositoryPathFromResourceLocator(rl)
	uri = filepath.Join(repositoryPath, rl.Path)
	if initRepo {
		if err := g.prepareGitRepository(ctx, repositoryPath, rl); err != nil {
			return "", err
		}
	}
	return uri, nil
}

// ReadGitInfo implements resourcehandlers/ResourceHandler#ReadGitInfo
func (g *Git) ReadGitInfo(ctx context.Context, uri string) ([]byte, error) {
	return github.ReadGitInfo(ctx, uri, g.client)
}

// ResourceName returns a breakdown of a resource name in the link, consisting
// of name and potentially and extension without the dot.
func (g *Git) ResourceName(link string) (string, string) {
	if u, err := urls.Parse(link); err == nil {
		return u.ResourceName, u.Extension
	}
	return "", ""
}

// BuildAbsLink should return an absolute path of a relative link in regard to the provided
// source
// BuildAbsLink builds the abs link from the source and the relative path
// Implements resourcehandlers/ResourceHandler#BuildAbsLink
func (g *Git) BuildAbsLink(source, relPath string) (string, error) {
	u, err := url.Parse(relPath)
	if err != nil {
		return "", err
	}

	if !u.IsAbs() {
		u, err = url.Parse(source)
		if err != nil {
			return "", err
		}

		if strings.HasPrefix(relPath, "/") {
			// local link path starting from repo root
			var rl *github.ResourceLocator
			if rl, err = github.Parse(source); err != nil {
				return "", err
			}
			if rl != nil {
				repo := fmt.Sprintf("/%s/%s/%s/%s", rl.Owner, rl.Repo, rl.Type, rl.SHAAlias)
				if !strings.HasPrefix(relPath, repo+"/") {
					relPath = fmt.Sprintf("%s%s", repo, relPath)
				}
			}
		}

		u, err = u.Parse(relPath)
		if err != nil {
			return "", err
		}
	}

	return g.verifyLinkType(u)
}

// verifyLinkType verifies the relative link type ('blob' or 'tree')
// and change the type if required. If the link doesn't exist
// #resourcehandlers.ErrResourceNotFound error is returned.
func (g *Git) verifyLinkType(u *url.URL) (string, error) {
	link := u.String()
	linkPath, err := g.getGitFilePath(context.Background(), link, false)
	if err != nil {
		return "", err
	}
	info, err := g.fileReader.Stat(linkPath)
	if err != nil {
		if g.fileReader.IsNotExist(err) {
			return link, resourcehandlers.ErrResourceNotFound(link)
		}
		return "", err
	}
	rl, err := github.Parse(link)
	if err != nil {
		return "", err
	}
	if info.IsDir() && rl.Type == github.Blob {
		// change the type
		rl.Type = github.Tree
		link = rl.String()
	} else if !info.IsDir() && rl.Type == github.Tree {
		// change the type
		rl.Type = github.Blob
		link = rl.String()
	}
	return link, nil
}

// GetRawFormatLink returns a link to an embeddable object (image) in raw format.
// If the provided link is not referencing an embeddable object, the function
// returns absLink without changes.
func (g *Git) GetRawFormatLink(absLink string) (string, error) {
	var (
		rl  *github.ResourceLocator
		err error
	)
	if rl, err = github.Parse(absLink); err != nil {
		return "", err
	}
	if l := rl.GetRaw(); len(l) > 0 {
		return l, nil
	}
	return absLink, nil
}

// SetVersion sets version to absLink according to the API scheme. For GitHub
// for example this would replace e.g. the 'master' segment in the path with version
func (g *Git) SetVersion(absLink, version string) (string, error) {
	var (
		rl  *github.ResourceLocator
		err error
	)
	if rl, err = github.Parse(absLink); err != nil {
		return "", err
	}

	if len(rl.SHAAlias) > 0 {
		rl.SHAAlias = version
		return rl.String(), nil
	}

	return absLink, nil
}

// ResolveDocumentation for a given uri
func (g *Git) ResolveDocumentation(ctx context.Context, uri string) (*api.Documentation, error) {
	rl, err := github.Parse(uri)
	if err != nil {
		return nil, err
	}
	if rl.SHAAlias == "DEFAULT_BRANCH" {
		if rl.SHAAlias, err = github.GetDefaultBranch(ctx, g.client, rl); err != nil {
			return nil, err
		}
	}
	//here rl.SHAAlias on the right side is the repo current branch
	rl.SHAAlias = api.ChooseTargetBranch(uri, rl.SHAAlias)
	//getting nVersions based on configuration
	nVersions := api.ChooseNVersions(uri)

	repositoryPath := g.repositoryPathFromResourceLocator(rl)
	if err := g.prepareGitRepository(ctx, repositoryPath, rl); err != nil {
		return nil, err
	}

	tags, err := g.getAllTags(ctx, rl)
	if err != nil {
		return nil, err
	}

	blob, err := g.Read(ctx, rl.String())
	if err != nil {
		return nil, err
	}

	// not a documentation structure
	if blob == nil {
		return nil, nil
	}
	return api.ParseWithMetadata(blob, tags, nVersions, rl.SHAAlias)
}

//internally used
func (g *Git) getAllTags(ctx context.Context, rl *github.ResourceLocator) ([]string, error) {
	repositoryPath := g.repositoryPathFromResourceLocator(rl)
	repo := g.getOrInitRepository(repositoryPath, rl)
	gitRepo, err := repo.Git.PlainOpen(repo.LocalPath)
	if err != nil {
		return nil, err
	}
	tags, err := gitRepo.Tags()
	return tags, err
}

// GetClient implements resourcehandlers.ResourceHandler#GetClient
func (g *Git) GetClient() httpclient.Client {
	return g.httpClient
}

func (g *Git) repositoryPathFromResourceLocator(rl *github.ResourceLocator) string {
	return filepath.Join(g.gitRepositoriesAbsPath, rl.Host, rl.Owner, rl.Repo, rl.SHAAlias)
}

// getOrInitRepository serves as a sync point to avoid more complicated logic for synchronization between workers working on the same repository. In case it returns false no one began working on
func (g *Git) getOrInitRepository(repositoryPath string, rl *github.ResourceLocator) *Repository {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	if g.preparedRepos == nil {
		g.preparedRepos = map[string]*Repository{}
	}

	if repoInfo, ok := g.preparedRepos[repositoryPath]; ok {
		return repoInfo
	}
	repository := &Repository{
		Git:           g.git,
		Auth:          g.gitAuth,
		LocalPath:     repositoryPath,
		RemoteURL:     "https://" + rl.Host + "/" + rl.Owner + "/" + rl.Repo,
		PreviousError: nil,
		mutex:         sync.RWMutex{},
	}

	g.preparedRepos[repositoryPath] = repository
	return repository
}
