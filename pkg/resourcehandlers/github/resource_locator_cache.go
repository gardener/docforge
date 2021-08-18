// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package github

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
)

// Cache is indexes GitHub TreeEntries by website resource URLs as keys,
// mapping ResourceLocator objects to them.
// TODO: implement me efficiently and for parallel use
type Cache struct {
	cache map[string]*ResourceLocator
	mux   sync.RWMutex
}

// Get returns a ResourceLocator object mapped to the path (URL)
func (c *Cache) Get(entry *ResourceLocator) (*ResourceLocator, error) {
	defer c.mux.Unlock()
	c.mux.Lock()
	path, err := c.Key(entry)
	if err != nil {
		return nil, err
	}
	return c.cache[path], nil
}

// Key converts a ResourceLocator to a string that could be used for a cache key
func (c *Cache) Key(rl *ResourceLocator) (string, error) {
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

	return strings.ToLower(fmt.Sprintf("%s:%s:%s:%s:%s", host, rl.Owner, rl.Repo, rl.SHAAlias, u.Path)), nil
}

// GetSubset returns a subset of the ResourceLocator objects mapped to keys
// with this pathPrefix
func (c *Cache) GetSubset(pathPrefix string) ([]*ResourceLocator, error) {
	defer c.mux.Unlock()
	c.mux.Lock()
	rl, _ := Parse(pathPrefix)

	var entries = make([]*ResourceLocator, 0)
	for k, v := range c.cache {
		key, err := c.Key(rl)
		if err != nil {
			return nil, err
		}
		if k == key {
			continue
		}
		if strings.HasPrefix(k, key) {
			entries = append(entries, v)
		}
	}
	return entries, nil
}

// Set adds a mapping between a path (URL) and a ResourceLocator to the cache
func (c *Cache) Set(entry *ResourceLocator) (*ResourceLocator, error) {
	defer c.mux.Unlock()
	c.mux.Lock()
	path, err := c.Key(entry)
	if err != nil {
		return nil, err
	}
	c.cache[path] = entry
	return entry, nil
}
