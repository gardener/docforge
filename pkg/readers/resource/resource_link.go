// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"fmt"
	"net/url"
	"regexp"
)

// Resource represents a GitHub resource URL
type Resource struct {
	url.URL
	Host  string
	Owner string
	Repo  string
	Type  string
	Ref   string
	Path  string
}

var (
	rawPrefixed       = regexp.MustCompile(`https://([^/]+)/raw/([^/]+)/([^/]+)/([^/]+)/([^\?#]+).*`)
	resource          = regexp.MustCompile(`https://([^/]+)/([^/]+)/([^/]+)/([^/]+)/([^/]+)/([^\?#]+).*`)
	githubusercontent = regexp.MustCompile(`https://raw.githubusercontent.com/([^/]+)/([^/]+)/([^/]+)/([^\?#]+).*`)
	other             = regexp.MustCompile(`([^\?#]*).*`)
)

// NewResource creates new resource from url as string
func NewResource(URL string) (Resource, error) {
	u, err := url.Parse(URL)
	if err != nil {
		return Resource{}, err
	}
	return NewResourceFromURL(u)
}

// NewResourceFromURL creates new resource from url object
func NewResourceFromURL(u *url.URL) (Resource, error) {
	if u.String() == "" {
		return Resource{}, nil
	}
	components := rawPrefixed.FindStringSubmatch(u.String())
	if components != nil {
		return Resource{
			URL:   *u,
			Host:  components[1],
			Owner: components[2],
			Repo:  components[3],
			Type:  "raw",
			Ref:   components[4],
			Path:  components[5],
		}, nil
	}
	components = githubusercontent.FindStringSubmatch(u.String())
	if components != nil {
		return Resource{
			URL:   *u,
			Host:  "github.com",
			Owner: components[1],
			Repo:  components[2],
			Type:  "raw",
			Ref:   components[3],
			Path:  components[4],
		}, nil
	}
	components = resource.FindStringSubmatch(u.String())
	if components != nil {
		return Resource{
			URL:   *u,
			Host:  components[1],
			Owner: components[2],
			Repo:  components[3],
			Type:  components[4],
			Ref:   components[5],
			Path:  components[6],
		}, nil
	}
	components = other.FindStringSubmatch(u.String())
	if components != nil {
		return Resource{
			URL:  *u,
			Path: components[1],
		}, nil
	}
	return Resource{}, fmt.Errorf("unknown link type for resource %s", u.String())
}

// ToResourceURL returns the u
func (r *Resource) ToResourceURL() (string, error) {
	if r.Host == "" {
		return "", fmt.Errorf("can't convert to resource URL, %s is relative", r.Path)
	}
	return fmt.Sprintf("https://%s/%s/%s/%s/%s/%s", r.Host, r.Owner, r.Repo, r.Type, r.Ref, r.Path), nil
}

// TotRepoURL returns the GitHub repository URL
func (r *Resource) TotRepoURL() (string, error) {
	if r.Host == "" {
		return "", fmt.Errorf("can't convert to repo URL, %s is relative", r.Path)
	}
	return fmt.Sprintf("https://%s/%s/%s", r.Host, r.Owner, r.Repo), nil
}

// ToRawURL returns the GitHub raw URL if the resource is 'blob', otherwise returns the origin URL
func (r *Resource) ToRawURL() (string, error) {
	if r.Host == "" {
		return "", fmt.Errorf("can't convert to raw URL, %s is relative", r.Path)
	}
	return fmt.Sprintf("https://%s/%s/%s/raw/%s/%s", r.Host, r.Owner, r.Repo, r.Ref, r.Path), nil
}
