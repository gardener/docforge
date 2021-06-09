// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package configuration

import (
	"reflect"
	"testing"

	"k8s.io/utils/pointer"
)

func Test_load(t *testing.T) {
	tests := []struct {
		name           string
		configFilePath string
		want           *Config
		wantErr        bool
	}{
		{
			name:           "test_only_sources",
			configFilePath: "testdata/config_test.yaml",
			want: &Config{
				Sources: []*Source{
					{
						Host: "github.com",
						Credentials: Credentials{
							Username:   pointer.StringPtr("Bob"),
							OAuthToken: pointer.StringPtr("s0m3tok3n"),
						},
					}}},
			wantErr: false,
		},
		{
			name:           "",
			configFilePath: "",
			want:           &Config{},
			wantErr:        false,
		},
		{
			name:           "config_full_name",
			configFilePath: "testdata/config_full.yaml",
			want: &Config{
				CacheHome: pointer.StringPtr("~/.docforge/cache_old"),
				Sources: []*Source{
					{
						Host: "github.com",
						Credentials: Credentials{
							Username:   pointer.StringPtr("Bob"),
							OAuthToken: pointer.StringPtr("s0m3tok3n"),
						},
					},
				},
				ResourceMappings: map[string]string{
					"https://github.com/gardener/gardener/tree/master/docs": "/usr/user/home/git/github.com/gardener/gardener/docs",
				},
				Hugo: &Hugo{
					Enabled:    false,
					PrettyURLs: pointer.BoolPtr(true),
					BaseURL:    pointer.StringPtr("/gardener"),
					SectionFiles: []string{
						"indexmd",
					},
				},
			},
			wantErr: false,
		},
		{
			name:           "missing_config_file_name",
			configFilePath: "testdata/missing_file.yaml",
			want:           &Config{},
			wantErr:        false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := load(tt.configFilePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("load() = %v, want %v", got, tt.want)
			}
		})
	}
}
