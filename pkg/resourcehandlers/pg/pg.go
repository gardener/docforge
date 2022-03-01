// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/util"
	"github.com/gardener/docforge/pkg/util/httpclient"
	"github.com/gardener/docforge/pkg/util/osshim"
	"github.com/google/go-github/v43/github"
	"github.com/hashicorp/go-multierror"
	"io/fs"
	"io/ioutil"
	"k8s.io/klog/v2"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// PG implements resourcehandlers.ResourceHandler interface using GitHub API with transport level persistent cache.
type PG struct {
	client        *github.Client
	httpClient    *http.Client
	os            osshim.Os
	acceptedHosts []string
	localMappings map[string]string
	flagVars      map[string]string
	filesSHA      map[string]string
	muxSHA        sync.RWMutex
	defBranches   map[string]string
	muxDefBr      sync.Mutex
}

// NewPG creates new PG resource handler
func NewPG(client *github.Client, httpClient *http.Client, os osshim.Os, acceptedHosts []string, localMappings map[string]string, flagVars map[string]string) resourcehandlers.ResourceHandler {
	return &PG{
		client:        client,
		httpClient:    httpClient,
		os:            os,
		acceptedHosts: acceptedHosts,
		localMappings: localMappings,
		flagVars:      flagVars,
		filesSHA:      make(map[string]string),
		defBranches:   make(map[string]string),
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

//========================= resourcehandlers.ResourceHandler ===================================================

// Accept implements the resourcehandlers.ResourceHandler#Accept
func (p *PG) Accept(uri string) bool {
	r, err := util.BuildResourceInfo(uri)
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

// ResolveDocumentation implements the resourcehandlers.ResourceHandler#ResolveDocumentation
func (p *PG) ResolveDocumentation(ctx context.Context, uri string) (*api.Documentation, error) {
	r, err := p.getResolvedResourceInfo(ctx, uri)
	if err != nil {
		return nil, err
	}
	if r.Type != "blob" {
		return nil, fmt.Errorf("not a blob url: %s", r.Raw)
	}
	var cnt []byte
	if local := p.checkForLocalMapping(r); len(local) > 0 {
		if cnt, err = p.readLocalFile(ctx, r, local); err != nil {
			return nil, err
		}
	} else {
		if cnt, err = p.readFile(ctx, r); err != nil {
			return nil, err
		}
	}
	var doc *api.Documentation
	if doc, err = api.ParseWithMetadata(cnt, r.Ref, p.flagVars); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %s. %+v", uri, err)
	}
	n := &api.Node{Nodes: doc.Structure, NodeSelector: doc.NodeSelector}
	n.SetParentsDownwards()
	if err = p.resolveManifestRelativePaths(n, r); err != nil {
		return nil, err
	}
	for _, el := range doc.Structure {
		el.SetParent(nil)
	}
	return doc, nil
}

// ResolveNodeSelector implements the resourcehandlers.ResourceHandler#ResolveNodeSelector
func (p *PG) ResolveNodeSelector(ctx context.Context, node *api.Node) ([]*api.Node, error) {
	r, err := p.getResolvedResourceInfo(ctx, node.NodeSelector.Path)
	if err != nil {
		return nil, err
	}
	if r.Type != "tree" {
		return nil, fmt.Errorf("not a tree url: %s", r.Raw)
	}
	// prepare path filters
	var pfs []*regexp.Regexp
	if len(node.NodeSelector.ExcludePaths) > 0 {
		for _, ep := range node.NodeSelector.ExcludePaths {
			var rgx *regexp.Regexp
			if rgx, err = regexp.Compile(ep); err != nil {
				return nil, fmt.Errorf("manifest %s with invalid path exclude expression %s: %w", r.Raw, ep, err)
			}
			pfs = append(pfs, rgx)
		}
	}
	// prepare source prefixes
	srcBlobPrefix := fmt.Sprintf("%s://%s/%s/%s/blob/%s/%s", r.URL.Scheme, r.URL.Host, r.Owner, r.Repo, r.Ref, r.Path)
	srcTreePrefix := fmt.Sprintf("%s://%s/%s/%s/tree/%s/%s", r.URL.Scheme, r.URL.Host, r.Owner, r.Repo, r.Ref, r.Path)
	var vr *api.Node
	if local := p.checkForLocalMapping(r); len(local) > 0 {
		if vr, err = p.getLocalNodeSelectorTree(ctx, node, r, pfs, local, srcTreePrefix, srcBlobPrefix); err != nil {
			return nil, err
		}
	} else {
		if vr, err = p.getNodeSelectorTree(ctx, node, r, pfs, srcTreePrefix, srcBlobPrefix); err != nil {
			return nil, err
		}
	}
	vr.SetParentsDownwards()
	vr.Cleanup()
	vr.Sort()
	for _, cn := range vr.Nodes {
		cn.SetParent(nil)
	}
	return vr.Nodes, nil
}

// Read implements the resourcehandlers.ResourceHandler#Read
func (p *PG) Read(ctx context.Context, uri string) ([]byte, error) {
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
func (p *PG) ReadGitInfo(ctx context.Context, uri string) ([]byte, error) {
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

// ResourceName implements the resourcehandlers.ResourceHandler#ResourceName
func (p *PG) ResourceName(link string) (string, string) {
	r, err := util.BuildResourceInfo(link)
	if err != nil {
		return "", ""
	}
	ext := r.GetResourceExt()
	name := strings.TrimSuffix(r.GetResourceName(), ext)
	return name, ext
}

// BuildAbsLink implements the resourcehandlers.ResourceHandler#BuildAbsLink
func (p *PG) BuildAbsLink(source, link string) (string, error) {
	r, err := util.BuildResourceInfo(source)
	if err != nil {
		return "", err
	}
	return p.buildAbsLink(r, link)
}

// GetRawFormatLink implements the resourcehandlers.ResourceHandler#GetRawFormatLink
func (p *PG) GetRawFormatLink(absLink string) (string, error) {
	r, err := util.BuildResourceInfo(absLink)
	if err != nil {
		return "", err
	}
	if !r.URL.IsAbs() {
		return absLink, nil // don't modify relative links
	}
	return r.GetRawURL(), nil
}

// GetClient implements the resourcehandlers.ResourceHandler#GetClient
func (p *PG) GetClient() httpclient.Client {
	return p.httpClient
}

// GetRateLimit implements the resourcehandlers.ResourceHandler#GetRateLimit
func (p *PG) GetRateLimit(ctx context.Context) (int, int, time.Time, error) {
	r, _, err := p.client.RateLimits(ctx)
	if err != nil {
		return -1, -1, time.Now(), err
	}
	return r.Core.Limit, r.Core.Remaining, r.Core.Reset.Time, nil
}

//==============================================================================================================

// checkForLocalMapping returns repository root on file system if local mapping configuration
// for the repository is set in config file or empty string otherwise.
func (p *PG) checkForLocalMapping(r *util.ResourceInfo) string {
	key := strings.ToLower(r.GetRepoURL())
	if localPath, ok := p.localMappings[key]; ok {
		return localPath
	}
	// repo URLs keys in config file may end with '/'
	return p.localMappings[key+"/"]
}

// readFile reads a file from GitHub
func (p *PG) readFile(ctx context.Context, r *util.ResourceInfo) ([]byte, error) {
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
	// read using RepositoriesService.DownloadContents for non-markdown and non-manifest files - 2 API calls
	opt := &github.RepositoryContentGetOptions{Ref: r.Ref}
	if !strings.HasSuffix(strings.ToLower(r.Path), ".md") && !strings.HasSuffix(strings.ToLower(r.Path), ".yaml") {
		return p.downloadContent(ctx, opt, r)
	}
	// read using RepositoriesService.GetContents for markdowns and module manifests - 1 API call
	fc, _, resp, err := p.client.Repositories.GetContents(ctx, r.Owner, r.Repo, r.Path, opt)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, resourcehandlers.ErrResourceNotFound(r.Raw)
		}
		if resp != nil && resp.StatusCode == http.StatusForbidden {
			// if file is bigger than 1 MB -> content should be downloaded
			// it makes two additional API cals, but it's unlikely to have large manifest.yaml
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
func (p *PG) readLocalFile(_ context.Context, r *util.ResourceInfo, localPath string) ([]byte, error) {
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

// downloadContent download file content like: github.Client.Repositories#DownloadContents, but with different error handling
func (p *PG) downloadContent(ctx context.Context, opt *github.RepositoryContentGetOptions, r *util.ResourceInfo) ([]byte, error) {
	dir := path.Dir(r.Path)
	filename := path.Base(r.Path)
	_, dirContents, resp, err := p.client.Repositories.GetContents(ctx, r.Owner, r.Repo, dir, opt)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, resourcehandlers.ErrResourceNotFound(r.Raw)
		}
		return nil, err
	}
	var dlResp *http.Response
	for _, contents := range dirContents {
		if *contents.Name == filename {
			if contents.DownloadURL == nil || *contents.DownloadURL == "" {
				return nil, fmt.Errorf("no download link found for %s", r.Raw)
			}
			dlResp, err = p.httpClient.Get(*contents.DownloadURL)
			if err != nil {
				return nil, err
			}
			break
		}
	}
	if dlResp == nil {
		return nil, resourcehandlers.ErrResourceNotFound(r.Raw)
	}
	defer func() {
		_ = dlResp.Body.Close()
	}()
	if dlResp.StatusCode >= 400 {
		return nil, fmt.Errorf("content download for %s fails with HTTP status: %d", r.Raw, resp.StatusCode)
	}
	var cnt []byte
	cnt, err = ioutil.ReadAll(dlResp.Body)
	return cnt, err
}

// getNodeSelectorTree returns nodes selected by api.NodeSelector with GitHub backend
func (p *PG) getNodeSelectorTree(ctx context.Context, node *api.Node, r *util.ResourceInfo, pfs []*regexp.Regexp, tPrefix string, bPrefix string) (*api.Node, error) {
	recursive := node.NodeSelector.Depth != 1
	t, err := p.getTree(ctx, r, recursive)
	if err != nil {
		return nil, err
	}
	vr := &api.Node{Name: "vRoot", Properties: make(map[string]interface{})}
	vr.Properties[api.ContainerNodeSourceLocation] = tPrefix
	for _, e := range t.Entries {
		ePath := strings.TrimPrefix(*e.Path, "/")
		// add files SHA
		if *e.Type == "blob" {
			key := fmt.Sprintf("%s/%s", bPrefix, ePath)
			p.setFileSHA(key, *e.SHA)
		}
		// skip node if it is not a markdown file
		if *e.Type != "blob" || !strings.HasSuffix(strings.ToLower(ePath), ".md") {
			klog.V(6).Infof("node selector %s skip entry %s\n", node.NodeSelector.Path, ePath)
			continue
		}
		if filterPath(node, pfs, ePath) {
			continue
		}
		buildNode(vr, bPrefix, ePath)
	}
	return vr, nil
}

// getNodeSelectorTree returns nodes selected by api.NodeSelector with mapped on FS local repository
func (p *PG) getLocalNodeSelectorTree(_ context.Context, node *api.Node, r *util.ResourceInfo, pfs []*regexp.Regexp, localPath string, tPrefix string, bPrefix string) (*api.Node, error) {
	dn := filepath.Join(localPath, r.Path)
	info, err := p.os.Lstat(dn)
	if err != nil {
		if p.os.IsNotExist(err) {
			return nil, resourcehandlers.ErrResourceNotFound(r.Raw)
		}
		return nil, fmt.Errorf("nodeSelector path stats %s for uri %s and node %s fails: %v", dn, r.Raw, node.Path("/"), err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("nodeSelector path %s for uri %s and node %s is not a directory", dn, r.Raw, node.Path("/"))
	}
	vr := &api.Node{Name: "vRoot", Properties: make(map[string]interface{})}
	vr.Properties[api.ContainerNodeSourceLocation] = tPrefix
	err = filepath.WalkDir(dn, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		lPath := filepath.ToSlash(strings.TrimPrefix(path, dn))
		lPath = strings.TrimPrefix(lPath, "/")
		// skip entry if it is not a markdown
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(lPath), ".md") {
			klog.V(6).Infof("node selector %s skip entry %s\n", node.NodeSelector.Path, lPath)
			return nil
		}
		if filterPath(node, pfs, lPath) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		buildNode(vr, bPrefix, lPath)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error walking nodeSelector path %s for uri %s and node %s: %v", dn, r.Raw, node.Path("/"), err)
	}
	return vr, nil
}

// getTree returns subtree with root r#Path
func (p *PG) getTree(ctx context.Context, r *util.ResourceInfo, recursive bool) (*github.Tree, error) {
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

// resolveManifestRelativePaths resolves relative paths in module manifest
func (p *PG) resolveManifestRelativePaths(node *api.Node, r *util.ResourceInfo) error {
	var errs error
	if node.Source != "" {
		u, err := url.Parse(node.Source)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("manifest %s with invalid node %s source: %s", r.Raw, node.FullName("/"), node.Source))
		} else if !u.IsAbs() {
			// resolve relative path
			if node.Source, err = p.buildAbsLink(r, node.Source); err != nil {
				errs = multierror.Append(errs, fmt.Errorf("cannot resolve source relative path %s in node %s and manifest %s", node.Source, node.FullName("/"), r.Raw))
			}
		}
	}
	if len(node.MultiSource) > 0 {
		for idx, src := range node.MultiSource {
			u, err := url.Parse(src)
			if err != nil {
				errs = multierror.Append(errs, fmt.Errorf("manifest %s with invalid node %s multiSource[%d]: %s", r.Raw, node.FullName("/"), idx, node.MultiSource[idx]))
			} else if !u.IsAbs() {
				// resolve relative path
				if node.Source, err = p.buildAbsLink(r, src); err != nil {
					errs = multierror.Append(errs, fmt.Errorf("cannot resolve multiSource[%d] relative path %s in node %s and manifest %s", idx, node.MultiSource[idx], node.FullName("/"), r.Raw))
				}
			}
		}
	}
	if node.NodeSelector != nil {
		u, err := url.Parse(node.NodeSelector.Path)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("manifest %s with invalid nodeSelector path %s in node %s", r.Raw, node.NodeSelector.Path, node.FullName("/")))
		} else if !u.IsAbs() {
			// resolve relative path
			if node.NodeSelector.Path, err = p.buildAbsLink(r, node.NodeSelector.Path); err != nil {
				errs = multierror.Append(errs, fmt.Errorf("cannot resolve nodeSelector relative path %s in node %s and manifest %s", node.NodeSelector.Path, node.FullName("/"), r.Raw))
			}
		}
	}
	for _, n := range node.Nodes {
		if err := p.resolveManifestRelativePaths(n, r); err != nil {
			errs = multierror.Append(errs, err)
		}
	}
	return errs
}

// buildAbsLink builds absolute link if <link> is relative using <source> as a base
// resourcehandlers.ErrResourceNotFound if target resource doesn't exist
func (p *PG) buildAbsLink(source *util.ResourceInfo, link string) (string, error) {
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
	res, _ := url.Parse(source.URL.String())
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
func (p *PG) determineLinkType(source *util.ResourceInfo, rel *url.URL) (string, error) {
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
		_, dc, resp, err = p.client.Repositories.GetContents(context.Background(), source.Owner, source.Repo, dir, opt)
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
				p.setFileSHA(*d.HTMLURL, *d.SHA)
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
func (p *PG) getResolvedResourceInfo(ctx context.Context, uri string) (*util.ResourceInfo, error) {
	r, err := util.BuildResourceInfo(uri)
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
func (p *PG) getDefaultBranch(ctx context.Context, owner string, repository string) (string, error) {
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

func (p *PG) setFileSHA(key, value string) {
	p.muxSHA.Lock()
	defer p.muxSHA.Unlock()
	p.filesSHA[key] = value
}

func (p *PG) getFileSHA(key string) (string, bool) {
	p.muxSHA.RLock()
	defer p.muxSHA.RUnlock()
	val, ok := p.filesSHA[key]
	return val, ok
}

// filterPath returns true if path is filtered by api.NodeSelector Depth or ExcludePaths
func filterPath(node *api.Node, pfs []*regexp.Regexp, path string) bool {
	// depth filter
	depth := strings.Count(path, "/") // depth is 1 for zero '/' 2 for one '/' etc.
	if node.NodeSelector.Depth > 0 && depth+1 > int(node.NodeSelector.Depth) {
		klog.V(6).Infof("node selector %s entry %s depth (%d) filter applied\n", node.NodeSelector.Path, path, node.NodeSelector.Depth)
		return true
	}
	// path filter
	var rgx *regexp.Regexp
	for _, pf := range pfs {
		if rgx.Match([]byte(path)) {
			rgx = pf
			break
		}
	}
	if rgx != nil {
		klog.V(6).Infof("node selector %s entry %s path filter %s applied\n", node.NodeSelector.Path, path, rgx.String())
		return true
	}
	return false
}

// buildNode creates new api.Node for the rPath markdown and adds it to the vRoot tree
func buildNode(vRoot *api.Node, bPrefix string, rPath string) {
	// build node
	n := &api.Node{Source: fmt.Sprintf("%s/%s", bPrefix, rPath)}
	n.Name = path.Base(rPath)
	loc := path.Dir(rPath)
	if loc == "." { // append in the root
		n.SetParent(vRoot)
		vRoot.Nodes = append(vRoot.Nodes, n)
		return
	}
	// find the location in the tree
	parent := vRoot
	ls := strings.Split(loc, "/")
	for _, l := range ls {
		var found bool
		for _, pn := range parent.Nodes {
			if pn.Name == l {
				found = true
				parent = pn
			}
		}
		if !found {
			// create missing container api.Node
			dn := &api.Node{Name: l}
			dn.Properties = make(map[string]interface{})
			dn.Properties[api.ContainerNodeSourceLocation] = fmt.Sprintf("%s/%s", parent.Properties[api.ContainerNodeSourceLocation], l)
			dn.SetParent(parent)
			parent.Nodes = append(parent.Nodes, dn)
			parent = dn
		}
	}
	n.SetParent(parent)
	parent.Nodes = append(parent.Nodes, n)
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
