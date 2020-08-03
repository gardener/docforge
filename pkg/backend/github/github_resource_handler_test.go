package github

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"os"
	"testing"
	"time"

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

/*
func TestGetTreeEntryIndex(t *testing.T) {
	cases := []struct {
		inURL   string
		want    map[string]*github.TreeEntry
		wantErr error
	}{
		{
			"https://api.github.com/repos/gardener/gardener/git/trees/master",
			map[string]*github.TreeEntry{
				".dockerignore": &github.TreeEntry{
					SHA:  github.String("5e27a248f3a7a3f9442c98b7e5d3c4b45b097491"),
					Path: github.String(".dockerignore"),
					Mode: github.String("100644"),
					Type: github.String("blob"),
					Size: github.Int(199),
					URL:  github.String("https://api.github.com/repos/gardener/gardener/git/blobs/5e27a248f3a7a3f9442c98b7e5d3c4b45b097491"),
				},
			},
			nil,
		},
	}
	for _, c := range cases {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		got, err := getTreeEntryIndex(ctx, c.inURL)
		if err != c.wantErr {
			fmt.Println(err)
			//t.Errorf("Something(%q) == %q, want %q", c.in, got, c.want)
			t.Fail()
			return
		}
		fmt.Printf("%v \n", got)
		if got != c.want {
			t.Errorf("Something(%q) == %q, want %q", c.in, got, c.want)
		}
	}

}
*/

func TestUrlToGitHubLocator(t *testing.T) {
	ghrl1 := &ResourceLocator{
		"github.com",
		"gardener",
		"gardener",
		"master",
		Tree,
		"docs/README.md",
		"",
	}
	ghrl1 := &ResourceLocator{
		"github.com",
		"gardener",
		"gardener",
		"master",
		Tree,
		"docs/README.md",
		"https://api.github.com/repos/gardener/gardener/git/blobs/91776959202ec10db883c5cfc05c51e78403f02c",
	}
	cases := []struct {
		inURL string
		inResolveAPI
		cache Cache
		mux  func() *http.Mux
		want *ResourceLocator
	}{
		{
			"https://github.com/gardener/gardener/tree/master/docs/README.md",
			false,
			Cache{
				"https://github.com/gardener/gardener/tree/master/docs/README.md": ghrl1,
			},
			nil,
			ghrl1,
		},
		{
			"https://github.com/gardener/gardener/tree/master/docs/README.md",
			true,
			Cache{},
			func(mux *http.Mux) {
				return mux.HandleFunc("/gardener/gardener/tree/master/docs/README.md", func(w http.ResponseWriter, r *http.Request) {
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
						}`))
				})
			},
			ghrl2,
		},
	}
	for _, c := range cases {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		gh: &GitHub{
			cache: inCache,
		}
		if c.inResolveAPI {
			client, mux, _, teardown := setup()
			defer teardown()
			if c.mux !=nil {
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
