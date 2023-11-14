// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package repositoryhosts

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

// ResourceInfo represents a GitHub resource URL
type ResourceInfo struct {
	Raw   string
	URL   *url.URL
	IsRaw bool
	Owner string
	Repo  string
	Type  string
	Ref   string
	Path  string
	SHA   string
}

// BuildResourceInfo creates ResourceInfo for given URL
func BuildResourceInfo(uri string) (*ResourceInfo, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}
	// 'github.com' && 'github.tools.sap' uses host 'raw' prefix
	// https://raw.githubusercontent.com/gardener/gardener/master/logo/gardener-large.png
	// https://raw.github.tools.sap/kubernetes/documentation/master/images/overview.png
	// 'github.wdf.sap.corp' uses path prefix 'raw' prefix
	// https://github.wdf.sap.corp/raw/CPET/kube-hpa/master/images/HPA_CustomMetrics.png
	// Enterprise GitHubs URLs requires query '?token=XXXXXXXXXXXXXXXXXXXXXXXXXXXXX', otherwise they don't work
	// All GitHubs supports '?raw=true' query & using 'raw' instead of 'blob' in GitHub resource URL:
	// https://github.com/gardener/gardener/blob/master/logo/gardener-large.png?raw=true
	// https://github.com/gardener/gardener/raw/master/logo/gardener-large.png
	r := &ResourceInfo{Raw: uri, URL: u}
	r.IsRaw = strings.HasPrefix(u.Host, "raw.") || strings.HasPrefix(u.Path, "/raw/")
	rPath := u.Path
	if r.IsRaw {
		rPath = strings.TrimPrefix(u.Path, "/raw/") // github.wdf.sap.corp case
	}
	rPath = strings.TrimPrefix(rPath, "/")
	rPath = strings.TrimSuffix(rPath, "/")
	segments := strings.Split(rPath, "/")
	if len(segments) > 0 {
		r.Owner = segments[0]
		if len(segments) > 1 {
			r.Repo = segments[1]
		}
		if len(segments) > 2 {
			if r.IsRaw {
				r.Ref = segments[2]
			} else {
				r.Type = segments[2]
			}
		}
		if len(segments) > 3 {
			if r.IsRaw {
				r.Path = strings.Join(segments[3:], "/")
			} else {
				r.Ref = segments[3]
			}
		}
		if len(segments) > 4 {
			if !r.IsRaw {
				r.Path = strings.Join(segments[4:], "/")
			}
		}
		if r.IsRaw {
			r.Type = "blob"
		}
		if r.Type == "raw" {
			r.Type = "blob"
			r.IsRaw = true
		}
	}
	return r, nil
}

// GetURL returns the url
func (ri *ResourceInfo) GetURL() string {
	return fmt.Sprintf("%s://%s/%s/%s/%s/%s/%s", ri.URL.Scheme, ri.URL.Host, ri.Owner, ri.Repo, ri.Type, ri.Ref, ri.Path)
}

// GetRepoURL returns the GitHub repository URL
func (ri *ResourceInfo) GetRepoURL() string {
	return fmt.Sprintf("%s://%s/%s/%s", ri.URL.Scheme, ri.URL.Host, ri.Owner, ri.Repo)
}

// GetRawURL returns the GitHub raw URL if the resource is 'blob', otherwise returns the origin URL
func (ri *ResourceInfo) GetRawURL() string {
	if ri.IsRaw || ri.Type != "blob" { // if already raw or if not a blob -> return without modification
		return ri.Raw
	}
	return fmt.Sprintf("%s://%s/%s/%s/raw/%s/%s", ri.URL.Scheme, ri.URL.Host, ri.Owner, ri.Repo, ri.Ref, ri.Path)
}

// GetResourceName returns the name of the resource (including extension), if resource path is empty returns '.'
func (ri *ResourceInfo) GetResourceName() string {
	return path.Base(ri.Path)
}

// GetResourceExt returns the resource name extension, empty string if when no extension exists
func (ri *ResourceInfo) GetResourceExt() string {
	return path.Ext(ri.Path)
}
