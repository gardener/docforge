// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"reflect"
	"testing"

	"github.com/gardener/docforge/cmd/configuration"
	"github.com/gardener/docforge/pkg/hugo"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/pointer"
)

func Test_hugoOptions(t *testing.T) {
	type args struct {
		f      *cmdFlags
		config *configuration.Config
	}
	tests := []struct {
		name string
		args args
		want *hugo.Options
	}{
		{
			name: "return_nil_when_no_config_or_flag_provided",
			args: args{
				f: &cmdFlags{
					hugo: false,
				},
				config: &configuration.Config{
					Hugo: nil,
				},
			},
			want: nil,
		},
		{
			name: "return_default_hugo_options",
			args: args{
				f: &cmdFlags{
					hugo:           true,
					hugoPrettyUrls: true,
				},
				config: &configuration.Config{
					Hugo: nil,
				},
			},
			want: &hugo.Options{
				PrettyUrls:     true,
				IndexFileNames: []string{},
				BaseURL:        "",
			},
		},
		{
			name: "use_base_url_from_config_when_not_specified_in_flags",
			args: args{
				f: &cmdFlags{
					hugo:           true,
					hugoPrettyUrls: true,
				},
				config: &configuration.Config{
					Hugo: &configuration.Hugo{
						BaseURL: pointer.StringPtr("/new/baseURL"),
					},
				},
			},
			want: &hugo.Options{
				PrettyUrls:     true,
				IndexFileNames: []string{},
				BaseURL:        "/new/baseURL",
			},
		},
		{
			name: "use_base_url_from_flags_with_priority",
			args: args{
				f: &cmdFlags{
					hugo:           true,
					hugoPrettyUrls: true,
					hugoBaseURL:    "/override",
				},
				config: &configuration.Config{
					Hugo: &configuration.Hugo{
						BaseURL: pointer.StringPtr("/new/baseURL"),
					},
				},
			},
			want: &hugo.Options{
				PrettyUrls:     true,
				IndexFileNames: []string{},
				BaseURL:        "/override",
			},
		},
		{
			name: "set_hugo_base_url_from_flags",
			args: args{
				f: &cmdFlags{
					hugo:           true,
					hugoPrettyUrls: true,
					hugoBaseURL:    "/fromFlag",
				},
				config: &configuration.Config{
					Hugo: &configuration.Hugo{
						BaseURL: nil,
					},
				},
			},
			want: &hugo.Options{
				PrettyUrls:     true,
				IndexFileNames: []string{},
				BaseURL:        "/fromFlag",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hugoOptions(tt.args.f, tt.args.config); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("hugoOptions() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_prettyURLs(t *testing.T) {
	type args struct {
		fromFlag   bool
		fromConfig *bool
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "returns_false_flag_default_value_and_configuration_set_to_false",
			args: args{
				fromFlag:   true,
				fromConfig: pointer.BoolPtr(false),
			},
			want: false,
		},
		{
			name: "returns_false_flag_set_to_false_and_configuration_set_to_false",
			args: args{
				fromFlag:   false,
				fromConfig: pointer.BoolPtr(false),
			},
			want: false,
		},
		{
			name: "returns_false_flag_set_to_false_and_configuration_set_to_true",
			args: args{
				fromFlag:   false,
				fromConfig: pointer.BoolPtr(true),
			},
			want: false,
		},
		{
			name: "returns_true_when_both_true",
			args: args{
				fromFlag:   true,
				fromConfig: pointer.BoolPtr(true),
			},
			want: true,
		},
		{
			name: "returns_true_flag_set_to_default_or_true_and_configuration_is_not_set",
			args: args{
				fromFlag:   true,
				fromConfig: nil,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := prettyURLs(tt.args.fromFlag, &configuration.Hugo{PrettyURLs: tt.args.fromConfig}); tt.want != got {
				t.Errorf("failed test: %s, wants %t got %t", tt.name, tt.want, got)
			}
		})
	}
}

func Test_combineSectionFiles(t *testing.T) {
	type args struct {
		sectionFilesFromFlags  []string
		sectionFilesFromConfig []string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "successfully_merges",
			args: args{
				sectionFilesFromFlags: []string{
					"index.md",
					"readme.md",
				},
				sectionFilesFromConfig: []string{
					"readme.md",
					"readme",
				},
			},
			want: []string{
				"readme.md",
				"readme",
				"index.md",
			},
		},
		{
			name: "successfully_adds_all_from_flags",
			args: args{
				sectionFilesFromFlags: []string{
					"index.md",
					"readme.md",
				},
				sectionFilesFromConfig: []string{},
			},
			want: []string{
				"readme.md",
				"index.md",
			},
		},
		{
			name: "successfully_adds_all_from_config",
			args: args{
				sectionFilesFromFlags: []string{},
				sectionFilesFromConfig: []string{
					"index.md",
					"readme.md"},
			},
			want: []string{
				"readme.md",
				"index.md",
			},
		},
		{
			name: "empty_input_returns_empty_output",
			args: args{
				sectionFilesFromFlags:  []string{},
				sectionFilesFromConfig: []string{},
			},
			want: []string{},
		},
		{
			name: "nil_slices_return_new_empty_slice",
			args: args{
				sectionFilesFromFlags:  nil,
				sectionFilesFromConfig: nil,
			},
			want: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := combineSectionFiles(tt.args.sectionFilesFromFlags, &configuration.Hugo{SectionFiles: tt.args.sectionFilesFromConfig})
			assert.ElementsMatch(t, got, tt.want)
		})
	}
}
