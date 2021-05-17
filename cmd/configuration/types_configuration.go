// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package configuration

type Config struct {
	CacheHome *string   `yaml:"cacheHome,omitempty"`
	Sources   []*Source `yaml:"sources,omitempty"`
}

type Source struct {
	Host        string `yaml:"host"`
	Credentials `yaml:"credentials,omitempty"`
}

type Credentials struct {
	Username   *string `yaml:"username,omitempty"`
	OAuthToken *string `yaml:"oauthToken,omitempty"`
}
