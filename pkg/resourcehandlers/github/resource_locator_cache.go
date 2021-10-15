// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/google/go-github/v32/github"
)

// Cache is indexes GitHub TreeEntries by website resource URLs as keys,
// mapping ResourceLocator objects to them.
type Cache struct {
	cache    map[string]*ResourceLocator
	mux      sync.RWMutex
	ghClient *github.Client
}

// Get returns a ResourceLocator object mapped to the path (URL).
// If mapping does not exist nil is returned.
func (c *Cache) Get(rl *ResourceLocator) (res *ResourceLocator, err error) {
	var path string
	if path, err = c.key(rl, false); err != nil {
		return
	}
	res, _ = c.get(path)
	return
}

// GetWithInit returns a ResourceLocator object mapped to the path (URL)
// Corresponding repository index will be added in the cache, if it has not already been added.
// If the entry doesn't exist #resourcehandlers.ErrResourceNotFound error is returned.
func (c *Cache) GetWithInit(ctx context.Context, rl *ResourceLocator) (res *ResourceLocator, err error) {
	var (
		path string
		ok   bool
	)
	if path, err = c.key(rl, false); err != nil {
		return
	}
	if res, ok = c.get(path); !ok {
		// init repository
		if _, err = c.set(ctx, rl); err != nil {
			return
		}
		// try go get resource again
		res, ok = c.get(path)
	}
	if ok {
		if res != nil {
			return
		} else {
			err = fmt.Errorf("missing value for key: %s", path)
			return
		}
	} else {
		err = resourcehandlers.ErrResourceNotFound(rl.String())
		return
	}
}

// GetSubset returns a subset of the ResourceLocator objects mapped to keys with this pathPrefix
func (c *Cache) GetSubset(pathPrefix string) ([]*ResourceLocator, error) {
	var (
		keyPrefix string
		err       error
		rl        *ResourceLocator
	)
	if rl, err = Parse(pathPrefix); err != nil {
		return nil, err
	}
	if keyPrefix, err = c.key(rl, false); err != nil {
		return nil, err
	}
	rls := c.getSubset(keyPrefix)
	return rls, nil
}

// GetSubsetWithInit returns a subset of the ResourceLocator objects mapped to keys with this pathPrefix
// Corresponding repository index will be added in the cache, if it has not already been added.
func (c *Cache) GetSubsetWithInit(ctx context.Context, pathPrefix string) ([]*ResourceLocator, error) {
	var (
		keyPrefix string
		ok        bool
		err       error
		rl        *ResourceLocator
	)
	if rl, err = Parse(pathPrefix); err != nil {
		return nil, err
	}
	if keyPrefix, err = c.key(rl, false); err != nil {
		return nil, err
	}
	if rl, ok = c.get(keyPrefix); !ok {
		// init repository
		if _, err = c.set(ctx, rl); err != nil {
			return nil, err
		}
	}
	rls := c.getSubset(keyPrefix)
	return rls, nil
}

// key converts a ResourceLocator to a string that could be used for a cache key
// if repo is 'true' the key for the corresponding resource repository is returned
func (c *Cache) key(rl *ResourceLocator, repo bool) (string, error) {
	host := strings.ToLower(rl.Host)
	if strings.HasPrefix(rl.Host, "raw.") {
		if rl.Host == "raw.githubusercontent.com" {
			host = "github.com"
		} else {
			host = rl.Host[len("raw."):]
		}
	} else if host == "api.github.com" {
		host = "github.com"
	}

	u, err := url.Parse(strings.TrimSuffix(rl.Path, "/"))
	if err != nil {
		return "", err
	}
	path := u.Path
	if repo {
		path = ""
	}
	return strings.ToLower(fmt.Sprintf("%s:%s:%s:%s:%s", host, rl.Owner, rl.Repo, rl.SHAAlias, path)), nil
}

// get a ResourceLocator element from the cache map.
// If the element key exist in the map ok is 'true', otherwise ok is 'false'.
func (c *Cache) get(key string) (rl *ResourceLocator, ok bool) {
	c.mux.RLock()
	defer c.mux.RUnlock()
	rl, ok = c.cache[key]
	return
}

// getSubset of ResourceLocator entries with keyPrefix from the cache map.
func (c *Cache) getSubset(keyPrefix string) []*ResourceLocator {
	c.mux.RLock()
	defer c.mux.RUnlock()
	entries := make([]*ResourceLocator, 0)
	for k, v := range c.cache {
		if k == keyPrefix {
			continue
		}
		if strings.HasPrefix(k, keyPrefix) {
			entries = append(entries, v)
		}
	}
	return entries
}

// set a repository tree index into the cache map.
// Returns 'true' if the repo is added and 'false' if the repo already exist
// If the entry doesn't exist #resourcehandlers.ErrResourceNotFound error is returned.
func (c *Cache) set(ctx context.Context, rl *ResourceLocator) (bool, error) {
	c.mux.Lock()
	defer c.mux.Unlock()
	var repo string
	var err error
	if repo, err = c.key(rl, true); err != nil {
		return false, err
	}
	// first check if repo root exist
	if _, ok := c.cache[repo]; ok {
		return false, nil
	}
	// grab the index of this repo
	gitTree, resp, err := c.ghClient.Git.GetTree(ctx, rl.Owner, rl.Repo, rl.SHAAlias, true)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			// add repo key to avoid further calls to this repo
			c.cache[repo] = nil
			return false, resourcehandlers.ErrResourceNotFound(rl.String())
		}
		return false, err
	}
	if resp.StatusCode >= 400 {
		return false, fmt.Errorf("request for %s://%s/repos/%s/%s/git/trees/%s failed with status: %s", rl.Scheme, rl.Host, rl.Owner, rl.Repo, rl.SHAAlias, resp.Status)
	}
	var eKey string
	for _, entry := range gitTree.Entries {
		if eRL := TreeEntryToGitHubLocator(entry, rl.SHAAlias); eRL != nil {
			if eKey, err = c.key(eRL, false); err != nil {
				return false, err
			}

			c.cache[eKey] = eRL
		}
	}
	// add root repo key if not already added
	if _, ok := c.cache[repo]; !ok {
		c.cache[repo] = nil
	}
	return true, nil
}
