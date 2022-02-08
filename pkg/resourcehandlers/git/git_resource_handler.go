// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate -header ../../../license_prefix.txt

//counterfeiter:generate os.FileInfo

package git

import (
	"context"
	"fmt"
	"github.com/hashicorp/go-multierror"
	nethttp "net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

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
	walker     func(root string, walkerFunc filepath.WalkFunc) error
}

// NewResourceHandlerTest creates new GitHub ResourceHandler objects given more arguments. Used when testing
func NewResourceHandlerTest(gitRepositoriesAbsPath string, user *string, oauthToken string, githubOAuthClient *ghclient.Client, httpClient *nethttp.Client, acceptedHosts []string, localMappings map[string]string, gitArg gitinterface.Git, prepRepos map[string]*Repository, fileR FileReader, walkerF func(root string, walkerFunc filepath.WalkFunc) error) resourcehandlers.ResourceHandler {
	out := &Git{
		client:                 githubOAuthClient,
		httpClient:             httpClient,
		gitAuth:                buildAuthMethod(user, oauthToken),
		localMappings:          localMappings,
		gitRepositoriesAbsPath: gitRepositoriesAbsPath,
		acceptedHosts:          acceptedHosts,
		git:                    gitArg,
		preparedRepos:          prepRepos,
		fileReader:             fileR,
		walker:                 walkerF,
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
		walker:                 filepath.Walk,
	}

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
func (g *Git) ResolveNodeSelector(ctx context.Context, node *api.Node) ([]*api.Node, error) {
	rl, err := github.Parse(node.NodeSelector.Path)
	if err != nil {
		return nil, err
	}
	repositoryPath := g.repositoryPathFromResourceLocator(rl)
	if err := g.prepareGitRepository(ctx, rl); err != nil {
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
		Nodes:        []*api.Node{},
		NodeSelector: node.NodeSelector,
	}
	nb := &nodeBuilder{
		rootNodePath:    nodesSelectorLocalPath,
		depth:           int(node.NodeSelector.Depth),
		excludePaths:    node.NodeSelector.ExcludePaths,
		someMap:         make(map[string]*api.Node),
		resourceLocator: rl,
	}
	g.walker(nodesSelectorLocalPath, nb.build)
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
	//removing child parent
	for _, child := range _node.Nodes {
		child.SetParent(nil)
	}
	// finally, cleanup folder entries from contentSelectors
	_node.Cleanup()
	if len(_node.Nodes) > 0 {
		sort.Slice(_node.Nodes, func(i, j int) bool {
			return _node.Nodes[i].Name < _node.Nodes[j].Name
		})
		return _node.Nodes, nil
	}
	return []*api.Node{}, nil
}

type nodeBuilder struct {
	rootNodePath    string
	depth           int
	excludePaths    []string
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
	source := filepath.ToSlash(strings.TrimPrefix(path, nb.rootNodePath))
	for _, excludePath := range nb.excludePaths {
		regex, err := regexp.Compile(excludePath)
		if err != nil {
			return fmt.Errorf("invalid path exclude expression %s: %w", excludePath, err)
		}
		urlString := source
		if regex.Match([]byte(urlString)) {
			return nil
		}
	}

	childPathSegmentsCount := len(strings.Split(source, "/")) - 1
	if nb.depth != 0 && childPathSegmentsCount > nb.depth {
		return nil
	}
	if info.IsDir() {
		newNode.Nodes = []*api.Node{}
		if newNode.Properties == nil {
			newNode.Properties = make(map[string]interface{})
			rl := getProperResourceLocator(nb.resourceLocator, github.Tree)
			newNode.Properties[api.ContainerNodeSourceLocation] = rl.String() + source
		}
	} else {
		if filepath.Ext(info.Name()) != ".md" {
			return nil
		}
		rl := getProperResourceLocator(nb.resourceLocator, github.Blob)
		newNode.Source = rl.String() + source
	}

	nb.someMap[path] = newNode
	return nil
}

func getProperResourceLocator(rl *github.ResourceLocator, desiredType github.ResourceType) *github.ResourceLocator {
	if rl.Type == desiredType {
		return rl
	}
	drl := &github.ResourceLocator{
		Scheme:   rl.Scheme,
		Host:     rl.Host,
		Owner:    rl.Owner,
		Repo:     rl.Repo,
		SHA:      rl.SHA,
		Type:     desiredType,
		Path:     rl.Path,
		SHAAlias: rl.SHAAlias,
		IsRawAPI: rl.IsRawAPI,
	}
	return drl
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
	if err = g.resolveDocumentationRelativePaths(&api.Node{Nodes: doc.Structure, NodeSelector: doc.NodeSelector}, rl.String()); err != nil {
		return nil, err
	}
	return doc, nil
}

// resolveDocumentationRelativePaths traverses api.Node#Nodes and resolve node Source, MultiSource and api.NodeSelector relative paths to absolute URLs
func (g *Git) resolveDocumentationRelativePaths(node *api.Node, moduleDocumentationPath string) error {
	var errs error
	if node.Source != "" {
		u, err := url.Parse(node.Source)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("manifest %s with invalid node %s source: %s", moduleDocumentationPath, node.FullName("/"), node.Source))
		} else if !u.IsAbs() {
			// resolve relative path
			if node.Source, err = g.BuildAbsLink(moduleDocumentationPath, node.Source); err != nil {
				errs = multierror.Append(errs, fmt.Errorf("cannot resolve source relative path %s in node %s and manifest %s", node.Source, node.FullName("/"), moduleDocumentationPath))
			}
		}
	}
	if len(node.MultiSource) > 0 {
		for idx, src := range node.MultiSource {
			u, err := url.Parse(src)
			if err != nil {
				errs = multierror.Append(errs, fmt.Errorf("manifest %s with invalid node %s multiSource[%d]: %s", moduleDocumentationPath, node.FullName("/"), idx, node.MultiSource[idx]))
			} else if !u.IsAbs() {
				// resolve relative path
				if node.Source, err = g.BuildAbsLink(moduleDocumentationPath, src); err != nil {
					errs = multierror.Append(errs, fmt.Errorf("cannot resolve multiSource[%d] relative path %s in node %s and manifest %s", idx, node.MultiSource[idx], node.FullName("/"), moduleDocumentationPath))
				}
			}
		}
	}
	if node.NodeSelector != nil {
		u, err := url.Parse(node.NodeSelector.Path)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("manifest %s with invalid nodeSelector path %s in node %s", moduleDocumentationPath, node.NodeSelector.Path, node.FullName("/")))
		} else if !u.IsAbs() {
			// resolve relative path
			if node.NodeSelector.Path, err = g.BuildAbsLink(moduleDocumentationPath, node.NodeSelector.Path); err != nil {
				errs = multierror.Append(errs, fmt.Errorf("cannot resolve nodeSelector relative path %s in node %s and manifest %s", node.NodeSelector.Path, node.FullName("/"), moduleDocumentationPath))
			}
		}
	}
	for _, n := range node.Nodes {
		if err := g.resolveDocumentationRelativePaths(n, moduleDocumentationPath); err != nil {
			errs = multierror.Append(errs, err)
		}
	}
	return errs
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
	host := rl.Host
	if strings.HasPrefix(rl.Host, "raw.") {
		if rl.Host == "raw.githubusercontent.com" {
			host = "github.com"
		} else {
			host = rl.Host[len("raw."):]
		}
	} else if host == "api.github.com" {
		host = "github.com"
	}
	return filepath.Join(g.gitRepositoriesAbsPath, host, rl.Owner, rl.Repo, rl.SHAAlias)
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

// GetRateLimit implements resourcehandlers.ResourceHandler#GetRateLimit
func (g *Git) GetRateLimit(ctx context.Context) (int, int, time.Time, error) {
	r, _, err := g.client.RateLimits(ctx)
	if err != nil {
		return -1, -1, time.Now(), err
	}
	return r.Core.Limit, r.Core.Remaining, r.Core.Reset.Time, nil
}
