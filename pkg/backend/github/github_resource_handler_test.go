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

	"github.com/gardener/docode/pkg/api"
	"github.com/gardener/docode/pkg/util/tests"
	"github.com/google/go-github/v32/github"
)

const (
	// baseURLPath is a non-empty Client.BaseURL path to use during tests,
	// to ensure relative URLs are used for all endpoints. See issue #752.
	baseURLPath = "/api-v3"
)

func init() {
	tests.SetGlogV(6)
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
		"github.com",
		"gardener",
		"gardener",
		"master",
		Blob,
		"docs/README.md",
		"",
	}
	ghrl2 := &ResourceLocator{
		"github.com",
		"gardener",
		"gardener",
		"master",
		Blob,
		"docs/README.md",
		"https://api.github.com/repos/gardener/gardener/git/blobs/91776959202ec10db883c5cfc05c51e78403f02c",
	}
	cases := []struct {
		description  string
		inURL        string
		inResolveAPI bool
		cache        Cache
		mux          func(mux *http.ServeMux)
		want         *ResourceLocator
	}{
		{
			"cached url should return valid GitHubResourceLocator",
			"https://github.com/gardener/gardener/blob/master/docs/README.md",
			false,
			Cache{
				"https://github.com/gardener/gardener/blob/master/docs/README.md": ghrl1,
			},
			nil,
			ghrl1,
		},
		{
			"non-cached url should resolve a valid GitHubResourceLocator from API",
			"https://github.com/gardener/gardener/blob/master/docs/README.md",
			true,
			Cache{},
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
	}
	for _, c := range cases {
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
		got := gh.URLToGitHubLocator(ctx, c.inURL, c.inResolveAPI)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("URLToGitHubLocator(%q) == %q, want %q", c.inURL, got, c.want)
		}
	}
}

func TestResolveNodeSelector(t *testing.T) {
	n1 := &api.Node{
		NodeSelector: &api.NodeSelector{
			Path: "https://github.com/gardener/gardener/tree/master/docs",
		},
	}
	cases := []struct {
		description string
		inNode      *api.Node
		mux         func(mux *http.ServeMux)
		want        *api.Node
		wantError   error
	}{
		{
			"resolve node selector",
			n1,
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
					&api.Node{
						Source: []string{"https://github.com/gardener/gardener/blob/master/docs/README.md"},
					},
					&api.Node{
						Source: []string{"https://github.com/gardener/gardener/tree/master/docs/concepts"},
						Nodes: []*api.Node{
							&api.Node{
								Source: []string{"https://github.com/gardener/gardener/blob/master/docs/concepts/apiserver.md"},
							},
						},
					},
				},
			},
			nil,
		},
	}
	for _, c := range cases {
		fmt.Println(c.description)
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		gh := &GitHub{
			cache: Cache{},
		}
		client, mux, _, teardown := setup()
		defer teardown()
		if c.mux != nil {
			c.mux(mux)
		}
		gh.Client = client
		gotError := gh.ResolveNodeSelector(ctx, c.inNode)
		if gotError != nil {
			t.Errorf("error == %q, want %q", gotError, c.wantError)
		}
		if !reflect.DeepEqual(c.inNode, c.want) {
			t.Errorf("ResolveNodeSelector == %v, want %v", c.inNode, c.want)
		}
	}
}

func TestName(t *testing.T) {
	ghrl1 := &ResourceLocator{
		"github.com",
		"gardener",
		"gardener",
		"master",
		Blob,
		"docs/README.md",
		"",
	}
	ghrl2 := &ResourceLocator{
		"github.com",
		"gardener",
		"gardener",
		"master",
		Tree,
		"docs",
		"",
	}
	cases := []struct {
		description string
		inURL       string
		cache       Cache
		want        string
	}{
		{
			"return file name for url",
			"https://github.com/gardener/gardener/blob/master/docs/README.md",
			Cache{
				"https://github.com/gardener/gardener/blob/master/docs/README.md": ghrl1,
			},
			"README.md",
		},
		{
			"return folder name for url",
			"https://github.com/gardener/gardener/tree/master/docs",
			Cache{
				"https://github.com/gardener/gardener/tree/master/docs": ghrl2,
			},
			"docs",
		},
	}
	for _, c := range cases {
		fmt.Println(c.description)
		gh := &GitHub{
			cache: c.cache,
		}
		got := gh.Name(c.inURL)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("Name(%q) == %q, want %q", c.inURL, got, c.want)
		}
	}
}

func TestRead(t *testing.T) {
	var sampleContent = []byte("Sample content")
	cases := []struct {
		description string
		inURI       string
		mux         func(mux *http.ServeMux)
		cache       Cache
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
			Cache{
				"https://github.com/gardener/gardener/blob/master/docs/README.md": &ResourceLocator{
					"github.com",
					"gardener",
					"gardener",
					"master",
					Blob,
					"docs/README.md",
					"",
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
		client, mux, serverUrl, teardown := setup()
		defer teardown()
		// rewrite cached url keys host to match the mock sevrer
		for k, v := range c.cache {
			c.cache[strings.Replace(k, "https://github.com", serverUrl, 1)] = v
		}
		gh := &GitHub{
			cache: c.cache,
		}
		if c.mux != nil {
			c.mux(mux)
		}
		gh.Client = client
		inURI := strings.Replace(c.inURI, "https://github.com", serverUrl, 1)
		got, gotError := gh.Read(ctx, c.inURI)
		if gotError != nil {
			t.Errorf("error == %q, want %q", gotError, c.wantError)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("Read(ctx,%v) == %v, want %v", inURI, string(got), string(c.want))
		}
	}
}
