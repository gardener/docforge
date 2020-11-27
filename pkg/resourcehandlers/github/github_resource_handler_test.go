// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package github

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/util/tests"
	"github.com/google/go-github/v32/github"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/pointer"
)

const (
	// baseURLPath is a non-empty Client.BaseURL path to use during tests,
	// to ensure relative URLs are used for all endpoints. See issue #752.
	baseURLPath = "/api-v3"
)

func init() {
	tests.SetKlogV(6)
}

// setup sets up a test HTTP server along with a github.Client that is
// configured to talk to that test server. Tests should register handlers on
// mux which provide mock responses for the API method being tested.
func setup() (client *github.Client, mux *http.ServeMux, serverURL string, teardown func()) {
	// mux is the HTTP request multiplexer used with the test server.
	mux = http.NewServeMux()

	// We want to ensure that tests catch mistakes where the endpoint URL is
	// specified as absolute rather than relative. It only makes a difference
	// when there's a non-empty base URL path. So, use that. See issue #752.
	apiHandler := http.NewServeMux()
	apiHandler.Handle(baseURLPath+"/", http.StripPrefix(baseURLPath, mux))
	apiHandler.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		fmt.Fprintln(os.Stderr, "FAIL: Client.BaseURL path prefix is not preserved in the request URL:")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "\t"+req.URL.String())
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "\tDid you accidentally use an absolute endpoint URL rather than relative?")
		fmt.Fprintln(os.Stderr, "\tSee https://github.com/google/go-github/issues/752 for information.")
		http.Error(w, "Client.BaseURL path prefix is not preserved in the request URL.", http.StatusInternalServerError)
	})

	// server is a test HTTP server used to provide mock API responses.
	server := httptest.NewServer(apiHandler)

	// client is the GitHub client being tested and is
	// configured to use test server.
	client = github.NewClient(nil)
	url, _ := url.Parse(server.URL + baseURLPath + "/")
	client.BaseURL = url
	client.UploadURL = url

	return client, mux, server.URL, server.Close
}

func TestUrlToGitHubLocator(t *testing.T) {
	ghrl1 := &ResourceLocator{
		"https",
		"github.com",
		"gardener",
		"gardener",
		"",
		Blob,
		"docs/README.md",
		"master",
		false,
	}
	ghrl2 := &ResourceLocator{
		"https",
		"github.com",
		"gardener",
		"gardener",
		"91776959202ec10db883c5cfc05c51e78403f02c",
		Blob,
		"docs/README.md",
		"master",
		false,
	}
	ghrl3 := &ResourceLocator{
		"https",
		"github.com",
		"gardener",
		"gardener",
		"",
		Pull,
		"123",
		"",
		false,
	}
	cases := []struct {
		description  string
		inURL        string
		inResolveAPI bool
		cache        *Cache
		mux          func(mux *http.ServeMux)
		want         *ResourceLocator
	}{
		{
			"cached url should return valid GitHubResourceLocator",
			"https://github.com/gardener/gardener/blob/master/docs/README.md",
			false,
			&Cache{
				cache: map[string]*ResourceLocator{
					"https://github.com/gardener/gardener/blob/master/docs/README.md": ghrl1,
				},
			},
			nil,
			ghrl1,
		},
		{
			"non-cached url should resolve a valid GitHubResourceLocator from API",
			"https://github.com/gardener/gardener/blob/master/docs/README.md",
			true,
			&Cache{
				cache: map[string]*ResourceLocator{},
			},
			func(mux *http.ServeMux) {
				mux.HandleFunc("/repos/gardener/gardener/git/trees/master", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte(fmt.Sprintf(`
						{
							"sha": "0255b12f5b51f821e59cf5cf343cb0c36f1cb1f9",
							"url": "http://api.github.com/repos/gardener/gardener/git/trees/0255b12f5b51f821e59cf5cf343cb0c36f1cb1f9",
							"tree": [
								{
									"path": "docs/README.md",
									"mode": "100644",
									"type": "blob",
									"sha": "91776959202ec10db883c5cfc05c51e78403f02c",
									"size": 6260,
									"url": "https://api.github.com/repos/gardener/gardener/git/blobs/91776959202ec10db883c5cfc05c51e78403f02c"
								}
							]
						}`)))
				})
			},
			ghrl2,
		},
		{
			"cached non-SHAAlias url should return valid GitHubResourceLocator",
			"https://github.com/gardener/gardener/pull/123",
			false,
			&Cache{
				cache: map[string]*ResourceLocator{
					"https://github.com/gardener/gardener/pull/123": ghrl3,
				},
			},
			nil,
			ghrl3,
		},
	}
	for _, c := range cases {
		t.Run("", func(t *testing.T) {
			fmt.Println(c.description)
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			gh := &GitHub{
				cache: c.cache,
			}
			if c.inResolveAPI {
				client, mux, _, teardown := setup()
				defer teardown()
				if c.mux != nil {
					c.mux(mux)
				}
				gh.Client = client
			}
			got, err := gh.URLToGitHubLocator(ctx, c.inURL, c.inResolveAPI)
			if err != nil {
				t.Errorf("Test failed %s", err.Error())
			}
			assert.Equal(t, c.want, got)
		})
	}
}

func TestResolveNodeSelector(t *testing.T) {
	n1 := &api.Node{
		NodeSelector: &api.NodeSelector{
			Path: "https://github.com/gardener/gardener/tree/master/docs",
		},
	}
	cases := []struct {
		description        string
		inNode             *api.Node
		excludePaths       []string
		frontMatter        map[string]interface{}
		excludeFrontMatter map[string]interface{}
		depth              int32
		mux                func(mux *http.ServeMux)
		want               *api.Node
		wantError          error
	}{
		{
			"resolve node selector",
			n1,
			nil,
			nil,
			nil,
			0,
			func(mux *http.ServeMux) {
				mux.HandleFunc("/repos/gardener/gardener/git/trees/master", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte(fmt.Sprintf(`
						{
							"sha": "0255b12f5b51f821e59cf5cf343cb0c36f1cb1f9",
							"url": "http://api.github.com/repos/gardener/gardener/git/trees/0255b12f5b51f821e59cf5cf343cb0c36f1cb1f9",
							"tree": [
								{
									"path": "docs",
									"mode": "040000",
									"type": "tree",
									"sha": "5e11bda664b234920d85db5ca10055916c11e35d",
									"url": "https://api.github.com/repos/gardener/gardener/git/trees/5e11bda664b234920d85db5ca10055916c11e35d"
								},
								{
									"path": "docs/README.md",
									"mode": "100644",
									"type": "blob",
									"sha": "91776959202ec10db883c5cfc05c51e78403f02c",
									"size": 6260,
									"url": "https://api.github.com/repos/gardener/gardener/git/blobs/91776959202ec10db883c5cfc05c51e78403f02c"
								},
								{
									"path": "docs/concepts",
									"mode": "040000",
									"type": "tree",
									"sha": "e3ac8f22d00ab4423b184687d3ecc7e03e7393eb",
									"url": "https://api.github.com/repos/gardener/gardener/git/trees/e3ac8f22d00ab4423b184687d3ecc7e03e7393eb"
								},
								{
									"path": "docs/concepts/apiserver.md",
									"mode": "100644",
									"type": "blob",
									"sha": "30c4e21a53be25f9300f9cca8bd73309b1257d1f",
									"size": 5209,
									"url": "https://api.github.com/repos/gardener/gardener/git/blobs/30c4e21a53be25f9300f9cca8bd73309b1257d1f"
								}
							]
						}`)))
				})
			},
			&api.Node{
				NodeSelector: &api.NodeSelector{
					Path: "https://github.com/gardener/gardener/tree/master/docs",
				},
				Nodes: []*api.Node{
					{
						Name:   "README.md",
						Source: "https://github.com/gardener/gardener/blob/master/docs/README.md",
					},
					{
						Name: "concepts",
						Nodes: []*api.Node{
							{
								Name:   "apiserver.md",
								Source: "https://github.com/gardener/gardener/blob/master/docs/concepts/apiserver.md",
							},
						},
					},
				},
			},
			nil,
		},
	}
	for _, c := range cases {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		gh := &GitHub{
			cache: &Cache{
				cache: map[string]*ResourceLocator{},
			},
		}
		client, mux, _, teardown := setup()
		defer teardown()
		if c.mux != nil {
			c.mux(mux)
		}
		gh.Client = client

		nodes, gotError := gh.ResolveNodeSelector(ctx, c.inNode, c.excludePaths, c.frontMatter, c.excludeFrontMatter, c.depth)
		if gotError != nil {
			t.Errorf("error == %q, want %q", gotError, c.wantError)
		}
		c.inNode.Nodes = append(c.inNode.Nodes, nodes...)
		c.want.SetParentsDownwards()
		api.SortNodesByName(c.inNode)
		api.SortNodesByName(c.want)
		assert.Equal(t, c.want, c.inNode)
	}
}

func TestName(t *testing.T) {
	ghrl1 := &ResourceLocator{
		"https",
		"github.com",
		"gardener",
		"gardener",
		"master",
		Blob,
		"docs/README.md",
		"",
		false,
	}
	ghrl2 := &ResourceLocator{
		"https",
		"github.com",
		"gardener",
		"gardener",
		"master",
		Tree,
		"docs",
		"",
		false,
	}
	testCases := []struct {
		description string
		inURL       string
		cache       *Cache
		wantName    string
		wantExt     string
	}{
		{
			"return file name for url",
			"https://github.com/gardener/gardener/blob/master/docs/README.md",
			&Cache{
				cache: map[string]*ResourceLocator{
					"https://github.com/gardener/gardener/blob/master/docs/README.md": ghrl1,
				},
			},
			"README",
			"md",
		},
		{
			"return folder name for url",
			"https://github.com/gardener/gardener/tree/master/docs",
			&Cache{
				cache: map[string]*ResourceLocator{
					"https://github.com/gardener/gardener/tree/master/docs": ghrl2,
				},
			},
			"docs",
			"",
		},
	}
	for _, tc := range testCases {
		gh := &GitHub{
			cache: tc.cache,
		}
		gotName, gotExt := gh.ResourceName(tc.inURL)
		assert.Equal(t, tc.wantName, gotName)
		assert.Equal(t, tc.wantExt, gotExt)
	}
}

func TestRead(t *testing.T) {
	var sampleContent = []byte("Sample content")
	cases := []struct {
		description string
		inURI       string
		mux         func(mux *http.ServeMux)
		cache       *Cache
		want        []byte
		wantError   error
	}{
		{
			"read node source",
			"https://github.com/gardener/gardener/blob/master/docs/README.md",
			func(mux *http.ServeMux) {
				mux.HandleFunc("/repos/gardener/gardener/git/blobs/master", func(w http.ResponseWriter, r *http.Request) {
					w.Write(sampleContent)
				})
			},
			&Cache{
				cache: map[string]*ResourceLocator{
					"https://github.com/gardener/gardener/blob/master/docs/README.md": {
						"https",
						"github.com",
						"gardener",
						"gardener",
						"master",
						Blob,
						"docs/README.md",
						"",
						false,
					},
				},
			},
			sampleContent,
			nil,
		},
	}
	for _, c := range cases {
		fmt.Println(c.description)
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		client, mux, serverURL, teardown := setup()
		defer teardown()
		// rewrite cached url keys host to match the mock server
		for k, v := range c.cache.cache {
			c.cache.cache[strings.Replace(k, "https://github.com", serverURL, 1)] = v
		}
		gh := &GitHub{
			cache: c.cache,
		}
		if c.mux != nil {
			c.mux(mux)
		}
		gh.Client = client
		inURI := strings.Replace(c.inURI, "https://github.com", serverURL, 1)
		got, gotError := gh.Read(ctx, c.inURI)
		if gotError != nil {
			t.Errorf("error == %q, want %q", gotError, c.wantError)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("Read(ctx,%v) == %v, want %v", inURI, string(got), string(c.want))
		}
	}
}

func TestGitHub_ResolveRelLink(t *testing.T) {

	type args struct {
		source string
		link   string
	}
	tests := []struct {
		name        string
		args        args
		wantRelLink string
	}{
		{
			name: "test nested relative link",
			args: args{
				source: "https://github.com/gardener/gardener/master/tree/readme.md",
				link:   "jjbj.md",
			},
			wantRelLink: "https://github.com/gardener/gardener/master/tree/jjbj.md",
		},
		{
			name: "test outside link",
			args: args{
				source: "https://github.com/gardener/gardener/master/tree/docs/extensions/readme.md",
				link:   "../../images/jjbj.png",
			},
			wantRelLink: "https://github.com/gardener/gardener/master/tree/images/jjbj.png",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gh := &GitHub{}
			if gotRelLink, _ := gh.BuildAbsLink(tt.args.source, tt.args.link); gotRelLink != tt.wantRelLink {
				t.Errorf("GitHub.ResolveRelLink() = %v, want %v", gotRelLink, tt.wantRelLink)
			}
		})
	}
}

func TestCleanupNodeTree(t *testing.T) {
	tests := []struct {
		name     string
		node     *api.Node
		wantNode *api.Node
	}{
		{
			name: "",
			node: &api.Node{
				Name:   "00",
				Source: "https://github.com/gardener/gardener/tree/master/docs/00",
				Nodes: []*api.Node{
					{
						Name:   "01.md",
						Source: "https://github.com/gardener/gardener/blob/master/docs/01.md",
					},
					{
						Name:   "02",
						Source: "https://github.com/gardener/gardener/tree/master/docs/02",
						Nodes: []*api.Node{
							{
								Name:   "021.md",
								Source: "https://github.com/gardener/gardener/blob/master/docs/021.md",
							},
						},
					},
					{
						Name:   "03",
						Source: "https://github.com/gardener/gardener/tree/master/docs/03",
						Nodes:  []*api.Node{},
					},
				},
			},
			wantNode: &api.Node{
				Name: "00",
				Nodes: []*api.Node{
					{
						Name:   "01.md",
						Source: "https://github.com/gardener/gardener/blob/master/docs/01.md",
					},
					{
						Name: "02",
						Nodes: []*api.Node{
							{
								Name:   "021.md",
								Source: "https://github.com/gardener/gardener/blob/master/docs/021.md",
							},
						},
					},
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cleanupNodeTree(tc.node)
			assert.Equal(t, tc.wantNode, tc.node)
		})
	}
}

func TestTreeEntryToGitHubLocator(t *testing.T) {
	type args struct {
		treeEntry *github.TreeEntry
		shaAlias  string
	}
	tests := []struct {
		name       string
		args       args
		expectedRL *ResourceLocator
	}{
		{
			name: "should return the expected ResourceLocator for a enterprise GitHub entry",
			args: args{
				treeEntry: &github.TreeEntry{
					SHA:  pointer.StringPtr("b578f8f6cce210d44388e7136b9acce055da4d1b"),
					Path: pointer.StringPtr("docs/cluster_resources.md"),
					Mode: pointer.StringPtr("100644"),
					Type: pointer.StringPtr("blob"),
					Size: new(int),
					URL:  pointer.StringPtr("https://github.enterprise/api/v3/repos/test-org/test-repo/git/blobs/b578f8f6cce210d44388e7136b9acce055da4d1b"),
				},
				shaAlias: "master",
			},
			expectedRL: &ResourceLocator{
				Host:     "github.enterprise",
				Owner:    "test-org",
				Repo:     "test-repo",
				Path:     "docs/cluster_resources.md",
				SHA:      "b578f8f6cce210d44388e7136b9acce055da4d1b",
				Scheme:   "https",
				SHAAlias: "master",
				Type:     Blob,
			},
		},
		{
			name: "should return the expected ResourceLocator for a enterprise GitHub raw entry",
			args: args{
				treeEntry: &github.TreeEntry{
					SHA:  pointer.StringPtr("b578f8f6cce210d44388e7136b9acce055da4d1b"),
					Path: pointer.StringPtr("docs/cluster_resources.md"),
					Mode: pointer.StringPtr("100644"),
					Type: pointer.StringPtr("blob"),
					Size: new(int),
					URL:  pointer.StringPtr("https://github.enterprise/api/v3/repos/test-org/test-repo/git/blobs/b578f8f6cce210d44388e7136b9acce055da4d1b"),
				},
				shaAlias: "master",
			},
			expectedRL: &ResourceLocator{
				Host:     "github.enterprise",
				Owner:    "test-org",
				Repo:     "test-repo",
				Path:     "docs/cluster_resources.md",
				SHA:      "b578f8f6cce210d44388e7136b9acce055da4d1b",
				Scheme:   "https",
				SHAAlias: "master",
				Type:     Blob,
			},
		},
		{
			name: "should return the expected ResourceLocator for a GitHub raw entry",
			args: args{
				treeEntry: &github.TreeEntry{
					SHA:  pointer.StringPtr("b578f8f6cce210d44388e7136b9acce055da4d1b"),
					Path: pointer.StringPtr("docs/cluster_resources.md"),
					Mode: pointer.StringPtr("100644"),
					Type: pointer.StringPtr("blob"),
					Size: new(int),
					URL:  pointer.StringPtr("https://api.github.com/repos/test-org/test-repo/git/blobs/b578f8f6cce210d44388e7136b9acce055da4d1b"),
				},
				shaAlias: "master",
			},
			expectedRL: &ResourceLocator{
				Host:     "api.github.com",
				Owner:    "test-org",
				Repo:     "test-repo",
				Path:     "docs/cluster_resources.md",
				SHA:      "b578f8f6cce210d44388e7136b9acce055da4d1b",
				Scheme:   "https",
				SHAAlias: "master",
				Type:     Blob,
			},
		},
		{
			name: "should return the expected ResourceLocator for a GitHub entry",
			args: args{
				treeEntry: &github.TreeEntry{
					SHA:  pointer.StringPtr("b578f8f6cce210d44388e7136b9acce055da4d1b"),
					Path: pointer.StringPtr("docs/cluster_resources.md"),
					Mode: pointer.StringPtr("100644"),
					Type: pointer.StringPtr("blob"),
					Size: new(int),
					URL:  pointer.StringPtr("https://api.github.com/repos/test-org/test-repo/git/blobs/b578f8f6cce210d44388e7136b9acce055da4d1b"),
				},
				shaAlias: "master",
			},
			expectedRL: &ResourceLocator{
				Host:     "api.github.com",
				Owner:    "test-org",
				Repo:     "test-repo",
				Path:     "docs/cluster_resources.md",
				SHA:      "b578f8f6cce210d44388e7136b9acce055da4d1b",
				Scheme:   "https",
				SHAAlias: "master",
				Type:     Blob,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TreeEntryToGitHubLocator(tt.args.treeEntry, tt.args.shaAlias); !reflect.DeepEqual(got, tt.expectedRL) {
				t.Errorf("TreeEntryToGitHubLocator() = %v, want %v", got, tt.expectedRL)
			}
		})
	}
}

func TestSetVersion(t *testing.T) {
	tests := []struct {
		url         string
		version     string
		expectedURL string
		expectedErr bool
	}{
		{
			"https://github.com/gardener/gardener/blob/master/docs/README.md",
			"v1.12.0",
			"https://github.com/gardener/gardener/blob/v1.12.0/docs/README.md",
			false,
		},
		{
			"https://github.com/gardener/gardener/pull/1234",
			"v1.12.0",
			"https://github.com/gardener/gardener/pull/1234",
			false,
		},
		{
			"https://kubernetes.io",
			"v1.12.0",
			"",
			true,
		},
	}
	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			gh := GitHub{}
			gotURL, gotErr := gh.SetVersion(tc.url, tc.version)
			if tc.expectedErr {
				assert.Error(t, gotErr)
			} else {
				assert.Nil(t, gotErr)
			}
			assert.Equal(t, tc.expectedURL, gotURL)
		})
	}
}
