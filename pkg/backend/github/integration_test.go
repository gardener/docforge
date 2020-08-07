package github

import (
	"context"
	"flag"
	"fmt"
	"testing"

	"github.com/gardener/docode/pkg/api"
	"github.com/gardener/docode/pkg/backend"
	"github.com/gardener/docode/pkg/jobs"
	"github.com/gardener/docode/pkg/jobs/worker"
	"github.com/gardener/docode/pkg/reactor"
	"gopkg.in/yaml.v2"

	"github.com/google/go-github/v32/github"
	"golang.org/x/oauth2"
)

// Run with:
// go test -timeout 30s -run ^TestResolveNodeSelectorLive$ -v -tags=integration --token=<your-token-here>

var ghToken = flag.String("token", "", "GitHub personal token for authenticating requests")

func TestResolveNodeSelectorLive(t *testing.T) {
	ctx := context.Background()

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: "76262afc3723033f1f07f47425d89f93d6798f03"},
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
	rh := backend.ResourceHandlers{
		gh,
	}
	reactor := reactor.Reactor{
		ResourceHandlers: rh,
		ReplicateDocumentation: &jobs.Job{
			MaxWorkers: 50,
			MinWorkers: 1,
			FailFast:   false,
			Worker: &worker.DocWorker{
				Writer: &worker.FSWriter{
					Root: "target",
				},
				RdCh: make(chan *worker.ResourceData),
				Reader: &worker.GenericReader{
					Handlers: rh,
				},
				Processor: &worker.EmptyProcessor{},
			},
		},
		ReplicateDocResources: &jobs.Job{
			MaxWorkers: 50,
			MinWorkers: 1,
			FailFast:   false,
			Worker: &worker.ResourceWorker{
				Reader: &worker.GenericReader{Handlers: rh},
			},
		},
	}

	docs := &api.Documentation{Root: node}
	if err := reactor.Serialize(ctx, docs); err != nil {
		t.Errorf("failed with: %v", err)
	}
}
