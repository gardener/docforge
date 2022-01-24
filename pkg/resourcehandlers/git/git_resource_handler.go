// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate -header ../../../license_prefix.txt

//counterfeiter:generate os.FileInfo

package git

import (
	"context"
	"fmt"
	nethttp "net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/resourcehandlers/git/gitinterface"
	"github.com/gardener/docforge/pkg/resourcehandlers/github"
	"github.com/gardener/docforge/pkg/util/httpclient"
	"github.com/gardener/docforge/pkg/util/urls"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	ghclient "github.com/google/go-github/v32/github"
)

// CacheDir is the name of repository cache directory
const CacheDir string = "cache"

var (
	reHasTree = regexp.MustCompile("^(.*?)tree(.*)$")
	repStr    = "${1}blob$2"
)

// FileReader defines interface for reading file attributes and content
//counterfeiter:generate . FileReader
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
	git                    gitinterface.Git

	preparedRepos map[string]*Repository
	mutex         sync.RWMutex

	fileReader FileReader
	cache      *github.Cache
}

// NewResourceHandlerCachedTest creates new GitHub ResourceHandler objects given more arguments. Used when testing
func NewResourceHandlerCachedTest(gitRepositoriesAbsPath string, user *string, oauthToken string, githubOAuthClient *ghclient.Client, httpClient *nethttp.Client, acceptedHosts []string, localMappings map[string]string, gitArg gitinterface.Git, prepRepos map[string]*Repository, fileR FileReader, cache *github.Cache) resourcehandlers.ResourceHandler {
	out := &Git{
		client:                 githubOAuthClient,
		httpClient:             httpClient,
		gitAuth:                buildAuthMethod(user, oauthToken),
		localMappings:          localMappings,
		gitRepositoriesAbsPath: gitRepositoriesAbsPath,
		acceptedHosts:          acceptedHosts,
		git:                    gitArg,
		fileReader:             fileR,
		preparedRepos:          prepRepos,
		cache:                  cache,
	}
	return out
}

// NewResourceHandler creates new GitHub ResourceHandler objects
func NewResourceHandler(gitRepositoriesAbsPath string, user *string, oauthToken string, githubOAuthClient *ghclient.Client, httpClient *nethttp.Client, acceptedHosts []string, localMappings map[string]string) resourcehandlers.ResourceHandler {
	out := &Git{
		client:                 githubOAuthClient,
		httpClient:             httpClient,
		gitAuth:                buildAuthMethod(user, oauthToken),
		localMappings:          localMappings,
		gitRepositoriesAbsPath: gitRepositoriesAbsPath,
		acceptedHosts:          acceptedHosts,
		git:                    gitinterface.NewGit(),
		fileReader:             &osReader{},
	}

	out.cache = github.NewEmptyCache(&TreeExtractorGit{gitRH: out, walker: filepath.Walk})
	return out

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
		if _, ok := err.(resourcehandlers.ErrResourceNotFound); ok {
			return []*api.Node{}, nil
		}
		return nil, err
	}

	//preparing repository
	if err := g.prepareGitRepository(ctx, rl); err != nil {
		return nil, err
	}

	return github.BaseResolveNodeSelector(ctx, rl, g, g.cache, node, excludePaths, frontMatter, excludeFrontMatter, depth)
}

//TreeExtractorGit extracts the tree structure from a local git repository
type TreeExtractorGit struct {
	gitRH  *Git
	walker func(root string, walkerFunc filepath.WalkFunc) error
}

//NewTreeExtractorTest creates a new git tree extractor for testing
func NewTreeExtractorTest(gitRH *Git, walker func(root string, walkerFunc filepath.WalkFunc) error) *TreeExtractorGit {
	return &TreeExtractorGit{gitRH: gitRH, walker: walker}
}

//ExtractTree extracts the content given a resource locator, ignoring its path. In other words, it treats it as a repo
func (tE *TreeExtractorGit) ExtractTree(ctx context.Context, rl *github.ResourceLocator) ([]*github.ResourceLocator, error) {
	//preparing repository
	repositoryPath := tE.gitRH.repositoryPathFromResourceLocator(rl)
	if err := tE.gitRH.prepareGitRepository(ctx, rl); err != nil {
		return nil, err
	}
	nodesSelectorLocalPath := tE.gitRH.getNodeSelectorLocalPath(repositoryPath, rl)
	root := strings.Split(nodesSelectorLocalPath, rl.SHAAlias)[0] + rl.SHAAlias

	result := make([]*github.ResourceLocator, 0)
	err := tE.walker(root, func(path string, info os.FileInfo, err error) error {
		if path == root {
			return nil
		}
		relativePath := strings.TrimPrefix(path, root+"/")
		//dont include files from the .git folder, because they are not part of the repository
		if relativePath == ".git" || strings.HasPrefix(relativePath, ".git/") {
			return nil
		}
		resourceType := github.Blob
		if info.IsDir() {
			resourceType = github.Tree
		}
		result = append(result, &github.ResourceLocator{
			Scheme:   rl.Scheme,
			Host:     rl.Host,
			Owner:    rl.Owner,
			Repo:     rl.Repo,
			SHAAlias: rl.SHAAlias,
			Type:     resourceType,
			Path:     relativePath,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
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

func (g *Git) prepareGitRepository(ctx context.Context, rl *github.ResourceLocator) error {
	repositoryPath := g.repositoryPathFromResourceLocator(rl)
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
		if err := g.prepareGitRepository(ctx, rl); err != nil {
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

	if err := g.prepareGitRepository(ctx, rl); err != nil {
		return nil, err
	}

	tags, err := g.getAllTags(rl)
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
	doc, err := api.ParseWithMetadata(blob, tags, nVersions, rl.SHAAlias)
	if err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %s. %+v", uri, err)
	}

	return doc, nil
}

//internally used
func (g *Git) getAllTags(rl *github.ResourceLocator) ([]string, error) {
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
