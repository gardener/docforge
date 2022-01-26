// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package configuration

// Config defines docforge configuration
type Config struct {
	// CacheHome defines git repositories cache location
	CacheHome   *string        `yaml:"cacheHome,omitempty"`
	Credentials []*Credentials `yaml:"credentials,omitempty"`
	// ResourceMappings defines URL -> location mapping for existing git repositories
	ResourceMappings map[string]string `yaml:"resourceMappings,omitempty"`
	Hugo             *Hugo             `yaml:"hugo,omitempty"`
	DefaultBranches  map[string]string `yaml:"defaultBranches,omitempty"`
	LastNVersions    map[string]int    `yaml:"lastNVersions,omitempty"`
}

// Credentials holds repositories access credentials
type Credentials struct {
	Host       string  `yaml:"host"`
	Username   *string `yaml:"username,omitempty"`
	OAuthToken *string `yaml:"oauthToken,omitempty"`
}

// Hugo is the configuration options for creating HUGO implementations
// docforge interfaces
type Hugo struct {
	Enabled bool `yaml:"enabled"`
	// PrettyUrls indicates if links rewritten for Hugo will be
	// formatted for pretty url support or not. Pretty urls in Hugo
	// place built source content in index.html, which resides in a path segment with
	// the name of the file, making request URLs more resource-oriented.
	// Example: (source) sample.md -> (build) sample/index.html -> (runtime) ./sample
	PrettyURLs bool `yaml:"prettyURLs,omitempty"`
	// BaseURL is used from the Hugo processor to rewrite relative links to root-relative
	BaseURL string `yaml:"baseURL,omitempty"`
	// IndexFileNames defines a list of file names that indicate
	// their content can be used as Hugo section files (_index.md).
	IndexFileNames []string `yaml:"sectionFiles,omitempty"`
}
