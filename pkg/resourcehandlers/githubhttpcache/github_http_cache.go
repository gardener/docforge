// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package githubhttpcache

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gardener/docforge/pkg/httpclient"
	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/resourcehandlers/osshim"
	"github.com/google/go-github/v43/github"
	"k8s.io/klog/v2"
)

// GHC implements resourcehandlers.ResourceHandler interface using GitHub manifestadapter with transport level persistent cache.
type GHC struct {
	client        *github.Client
	httpClient    *http.Client
	os            osshim.Os
	acceptedHosts []string
	localMappings map[string]string
	filesCache    map[string]string
	muxSHA        sync.RWMutex
	defBranches   map[string]string
	muxDefBr      sync.Mutex
	muxCnt        sync.Mutex
	options       manifest.ParsingOptions
}

// NewGHC creates new GHC resource handler
func NewGHC(client *github.Client, httpClient *http.Client, os osshim.Os, acceptedHosts []string, localMappings map[string]string, options manifest.ParsingOptions) resourcehandlers.ResourceHandler {
	return &GHC{
		client:        client,
		httpClient:    httpClient,
		os:            os,
		acceptedHosts: acceptedHosts,
		localMappings: localMappings,
		filesCache:    make(map[string]string),
		defBranches:   make(map[string]string),
		options:       options,
	}
}

const (
	// DateFormat defines format for LastModifiedDate & PublishDate
	DateFormat = "2006-01-02 15:04:05"
)

// GitInfo defines git resource attributes
type GitInfo struct {
	LastModifiedDate *string        `json:"lastmod,omitempty"`
	PublishDate      *string        `json:"publishdate,omitempty"`
	Author           *github.User   `json:"author,omitempty"`
	Contributors     []*github.User `json:"contributors,omitempty"`
	WebURL           *string        `json:"weburl,omitempty"`
	SHA              *string        `json:"sha,omitempty"`
	SHAAlias         *string        `json:"shaalias,omitempty"`
	Path             *string        `json:"path,omitempty"`
}

//========================= manifest.FileSource ===================================================

// FileTreeFromURL implements manifest.FileSource#FileTreeFromURL
func (p *GHC) FileTreeFromURL(url string) ([]string, error) {
	r, err := p.getResolvedResourceInfo(context.TODO(), url)
	if err != nil {
		return nil, err
	}
	if r.Type != "tree" {
		return nil, fmt.Errorf("not a tree url: %s", r.Raw)
	}
	//bPrefix := fmt.Sprintf("%s://%s/%s/%s/blob/%s/%s", r.URL.Scheme, r.URL.Host, r.Owner, r.Repo, r.Ref, r.Path)
	p.muxSHA.Lock()
	defer p.muxSHA.Unlock()
	if local := p.checkForLocalMapping(r); len(local) > 0 {
		return p.readLocalFileTree(*r, local), nil
	}
	t, err := p.getTree(context.TODO(), r, true)
	if err != nil {
		return nil, err
	}
	res := []string{}
	for _, e := range t.Entries {
		extracted := false
		ePath := strings.TrimPrefix(*e.Path, "/")
		for _, extractedFormat := range p.options.ExtractedFilesFormats {
			if strings.HasSuffix(strings.ToLower(ePath), extractedFormat) {
				extracted = true
				break
			}
		}
		// skip node if it is not a supported format
		if *e.Type != "blob" || !extracted {
			//klog.V(6).Infof("node selector %s skip entry %s\n", node.NodeSelector.Path, ePath)
			continue
		}
		res = append(res, ePath)

	}
	return res, nil
}

// ManifestFromURL implements manifest.FileSource#ManifestFromURL
func (p *GHC) ManifestFromURL(url string) (string, error) {
	r, err := p.getResolvedResourceInfo(context.TODO(), url)
	if err != nil {
		return "", err
	}
	content, err := p.Read(context.TODO(), r.GetURL())
	return string(content), err
}

// BuildAbsLink implements manifest.FileSource#BuildAbsLink
func (p *GHC) BuildAbsLink(source, link string) (string, error) {
	r, err := p.getResolvedResourceInfo(context.TODO(), source)
	if err != nil {
		return "", err
	}

	if strings.HasPrefix(link, "http") {
		l, err := p.getResolvedResourceInfo(context.TODO(), link)
		if err != nil {
			return "", err
		}
		link = l.GetURL()
	}
	return p.buildAbsLink(r, link)
}

//========================= resourcehandlers.ResourceHandler ===================================================

// Accept implements the resourcehandlers.ResourceHandler#Accept
func (p *GHC) Accept(uri string) bool {
	r, err := resourcehandlers.BuildResourceInfo(uri)
	if err != nil || r.URL.Scheme != "https" {
		return false
	}
	for _, h := range p.acceptedHosts {
		if h == r.URL.Host {
			return true
		}
	}
	return false
}

// Read implements the resourcehandlers.ResourceHandler#Read
func (p *GHC) Read(ctx context.Context, uri string) ([]byte, error) {
	r, err := p.getResolvedResourceInfo(ctx, uri)
	if err != nil {
		return nil, err
	}
	if r.Type != "blob" {
		return nil, fmt.Errorf("not a blob url: %s", r.Raw)
	}
	if local := p.checkForLocalMapping(r); len(local) > 0 {
		return p.readLocalFile(ctx, r, local)
	}
	return p.readFile(ctx, r)
}

// ReadGitInfo implements the resourcehandlers.ResourceHandler#ReadGitInfo
func (p *GHC) ReadGitInfo(ctx context.Context, uri string) ([]byte, error) {
	r, err := p.getResolvedResourceInfo(ctx, uri)
	if err != nil {
		return nil, err
	}
	opts := &github.CommitsListOptions{
		Path: r.Path,
		SHA:  r.Ref,
	}
	var commits []*github.RepositoryCommit
	var resp *github.Response
	if commits, resp, err = p.client.Repositories.ListCommits(ctx, r.Owner, r.Repo, opts); err != nil {
		return nil, err
	}
	if resp != nil && resp.StatusCode >= 400 {
		return nil, fmt.Errorf("list commits for %s fails with HTTP status: %d", r.Raw, resp.StatusCode)
	}
	var blob []byte
	if commits != nil {
		gitInfo := transform(commits)
		if gitInfo == nil {
			return nil, nil
		}
		if len(r.SHA) > 0 {
			gitInfo.SHA = &r.SHA
		}
		if len(r.Ref) > 0 {
			gitInfo.SHAAlias = &r.Ref
		}
		if len(r.Path) > 0 {
			gitInfo.Path = &r.Path
		}
		if blob, err = marshallGitInfo(gitInfo); err != nil {
			return nil, err
		}
	}
	return blob, nil
}

// GetRawFormatLink implements the resourcehandlers.ResourceHandler#GetRawFormatLink
func (p *GHC) GetRawFormatLink(absLink string) (string, error) {
	r, err := resourcehandlers.BuildResourceInfo(absLink)
	if err != nil {
		return "", err
	}
	if !r.URL.IsAbs() {
		return absLink, nil // don't modify relative links
	}
	return r.GetRawURL(), nil
}

// GetClient implements the resourcehandlers.ResourceHandler#GetClient
func (p *GHC) GetClient() httpclient.Client {
	return p.httpClient
}

// GetRateLimit implements the resourcehandlers.ResourceHandler#GetRateLimit
func (p *GHC) GetRateLimit(ctx context.Context) (int, int, time.Time, error) {
	r, _, err := p.client.RateLimits(ctx)
	if err != nil {
		return -1, -1, time.Now(), err
	}
	return r.Core.Limit, r.Core.Remaining, r.Core.Reset.Time, nil
}

//==============================================================================================================

// checkForLocalMapping returns repository root on file system if local mapping configuration
// for the repository is set in config file or empty string otherwise.
func (p *GHC) checkForLocalMapping(r *resourcehandlers.ResourceInfo) string {
	key := strings.ToLower(r.GetRepoURL())
	if localPath, ok := p.localMappings[key]; ok {
		return localPath
	}
	// repo URLs keys in config file may end with '/'
	return p.localMappings[key+"/"]
}

// readFile reads a file from GitHub
func (p *GHC) readFile(ctx context.Context, r *resourcehandlers.ResourceInfo) ([]byte, error) {
	var cnt []byte
	// read using GitService and file URL -> file SHA mapping
	if SHA, ok := p.getFileSHA(r.Raw); ok {
		raw, resp, err := p.client.Git.GetBlobRaw(ctx, r.Owner, r.Repo, SHA)
		if err != nil {
			if resp != nil && resp.StatusCode == http.StatusNotFound {
				return nil, resourcehandlers.ErrResourceNotFound(r.Raw)
			}
			return nil, err
		}
		if resp != nil && resp.StatusCode >= 400 {
			return nil, fmt.Errorf("reading blob %s fails with HTTP status: %d", r.Raw, resp.StatusCode)
		}
		return raw, nil
	}
	// read using RepositoriesService.DownloadContents for non-markdown and non-manifest files - 2 manifestadapter calls
	opt := &github.RepositoryContentGetOptions{Ref: r.Ref}
	if !strings.HasSuffix(strings.ToLower(r.Path), ".md") && !strings.HasSuffix(strings.ToLower(r.Path), ".yaml") {
		return p.downloadContent(ctx, opt, r)
	}
	// read using RepositoriesService.GetContents for markdowns and module manifests - 1 manifestadapter call
	fc, _, resp, err := p.client.Repositories.GetContents(ctx, r.Owner, r.Repo, r.Path, opt)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, resourcehandlers.ErrResourceNotFound(r.Raw)
		}
		if resp != nil && resp.StatusCode == http.StatusForbidden {
			// if file is bigger than 1 MB -> content should be downloaded
			// it makes two additional manifestadapter cals, but it's unlikely to have large manifest.yaml
			return p.downloadContent(ctx, opt, r)
		}
		return nil, err
	}
	if resp != nil && resp.StatusCode >= 400 {
		return nil, fmt.Errorf("reading blob %s fails with HTTP status: %d", r.Raw, resp.StatusCode)
	}
	cnt, err = base64.StdEncoding.DecodeString(*fc.Content)
	if err != nil {
		return nil, err
	}
	return cnt, nil
}

// readLocalFile reads a file from FS
func (p *GHC) readLocalFile(_ context.Context, r *resourcehandlers.ResourceInfo, localPath string) ([]byte, error) {
	fn := filepath.Join(localPath, r.Path)
	cnt, err := p.os.ReadFile(fn)
	if err != nil {
		if p.os.IsNotExist(err) {
			return nil, resourcehandlers.ErrResourceNotFound(r.Raw)
		}
		return nil, fmt.Errorf("reading file %s for uri %s fails: %v", fn, r.Raw, err)
	}
	return cnt, nil
}

func (p *GHC) readLocalFileTree(r resourcehandlers.ResourceInfo, localPath string) []string {
	dirPath := filepath.Join(localPath, r.Path)
	files := []string{}
	filepath.Walk(dirPath, func(path string, info fs.FileInfo, err error) error {
		if !info.IsDir() && strings.HasSuffix(path, ".md") {
			files = append(files, strings.TrimPrefix(strings.TrimPrefix(path, dirPath), "/"))
		}
		return nil
	})
	return files
}

// downloadContent download file content like: github.Client.Repositories#DownloadContents, but with different error handling
func (p *GHC) downloadContent(ctx context.Context, opt *github.RepositoryContentGetOptions, r *resourcehandlers.ResourceInfo) ([]byte, error) {
	dir := path.Dir(r.Path)
	filename := path.Base(r.Path)
	dirContents, resp, err := p.getDirContents(ctx, r.Owner, r.Repo, dir, opt)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, resourcehandlers.ErrResourceNotFound(r.Raw)
		}
		return nil, err
	}
	for _, contents := range dirContents {
		if *contents.Name == filename {
			if contents.SHA == nil || *contents.SHA == "" {
				return nil, fmt.Errorf("no SHA found for %s", r.Raw)
			}
			var cnt []byte
			cnt, resp, err = p.client.Git.GetBlobRaw(ctx, r.Owner, r.Repo, *contents.SHA)
			if err != nil {
				if resp != nil && resp.StatusCode == http.StatusNotFound {
					return nil, resourcehandlers.ErrResourceNotFound(r.Raw)
				}
				return nil, err
			}
			if resp != nil && resp.StatusCode >= 400 {
				return nil, fmt.Errorf("content download for %s fails with HTTP status: %d", r.Raw, resp.StatusCode)
			}
			return cnt, nil
		}
	}
	// not found
	return nil, resourcehandlers.ErrResourceNotFound(r.Raw)
}

// wraps github.Client Repositories.GetContents and synchronize the access to avoid 'unexpected EOF' errors when reading directory content
func (p *GHC) getDirContents(ctx context.Context, owner, repo, path string, opts *github.RepositoryContentGetOptions) (dc []*github.RepositoryContent, resp *github.Response, err error) {
	p.muxCnt.Lock()
	defer p.muxCnt.Unlock()
	_, dc, resp, err = p.client.Repositories.GetContents(ctx, owner, repo, path, opts)
	return
}

// getTree returns subtree with root r#Path
func (p *GHC) getTree(ctx context.Context, r *resourcehandlers.ResourceInfo, recursive bool) (*github.Tree, error) {
	sha := fmt.Sprintf("%s:%s", r.Ref, r.Path)
	sha = url.PathEscape(sha)
	gitTree, resp, err := p.client.Git.GetTree(ctx, r.Owner, r.Repo, sha, recursive)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, resourcehandlers.ErrResourceNotFound(r.Raw)
		}
		return nil, err
	}
	if resp != nil && resp.StatusCode >= 400 {
		return nil, fmt.Errorf("reading tree %s fails with HTTP status: %d", r.Raw, resp.StatusCode)
	}
	return gitTree, nil
}

// buildAbsLink builds absolute link if <link> is relative using <source> as a base
// resourcehandlers.ErrResourceNotFound if target resource doesn't exist
func (p *GHC) buildAbsLink(source *resourcehandlers.ResourceInfo, link string) (string, error) {
	l, err := url.Parse(strings.TrimSuffix(link, "/"))
	if err != nil {
		return "", err
	}
	if l.IsAbs() {
		return link, nil // already absolute
	}
	// build URL based on source path
	var u *url.URL
	if u, err = url.Parse("/" + source.Path); err != nil {
		return "", err
	}
	if u, err = u.Parse(l.Path); err != nil {
		return "", err
	}
	// determine the type of the resource: (blob|tree)
	var tp string
	if tp, err = p.determineLinkType(source, u); err != nil {
		return tp, err
	}
	res, err := url.Parse(source.URL.String())
	if err != nil {
		return "", err
	}
	// set path
	res.Path = fmt.Sprintf("/%s/%s/%s/%s%s", source.Owner, source.Repo, tp, source.Ref, u.Path)
	// set query & fragment
	res.ForceQuery = l.ForceQuery
	res.RawQuery = l.RawQuery
	res.Fragment = l.Fragment
	return res.String(), nil
}

// determineLinkType returns the type of relative link (blob|tree)
// resourcehandlers.ErrResourceNotFound if target resource doesn't exist
func (p *GHC) determineLinkType(source *resourcehandlers.ResourceInfo, rel *url.URL) (string, error) {
	var tp string
	var err error
	gtp := "tree" // guess the type of resource
	if len(path.Ext(rel.Path)) > 0 {
		gtp = "blob"
	}
	expURI := fmt.Sprintf("%s://%s/%s/%s/%s/%s%s", source.URL.Scheme, source.URL.Host, source.Owner, source.Repo, gtp, source.Ref, rel.Path)
	// local case
	if local := p.checkForLocalMapping(source); len(local) > 0 {
		fn := filepath.Join(local, rel.Path)
		var info os.FileInfo
		info, err = p.os.Lstat(fn)
		if err != nil {
			if p.os.IsNotExist(err) {
				return expURI, resourcehandlers.ErrResourceNotFound(expURI)
			}
			return "", fmt.Errorf("cannot determine resource type for path %s and source %s: %v", rel.Path, source.Raw, err)
		}
		tp = "blob"
		if info.IsDir() {
			tp = "tree"
		}
		return tp, nil
	}
	// list remote repo
	key := fmt.Sprintf("%s://%s/%s/%s/blob/%s%s", source.URL.Scheme, source.URL.Host, source.Owner, source.Repo, source.Ref, rel.Path)
	if _, ok := p.getFileSHA(key); ok {
		tp = "blob" // as file SHA is cached, type is blob
	} else {
		opt := &github.RepositoryContentGetOptions{Ref: source.Ref}
		dir := path.Dir(rel.Path)
		name := path.Base(rel.Path)
		var dc []*github.RepositoryContent
		var resp *github.Response
		p.muxSHA.Lock()
		defer p.muxSHA.Unlock()
		dc, resp, err = p.getDirContents(context.Background(), source.Owner, source.Repo, dir, opt)
		if err != nil {
			if resp != nil && resp.StatusCode == http.StatusNotFound { // parent folder doesn't exist
				uri := fmt.Sprintf("%s://%s/%s/%s/tree/%s%s", source.URL.Scheme, source.URL.Host, source.Owner, source.Repo, source.Ref, dir)
				return expURI, resourcehandlers.ErrResourceNotFound(uri)
			}
			return "", fmt.Errorf("cannot determine resource type for path %s and source %s: %v", rel.Path, source.Raw, err)
		}
		for _, d := range dc {
			// add files SHA
			if *d.Type == "blob" {
				p.filesCache[*d.HTMLURL] = *d.SHA
			}
			if *d.Name == name {
				if *d.Type == "file" {
					tp = "blob"
				} else {
					tp = "tree"
				}
			}
		}
	}
	if tp == "" { // resource doesn't exist
		return expURI, resourcehandlers.ErrResourceNotFound(expURI)
	}
	return tp, nil
}

// getResourceInfo build ResourceInfo and resolves 'DEFAULT_BRANCH' to repo default branch
func (p *GHC) getResolvedResourceInfo(ctx context.Context, uri string) (*resourcehandlers.ResourceInfo, error) {
	r, err := resourcehandlers.BuildResourceInfo(uri)
	if err != nil {
		return nil, err
	}
	if r.Ref == "DEFAULT_BRANCH" {
		defaultBranch, err := p.getDefaultBranch(ctx, r.Owner, r.Repo)
		if err != nil {
			return nil, err
		}
		r.Ref = defaultBranch
	}
	return r, nil
}

// getDefaultBranch gets the default branch for given repo
func (p *GHC) getDefaultBranch(ctx context.Context, owner string, repository string) (string, error) {
	p.muxDefBr.Lock()
	defer p.muxDefBr.Unlock()
	key := fmt.Sprintf("%s/%s", owner, repository)
	if def, ok := p.defBranches[key]; ok {
		return def, nil
	}
	repo, _, err := p.client.Repositories.Get(ctx, owner, repository)
	if err != nil {
		return "", err
	}
	def := repo.GetDefaultBranch()
	p.defBranches[key] = def
	return def, nil
}

func (p *GHC) getFileSHA(key string) (string, bool) {
	p.muxSHA.RLock()
	defer p.muxSHA.RUnlock()
	val, ok := p.filesCache[key]
	return val, ok
}

// transform builds git.Info from a commits list
func transform(commits []*github.RepositoryCommit) *GitInfo {
	if commits == nil {
		return nil
	}
	gitInfo := &GitInfo{}
	var nonInternalCommits []*github.RepositoryCommit
	// skip internal commits
	for _, commit := range commits {
		if !isInternalCommit(commit) {
			nonInternalCommits = append(nonInternalCommits, commit)
		}
	}
	if len(nonInternalCommits) == 0 {
		return nil
	}
	sort.Slice(nonInternalCommits, func(i, j int) bool {
		return nonInternalCommits[i].GetCommit().GetCommitter().GetDate().After(nonInternalCommits[j].GetCommit().GetCommitter().GetDate())
	})

	lastModifiedDate := nonInternalCommits[0].GetCommit().GetCommitter().GetDate().Format(DateFormat)
	gitInfo.LastModifiedDate = &lastModifiedDate
	webURL := nonInternalCommits[0].GetHTMLURL()
	webURL = strings.Split(webURL, "/commit/")[0]
	gitInfo.WebURL = &webURL

	publishDate := commits[len(nonInternalCommits)-1].GetCommit().GetCommitter().GetDate().Format(DateFormat)
	gitInfo.PublishDate = &publishDate

	if gitInfo.Author = getCommitAuthor(nonInternalCommits[len(nonInternalCommits)-1]); gitInfo.Author == nil {
		klog.Warningf("cannot get commit author")
	}
	if len(nonInternalCommits) > 1 {
		gitInfo.Contributors = []*github.User{}
		var registered []string
		for _, commit := range nonInternalCommits {
			var contributor *github.User
			if contributor = getCommitAuthor(commit); contributor == nil {
				continue
			}
			if contributor.GetType() == "User" && contributor.GetEmail() != gitInfo.Author.GetEmail() && !contains(registered, contributor.GetEmail()) {
				gitInfo.Contributors = append(gitInfo.Contributors, contributor)
				registered = append(registered, contributor.GetEmail())
			}
		}
	}

	return gitInfo
}

func contains(slice []string, s string) bool {
	for _, _s := range slice {
		if s == _s {
			return true
		}
	}
	return false
}

// marshallGitInfo serializes git.Info to byte array
func marshallGitInfo(gitInfo *GitInfo) ([]byte, error) {
	blob, err := json.MarshalIndent(gitInfo, "", "  ")
	if err != nil {
		return nil, err
	}
	return blob, nil
}

func isInternalCommit(commit *github.RepositoryCommit) bool {
	message := commit.GetCommit().GetMessage()
	email := commit.GetCommitter().GetEmail()
	return strings.HasPrefix(message, "[int]") ||
		strings.Contains(message, "[skip ci]") ||
		strings.HasPrefix(email, "gardener.ci") ||
		strings.HasPrefix(email, "gardener.opensource")
}

func mergeAuthors(author *github.User, commitAuthor *github.CommitAuthor) *github.User {
	if author == nil {
		author = &github.User{}
	}
	if commitAuthor != nil {
		author.Name = commitAuthor.Name
		author.Email = commitAuthor.Email
	}
	return author
}

func getCommitAuthor(commit *github.RepositoryCommit) *github.User {
	var contributor *github.User
	if contributor = commit.GetAuthor(); contributor != nil && commit.GetCommit().GetAuthor() != nil {
		contributor = mergeAuthors(contributor, commit.GetCommit().GetAuthor())
	}
	if contributor == nil && commit.GetCommit().GetAuthor() != nil {
		contributor = mergeAuthors(&github.User{}, commit.GetCommit().GetAuthor())
	}
	if contributor == nil && commit.GetCommit().GetCommitter() != nil {
		contributor = mergeAuthors(&github.User{}, commit.GetCommit().GetCommitter())
	}
	return contributor
}
