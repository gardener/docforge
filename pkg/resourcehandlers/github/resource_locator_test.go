package github

import (
	"reflect"
	"testing"

	"github.com/gardener/docode/pkg/util/tests"
)

func init() {
	tests.SetGlogV(6)
}

func Test_parse(t *testing.T) {
	tests := []struct {
		name                string
		url                 string
		wantResourceLocator *ResourceLocator
		wantErr             error
	}{
		{
			name: "",
			url:  "https://github.com/gardener/gardener/blob/master/README.md",
			wantResourceLocator: &ResourceLocator{
				Host:     "github.com",
				Owner:    "gardener",
				Repo:     "gardener",
				Path:     "README.md",
				SHAAlias: "master",
				Type:     Blob,
			},
			wantErr: nil,
		},
		{
			name: "",
			url:  "https://github.com/gardener/gardener/tree/master/docs",
			wantResourceLocator: &ResourceLocator{
				Host:     "github.com",
				Owner:    "gardener",
				Repo:     "gardener",
				Path:     "docs",
				SHAAlias: "master",
				Type:     Tree,
			},
			wantErr: nil,
		},
		{
			name: "",
			url:  "https://github.com/gardener/gardener",
			wantResourceLocator: &ResourceLocator{
				Host:     "github.com",
				Owner:    "gardener",
				Repo:     "gardener",
				SHAAlias: "master",
				Type:     Tree,
			},
			wantErr: nil,
		},
		{
			name: "",
			url:  "https://github.com/gardener/gardener/releases/tag/v1.10.0",
			wantResourceLocator: &ResourceLocator{
				Host:  "github.com",
				Owner: "gardener",
				Repo:  "gardener",
				Path:  "releases/tag/v1.10.0",
			},
			wantErr: nil,
		},
		{
			name: "",
			url:  "https://github.com/gardener/gardener/blob/master/README.md#Proposals",
			wantResourceLocator: &ResourceLocator{
				Host:     "github.com",
				Owner:    "gardener",
				Repo:     "gardener",
				Path:     "README.md#Proposals",
				SHAAlias: "master",
				Type:     Blob,
			},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResourceLocator, gotErr := parse(tt.url)
			if gotErr != tt.wantErr {
				t.Errorf("Error %v != %v", gotErr, tt.wantErr)
			}
			if !reflect.DeepEqual(gotResourceLocator, tt.wantResourceLocator) {
				t.Errorf("Expected ResourceLocator %q != %v", tt.wantResourceLocator, gotResourceLocator)
			}
		})
	}
}
