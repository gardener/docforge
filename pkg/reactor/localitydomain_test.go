package reactor

import (
	"reflect"
	"testing"

	"github.com/gardener/docode/pkg/api"
)

func TestGitHubLocalityDomain_SetLocalityDomain(t *testing.T) {

	tests := []struct {
		name           string
		localityDomain LocalityDomain
		key            string
		urls           []string
		expected       string
	}{
		{
			name: "Should return the same and already existing locality domain",
			localityDomain: map[string]string{
				"https://github.com/gardener/gardener": "/gardener/gardener/master/docs",
			},
			key:      "https://github.com/gardener/gardener",
			urls:     []string{"/gardener/gardener/tree/master/docs"},
			expected: "/gardener/gardener/master/docs",
		},
		{
			name: "Should return the candidate locality domain as it is higher in the hierarchy",
			localityDomain: map[string]string{
				"https://github.com/gardener/gardener": "/gardener/gardener/master/docs",
			},
			key:      "github.com/gardener/gardener",
			urls:     []string{"/gardener/gardener/tree/master", "/gardener/gardener/tree/master/docs/concepts", "/gardener/gardener/blob/master/docs/concepts/apiserver.md"},
			expected: "/gardener/gardener/master",
		},
		{
			name:     "Should return one level higher because both are on the same level in the hierarchy",
			key:      "github.com/gardener/gardener",
			urls:     []string{"/gardener/gardener/tree/master/examples", "/gardener/gardener/tree/master"},
			expected: "/gardener/gardener/master",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gh := LocalityDomain{}
			for _, url := range tt.urls {
				gh.SetLocalityDomain(tt.key, url)
			}

			if gh[tt.key] != tt.expected {
				t.Errorf("test failed %s != %s", gh[tt.key], tt.expected)
			}
		})
	}
}

func Test_defineLocalityDomains(t *testing.T) {
	tests := []struct {
		name    string
		want    LocalityDomain
		wantErr bool
		mutate  func()
	}{
		{
			name: "Should return the expected locality domain",
			want: LocalityDomain{
				"github.com/org/repo": "/org/repo/master/docs",
			},
			wantErr: false,
			mutate: func() {
				documentation.Root.ContentSelectors = []api.ContentSelector{
					{Source: "https://github.com/org/repo/tree/master/docs/concepts"},
					{Source: "https://github.com/org/repo/tree/master/docs/architecture"},
				}
			},
		},
		{
			name: "Should return the expected locality domain",
			want: LocalityDomain{
				"github.com/org/repo": "/org/repo/master/docs",
			},
			wantErr: false,
			mutate: func() {
				documentation.Root.ContentSelectors = []api.ContentSelector{
					{Source: "https://github.com/org/repo/tree/master/docs"},
					{Source: "https://github.com/org/repo/tree/master/docs/architecture"},
				}
			},
		},
		{
			name: "Should return the expected locality domain",
			want: LocalityDomain{
				"github.com/org/repo": "/org/repo/master",
			},
			wantErr: false,
			mutate: func() {
				documentation.Root.ContentSelectors = []api.ContentSelector{
					{Source: "https://github.com/org/repo/tree/master/docs"},
					{Source: "https://github.com/org/repo/tree/master/example"},
				}
			},
		},
		{
			name: "Should return the expected locality domain",
			want: LocalityDomain{
				"github.com/org/repo":  "/org/repo/master",
				"github.com/org/repo2": "/org/repo2/master/example",
			},
			wantErr: false,
			mutate: func() {
				documentation.Root.ContentSelectors = []api.ContentSelector{
					{Source: "https://github.com/org/repo/tree/master/docs"},
					{Source: "https://github.com/org/repo/tree/master/example"},
				}
				documentation.Root.Nodes = []*api.Node{
					{
						Name:             "anotherrepo",
						ContentSelectors: []api.ContentSelector{{Source: "https://github.com/org/repo2/tree/master/example"}},
					},
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mutate()
			got, err := defineLocalityDomains(documentation.Root)
			if (err != nil) != tt.wantErr {
				t.Errorf("defineLocalityDomains() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("defineLocalityDomains() = %v, want %v", got, tt.want)
			}
		})
	}
}
