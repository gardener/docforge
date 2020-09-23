package github

import (
	"context"
	"flag"
	"fmt"
	"testing"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/util/tests"

	"github.com/google/go-github/v32/github"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v3"
)

// Run with:
// go test -timeout 30s -run ^TestResolveNodeSelectorLive$ -v -tags=integration --token=<your-token-here>

var ghToken = flag.String("token", "", "GitHub personal token for authenticating requests")

func init() {
	tests.SetGlogV(6)
}

func TestResolveNodeSelectorLive(t *testing.T) {
	ctx := context.Background()

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: *ghToken},
	)
	gh := &GitHub{
		Client: github.NewClient(oauth2.NewClient(ctx, ts)),
		cache:  Cache{},
	}
	node := &api.Node{
		Name: "docs",
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
