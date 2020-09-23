package reactor

import (
	"reflect"
	"testing"

	"github.com/gardener/docode/pkg/api"
	"github.com/gardener/docode/pkg/resourcehandlers"
	ghrs "github.com/gardener/docode/pkg/resourcehandlers/github"
)

func init() {
	gh := ghrs.NewResourceHandler(nil)
	resourcehandlers.Load(gh)
}

func TestGitHubLocalityDomain_Set(t *testing.T) {

	tests := []struct {
		name           string
		localityDomain localityDomain
		key            string
		urls           []string
		expected       *localityDomainValue
	}{
		{
			name: "Should return the same and already existing locality domain",
			localityDomain: localityDomain{
				"https://github.com/gardener/gardener": &localityDomainValue{
					"master",
					"/gardener/gardener/master/docs",
				},
			},
			key:  "https://github.com/gardener/gardener",
			urls: []string{"/gardener/gardener/master/docs"},
			expected: &localityDomainValue{
				"master",
				"/gardener/gardener/master/docs",
			},
		},
		{
			name: "Should return the candidate locality domain as it is higher in the hierarchy",
			localityDomain: localityDomain{
				"https://github.com/gardener/gardener": &localityDomainValue{
					"master",
					"/gardener/gardener/master/docs",
				},
			},
			key:  "github.com/gardener/gardener",
			urls: []string{"/gardener/gardener/master", "/gardener/gardener/master/docs/concepts", "/gardener/gardener/master/docs/concepts/apiserver.md"},
			expected: &localityDomainValue{
				"master",
				"/gardener/gardener/master",
			},
		},
		{
			name:           "Should return one level higher because both are on the same level in the hierarchy",
			localityDomain: localityDomain{},
			key:            "github.com/gardener/gardener",
			urls:           []string{"/gardener/gardener/master/examples", "/gardener/gardener/master"},
			expected: &localityDomainValue{
				"master",
				"/gardener/gardener/master",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ld := tt.localityDomain
			for _, url := range tt.urls {
				ld.Set(tt.key, url, "master")
			}

			if !reflect.DeepEqual(ld[tt.key], tt.expected) {
				t.Errorf("test failed %s != %s", ld[tt.key], tt.expected)
			}
		})
	}
}

func Test_SetLocalityDomainForNode(t *testing.T) {
	tests := []struct {
		name    string
		want    localityDomain
		wantErr bool
		mutate  func()
	}{
		{
			name: "Should return the expected locality domain",
			want: localityDomain{
				"github.com/org/repo": &localityDomainValue{
					"master",
					"org/repo/master/docs",
				},
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
			want: localityDomain{
				"github.com/org/repo": &localityDomainValue{
					"master",
					"org/repo/master/docs",
				},
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
			want: localityDomain{
				"github.com/org/repo": &localityDomainValue{
					"master",
					"org/repo/master",
				},
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
			want: localityDomain{
				"github.com/org/repo": &localityDomainValue{
					"master",
					"org/repo/master",
				},
				"github.com/org/repo2": &localityDomainValue{
					"master",
					"org/repo2/master/example",
				},
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
			got, err := setLocalityDomainForNode(documentation.Root)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetLocalityDomainForNode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SetLocalityDomainForNode() = %v, want %v", got, tt.want)
			}
		})
	}
}
