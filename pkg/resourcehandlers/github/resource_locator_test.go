package github

import (
	"fmt"
	"testing"

	"github.com/gardener/docforge/pkg/util/tests"
	"github.com/stretchr/testify/assert"
)

func init() {
	tests.SetKlogV(6)
}

func Test_parse(t *testing.T) {
	tests := []struct {
		name                string
		url                 string
		wantResourceLocator *ResourceLocator
		wantErr             error
	}{
		{
			name: "file url",
			url:  "https://github.com/gardener/gardener/blob/master/README.md",
			wantResourceLocator: &ResourceLocator{
				Scheme:   "https",
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
			name: "folder url",
			url:  "https://github.com/gardener/gardener/tree/master/docs",
			wantResourceLocator: &ResourceLocator{
				Scheme:   "https",
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
			name: "organization",
			url:  "https://github.com/gardener",
			wantResourceLocator: &ResourceLocator{
				Scheme: "https",
				Host:   "github.com",
				Owner:  "gardener",
				Type:   -1,
			},
			wantErr: nil,
		},
		{
			name: "repository root",
			url:  "https://github.com/gardener/gardener",
			wantResourceLocator: &ResourceLocator{
				Scheme: "https",
				Host:   "github.com",
				Owner:  "gardener",
				Repo:   "gardener",
				Type:   -1,
			},
			wantErr: nil,
		},
		{
			name: "releases",
			url:  "https://github.com/gardener/gardener/releases",
			wantResourceLocator: &ResourceLocator{
				Scheme: "https",
				Host:   "github.com",
				Owner:  "gardener",
				Repo:   "gardener",
				Type:   Releases,
				Path:   "",
			},
			wantErr: nil,
		},
		{
			name: "release",
			url:  "https://github.com/gardener/gardener/releases/tag/v1.10.0",
			wantResourceLocator: &ResourceLocator{
				Scheme: "https",
				Host:   "github.com",
				Owner:  "gardener",
				Repo:   "gardener",
				Type:   Releases,
				Path:   "tag/v1.10.0",
			},
			wantErr: nil,
		},
		{
			name: "pulls",
			url:  "https://github.com/gardener/gardener/pulls",
			wantResourceLocator: &ResourceLocator{
				Scheme: "https",
				Host:   "github.com",
				Owner:  "gardener",
				Repo:   "gardener",
				Type:   Pulls,
			},
			wantErr: nil,
		},
		{
			name: "pull",
			url:  "https://github.com/gardener/gardener/pull/123",
			wantResourceLocator: &ResourceLocator{
				Scheme: "https",
				Host:   "github.com",
				Owner:  "gardener",
				Repo:   "gardener",
				Type:   Pull,
				Path:   "123",
			},
			wantErr: nil,
		},
		{
			name: "file with fragment",
			url:  "https://github.com/gardener/gardener/blob/master/README.md#Proposals",
			wantResourceLocator: &ResourceLocator{
				Scheme:   "https",
				Host:     "github.com",
				Owner:    "gardener",
				Repo:     "gardener",
				Path:     "README.md#Proposals",
				SHAAlias: "master",
				Type:     Blob,
			},
			wantErr: nil,
		},
		{
			name: "file with query string",
			url:  "https://github.com/gardener/gardener/blob/master/README.md?a=b",
			wantResourceLocator: &ResourceLocator{
				Scheme:   "https",
				Host:     "github.com",
				Owner:    "gardener",
				Repo:     "gardener",
				Path:     "README.md?a=b",
				SHAAlias: "master",
				Type:     Blob,
			},
			wantErr: nil,
		},
		{
			name: "file with query string and fragment",
			url:  "https://github.com/gardener/gardener/blob/master/README.md#Proposals?a=b",
			wantResourceLocator: &ResourceLocator{
				Scheme:   "https",
				Host:     "github.com",
				Owner:    "gardener",
				Repo:     "gardener",
				Path:     "README.md#Proposals?a=b",
				SHAAlias: "master",
				Type:     Blob,
			},
			wantErr: nil,
		},
		{
			name: "wiki",
			url:  "https://github.com/gardener/documentation/wiki/Architecture",
			wantResourceLocator: &ResourceLocator{
				Scheme:   "https",
				Host:     "github.com",
				Owner:    "gardener",
				Repo:     "documentation",
				Path:     "Architecture",
				SHAAlias: "",
				Type:     Wiki,
			},
			wantErr: nil,
		},
		{
			name: "raw content",
			url:  "https://github.com/gardener/gardener/raw/master/logo/gardener-large.png",
			wantResourceLocator: &ResourceLocator{
				Scheme:   "https",
				Host:     "github.com",
				Owner:    "gardener",
				Repo:     "gardener",
				Path:     "logo/gardener-large.png",
				SHAAlias: "master",
				Type:     Raw,
			},
			wantErr: nil,
		},
		{
			name: "githubusercontent",
			url:  "https://raw.githubusercontent.com/gardener/gardener/master/logo/gardener-large.png",
			wantResourceLocator: &ResourceLocator{
				Scheme:   "https",
				Host:     "raw.githubusercontent.com",
				Owner:    "gardener",
				Repo:     "gardener",
				Path:     "logo/gardener-large.png",
				SHAAlias: "master",
				Type:     Raw,
			},
			wantErr: nil,
		},
		{
			name:                "unsupported url",
			url:                 "https://github.com",
			wantResourceLocator: nil,
			wantErr:             fmt.Errorf("Unsupported GitHub URL: https://github.com. Need at least host and organization|owner"),
		},
		{
			name:                "unsupported url",
			url:                 "https://github.com/gardener/gardener/abc/master/logo/gardener-large.png",
			wantResourceLocator: nil,
			wantErr:             fmt.Errorf("Unsupported GitHub URL: https://github.com/gardener/gardener/abc/master/logo/gardener-large.png . %s", fmt.Errorf("Unknown resource type string '%s'. Must be one of %v", "abc", []string{"tree", "blob", "raw", "wiki", "releases", "issues", "issue", "pulls", "pull"})),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotResourceLocator, gotErr := parse(tc.url)
			if gotErr != tc.wantErr {
				if tc.wantErr == nil || gotErr == nil {
					t.Errorf("Error %v != %v", gotErr, tc.wantErr)
				} else {
					assert.Equal(t, tc.wantErr.Error(), gotErr.Error())
				}
			}
			assert.Equal(t, tc.wantResourceLocator, gotResourceLocator)
		})
	}
}

func Test_GetRaw(t *testing.T) {
	tests := []struct {
		name    string
		rl      *ResourceLocator
		wantURL string
	}{
		{
			name: "",
			rl: &ResourceLocator{
				Scheme:   "https",
				Host:     "github.com",
				Owner:    "gardener",
				Repo:     "gardener",
				Path:     "README.md",
				SHAAlias: "master",
				Type:     Blob,
			},
			wantURL: "https://github.com/gardener/gardener/raw/master/README.md",
		},
		{
			name: "",
			rl: &ResourceLocator{
				Scheme:   "https",
				Host:     "github.com",
				Owner:    "gardener",
				Repo:     "gardener",
				Path:     "docs",
				SHAAlias: "master",
				Type:     Tree,
			},
			wantURL: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotURL := tc.rl.GetRaw()
			assert.Equal(t, tc.wantURL, gotURL)
		})
	}
}

func Test_String(t *testing.T) {
	tests := []struct {
		name    string
		rl      *ResourceLocator
		wantURL string
	}{
		{
			name: "",
			rl: &ResourceLocator{
				Scheme:   "https",
				Host:     "github.com",
				Owner:    "gardener",
				Repo:     "gardener",
				Path:     "README.md",
				SHAAlias: "master",
				Type:     Blob,
			},
			wantURL: "https://github.com/gardener/gardener/blob/master/README.md",
		},
		{
			name: "",
			rl: &ResourceLocator{
				Scheme:   "https",
				Host:     "github.com",
				Owner:    "gardener",
				Repo:     "gardener",
				Path:     "logo/gardener-large.png",
				SHAAlias: "master",
				Type:     Raw,
			},
			wantURL: "https://github.com/gardener/gardener/raw/master/logo/gardener-large.png",
		},
		{
			name: "",
			rl: &ResourceLocator{
				Scheme:   "https",
				Host:     "raw.githubusercontent.com",
				Owner:    "gardener",
				Repo:     "gardener",
				Path:     "logo/gardener-large.png",
				SHAAlias: "master",
				Type:     Raw,
			},
			wantURL: "https://raw.githubusercontent.com/gardener/gardener/master/logo/gardener-large.png",
		},
		{
			name: "",
			rl: &ResourceLocator{
				Scheme:   "https",
				Host:     "github.com",
				Owner:    "gardener",
				Repo:     "gardener",
				Path:     "docs",
				SHAAlias: "master",
				Type:     Tree,
			},
			wantURL: "https://github.com/gardener/gardener/tree/master/docs",
		},
		{
			name: "",
			rl: &ResourceLocator{
				Scheme: "https",
				Host:   "github.com",
				Owner:  "gardener",
				Repo:   "gardener",
				Type:   -1,
			},
			wantURL: "https://github.com/gardener/gardener",
		},
		{
			name: "",
			rl: &ResourceLocator{
				Scheme: "https",
				Host:   "github.com",
				Owner:  "gardener",
				Type:   -1,
			},
			wantURL: "https://github.com/gardener",
		},
		{
			name: "",
			rl: &ResourceLocator{
				Scheme: "https",
				Host:   "github.com",
				Owner:  "gardener",
				Repo:   "gardener",
				Path:   "Architecture",
				Type:   Wiki,
			},
			wantURL: "https://github.com/gardener/gardener/wiki/Architecture",
		},
		{
			name: "",
			rl: &ResourceLocator{
				Scheme: "https",
				Host:   "github.com",
				Owner:  "gardener",
				Repo:   "gardener",
				Path:   "tag/v1.10.0",
				Type:   Releases,
			},
			wantURL: "https://github.com/gardener/gardener/releases/tag/v1.10.0",
		},
		{
			name: "",
			rl: &ResourceLocator{
				Scheme: "https",
				Host:   "github.com",
				Owner:  "gardener",
				Repo:   "gardener",
				Path:   "",
				Type:   Pulls,
			},
			wantURL: "https://github.com/gardener/gardener/pulls",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotURL := tc.rl.String()
			assert.Equal(t, tc.wantURL, gotURL)
		})
	}
}
