package reactor

import (
	"reflect"
	"testing"

	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/resourcehandlers/github"

	"github.com/gardener/docforge/pkg/api"
)

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
		mutate  func(newDoc *api.Documentation)
	}{
		{
			name: "Should return the expected locality domain",
			want: localityDomain{
				"github.com/org/repo": &localityDomainValue{
					"master",
					"org/repo/docs",
				},
			},
			wantErr: false,
			mutate: func(newDoc *api.Documentation) {
				newDoc.Root.ContentSelectors = []api.ContentSelector{
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
					"org/repo/docs",
				},
			},
			wantErr: false,
			mutate: func(newDoc *api.Documentation) {
				newDoc.Root.ContentSelectors = []api.ContentSelector{
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
					"org/repo",
				},
			},
			wantErr: false,
			mutate: func(newDoc *api.Documentation) {
				newDoc.Root.ContentSelectors = []api.ContentSelector{
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
					"org/repo",
				},
				"github.com/org/repo2": &localityDomainValue{
					"master",
					"org/repo2/example",
				},
			},
			wantErr: false,
			mutate: func(newDoc *api.Documentation) {
				newDoc.Root.ContentSelectors = []api.ContentSelector{
					{Source: "https://github.com/org/repo/tree/master/docs"},
					{Source: "https://github.com/org/repo/tree/master/example"},
				}
				newDoc.Root.Nodes = []*api.Node{
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
			newDoc := createNewDocumentation()
			gh := github.NewResourceHandler(nil, []string{"github.com"})
			rhs := resourcehandlers.NewRegistry(gh)
			tt.mutate(newDoc)
			got, err := setLocalityDomainForNode(newDoc.Root, rhs)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetLocalityDomainForNode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			for k, v := range tt.want {
				var (
					_v *localityDomainValue
					ok bool
				)
				if _v, ok = got[k]; !ok {
					t.Errorf("want %s:%v, got %s:%v", k, v, k, _v)
				} else {
					if _v.Path != v.Path {
						t.Errorf("want path %s, got %s", v.Path, _v.Path)
					}
					if _v.Version != v.Version {
						t.Errorf("want version %s, got %s", v.Version, _v.Version)
					}
				}
			}
			rhs.Remove()
		})
	}
}
