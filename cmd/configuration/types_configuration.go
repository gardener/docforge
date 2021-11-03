// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package configuration

// Config defines docforge configuration
type Config struct {
	// CacheHome defines git repositories cache location
	CacheHome *string   `yaml:"cacheHome,omitempty"`
	Sources   []*Source `yaml:"sources,omitempty"`
	// ResourceMappings defines URL -> location mapping for existing git repositories
	ResourceMappings map[string]string `yaml:"resourceMappings,omitempty"`
	Hugo             *Hugo             `yaml:"hugo,omitempty"`
	DefaultBranches  map[string]string `yaml:"defaultBranches,omitempty"`
	LastNVersions    map[string]int    `yaml:"lastNVersions,omitempty"`
}

// Source holds repositories access credentials
type Source struct {
	Host        string `yaml:"host"`
	Credentials `yaml:"credentials,omitempty"`
}

// Credentials holds Username and OAuthToken
type Credentials struct {
	Username   *string `yaml:"username,omitempty"`
	OAuthToken *string `yaml:"oauthToken,omitempty"`
}

// Hugo defines HUGO specific configuration
type Hugo struct {
	Enabled      bool     `yaml:"enabled"`
	PrettyURLs   *bool    `yaml:"prettyURLs,omitempty"`
	BaseURL      *string  `yaml:"baseURL,omitempty"`
	SectionFiles []string `yaml:"sectionFiles"`
}
