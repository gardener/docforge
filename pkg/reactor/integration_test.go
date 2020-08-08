package reactor

import (
	"context"
	"flag"
	"testing"
	"time"

	"github.com/gardener/docode/pkg/api"
	"github.com/gardener/docode/pkg/backend"
	"github.com/gardener/docode/pkg/backend/github"
	"github.com/gardener/docode/pkg/jobs"
	"github.com/gardener/docode/pkg/processors"
	"github.com/gardener/docode/pkg/util/tests"
	"github.com/gardener/docode/pkg/writers"

	githubapi "github.com/google/go-github/v32/github"
	"golang.org/x/oauth2"
)

// Run with:
// go test -timeout 30s -run ^TestReactorWithGitHub$ -v -tags=integration --token=<your-token-here>

var ghToken = flag.String("token", "", "GitHub personal token for authenticating requests")

func init() {
	tests.SetGlogV(6)
}

func TestReactorWithGitHub(t *testing.T) {
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "97c4b9600449d854abe087dbff2c9754d1b9a54c"})
	gh := github.NewResourceHandler(githubapi.NewClient(oauth2.NewClient(ctx, ts)))
	node := &api.Node{
		Name: "docs",
		NodeSelector: &api.NodeSelector{
			Path: "https://github.com/gardener/gardener/tree/master/docs",
		},
	}
	rh := backend.ResourceHandlers{
		gh,
	}
	reactor := Reactor{
		ResourceHandlers: rh,
		ReplicateDocumentation: &jobs.Job{
			MaxWorkers: 50,
			FailFast:   false,
			Worker: &DocumentWorker{
				Writer: &writers.FSWriter{
					Root: "target",
				},
				RdCh: make(chan *ResourceData),
				Reader: &GenericReader{
					Handlers: rh,
				},
				Processor: &processors.FrontMatter{},
			},
		},
		ReplicateDocResources: &jobs.Job{
			MaxWorkers: 50,
			FailFast:   false,
			Worker: &LinkedResourceWorker{
				Reader: &GenericReader{Handlers: rh},
			},
		},
	}

	docs := &api.Documentation{Root: node}
	if err := reactor.Run(ctx, docs); err != nil {
		t.Errorf("failed with: %v", err)
	}

}
