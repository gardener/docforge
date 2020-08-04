// +build integration

package github

import (
	"context"
	"flag"
	"fmt"
	"testing"
	"time"

	"github.com/gardener/docode/pkg/api"
	"github.com/google/go-github/v32/github"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v3"
)

// Run with:
// go test -timeout 30s -run ^TestResolveNodeSelectorLive$ -v -tags=integration --token=<your-token-here>

var ghToken = flag.String("token", "", "GitHub personal token for authenticating requests")

func TestResolveNodeSelectorLive(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5000*time.Millisecond)
	defer cancel()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: *ghToken},
	)
	gh := &GitHub{
		Client: github.NewClient(oauth2.NewClient(ctx, ts)),
		cache:  Cache{},
	}
	node := &api.Node{
		NodeSelector: &api.NodeSelector{
			Path: "https://github.com/gardener/gardener/tree/master/docs",
		},
	}
	if err := gh.ResolveNodeSelector(ctx, node); err != nil {
		fmt.Printf("%v", err)
	}
	b, _ := yaml.Marshal(node)
	fmt.Println(string(b))
}
