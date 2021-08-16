// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package git

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/git"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/resourcehandlers/github"
	"github.com/gardener/docforge/pkg/util/urls"

	"github.com/go-git/go-git/v5/plumbing/transport/http"
	ghclient "github.com/google/go-github/v32/github"
)

const CacheDir string = "cache"

var (
	reHasTree = regexp.MustCompile("^(.*?)tree(.*)$")
	repStr    = "${1}blob$2"
)

type Git struct {
	client                 *ghclient.Client
	gitAuth                http.AuthMethod
	gitRepositoriesAbsPath string
	acceptedHosts          []string
	localMappings          map[string]string
	git                    git.Git

	preparedRepos map[string]*Repository
	mutex         sync.RWMutex
}

// NewResourceHandler creates new GitHub ResourceHandler objects
func NewResourceHandler(gitRepositoriesAbsPath string, user *string, oauthToken string, githubOAuthClient *ghclient.Client, acceptedHosts []string, localMappings map[string]string) resourcehandlers.ResourceHandler {
	return &Git{
		client:                 githubOAuthClient,
		gitAuth:                buildAuthMethod(user, oauthToken),
		localMappings:          localMappings,
		gitRepositoriesAbsPath: gitRepositoriesAbsPath,
		acceptedHosts:          acceptedHosts,
		git:                    git.NewGit(),
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

func (g *Git) ResolveNodeSelector(ctx context.Context, node *api.Node, excludePaths []string, frontMatter map[string]interface{}, excludeFrontMatter map[string]interface{}, depth int32) ([]*api.Node, error) {
	rl, err := github.Parse(node.NodeSelector.Path)
	if err != nil {
		return nil, err
	}
	repositoryPath := g.repositoryPathFromResourceLocator(rl)
	if err := g.prepareGitRepository(ctx, repositoryPath, rl); err != nil {
		return nil, err
	}

	nodesSelectorLocalPath := filepath.Join(repositoryPath, rl.Path)
	fileInfo, err := os.Stat(nodesSelectorLocalPath)
	if err != nil {
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
	if len(_node.Nodes) > 0 {
		return _node.Nodes, nil
	}
	return nil, nil
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
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("unable to parse file uri %s while reading: %v", uri, err)
	}
	// remove query from uri
	u.RawQuery = ""
	uri = u.String()
	rl, err := github.Parse(uri)
	if err != nil {
		return nil, err
	}

	for k, v := range g.localMappings {
		if strings.HasPrefix(uri, k) {
			fileInfo, err := os.Stat(v)
			if err != nil {
				return nil, fmt.Errorf("failed to use mapping because local path is invalid: %v", err)
			}
			if fileInfo.IsDir() {
				mappingResourceLocator, err := github.Parse(k)
				if err != nil {
					return nil, err
				}
				mappingPath := strings.TrimPrefix(rl.Path, mappingResourceLocator.Path)
				v = filepath.Join(v, mappingPath)
			}
			return readFile(v)
		}
	}

	repositoryPath := g.repositoryPathFromResourceLocator(rl)
	uri = filepath.Join(repositoryPath, rl.Path)
	if err := g.prepareGitRepository(ctx, repositoryPath, rl); err != nil {
		return nil, err
	}

	return readFile(uri)
}
func readFile(uri string) ([]byte, error) {
	fileInfo, err := os.Stat(uri)
	if err != nil {
		return nil, fmt.Errorf("Git resource handler failed to read file at %s: %v ", uri, err)
	}
	if fileInfo.IsDir() {
		return nil, nil
	}
	return ioutil.ReadFile(uri)

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

// BuildAbsLink should return an absolute path of a relative link in regards of the provided
// source
// BuildAbsLink builds the abs link from the source and the relative path
// Implements resourcehandlers/ResourceHandler#BuildAbsLink
func (g *Git) BuildAbsLink(source, relPath string) (string, error) {
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
	// if relative path ends with '/' change the type to Tree
	if strings.HasSuffix(relPath, "/") {
		var trl *github.ResourceLocator
		if trl, err = github.Parse(u.String()); err != nil {
			return "", err
		}
		trl.Type = github.Tree // change the type
		return trl.String(), nil
	} else {
		return u.String(), nil
	}
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
	repositoryPath := g.repositoryPathFromResourceLocator(rl)
	if err := g.prepareGitRepository(ctx, repositoryPath, rl); err != nil {
		return nil, err
	}

	blob, err := g.Read(ctx, uri)
	if err != nil {
		return nil, err
	}

	// not a documentation structure
	if blob == nil {
		return nil, nil
	}

	return api.Parse(blob)
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
