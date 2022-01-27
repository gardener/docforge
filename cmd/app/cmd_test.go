// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"testing"

	"github.com/gardener/docforge/cmd/configuration"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/pointer"
)

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
