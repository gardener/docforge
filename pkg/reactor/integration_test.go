package reactor

import (
	"context"
	"flag"
	"testing"
	"time"

	"github.com/gardener/docode/pkg/api"
	"github.com/gardener/docode/pkg/jobs"
	"github.com/gardener/docode/pkg/processors"
	"github.com/gardener/docode/pkg/resourcehandlers"
	"github.com/gardener/docode/pkg/resourcehandlers/github"
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
	timeout := 30 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "76262afc3723033f1f07f47425d89f93d6798f03"})
	gh := github.NewResourceHandler(githubapi.NewClient(oauth2.NewClient(ctx, ts)))
	node := &api.Node{
		Name: "docs",
		NodeSelector: &api.NodeSelector{
			Path: "https://github.com/gardener/gardener/tree/master/docs",
		},
		Nodes: []*api.Node{
			{
				Name: "calico",
				NodeSelector: &api.NodeSelector{
					Path: "https://github.com/gardener/gardener-extension-networking-calico/tree/master/docs",
				},
			},
			{
				Name: "aws",
				NodeSelector: &api.NodeSelector{
					Path: "https://github.com/gardener/gardener-extension-provider-aws/tree/master/docs",
				},
			},
		},
	}

	// init gh resource handler
	resourcehandlers.Load(gh)

	reactor := Reactor{
		ReplicateDocumentation: &jobs.Job{
			MaxWorkers: 50,
			FailFast:   false,
			Worker: &DocumentWorker{
				Writer: &writers.FSWriter{
					Root: "../../example/hugo/content",
				},
				RdCh:             make(chan *ResourceData),
				Reader:           &GenericReader{},
				Processor:        &processors.FrontMatter{},
				contentProcessor: &ContentProcessor{resourceAbsLink: make(map[string]string)},
			},
		},
		LinkedResourceWorker: &LinkedResourceWorker{
			Reader: &GenericReader{},
			Writer: &writers.FSWriter{
				Root: "target/__resources",
			},
		},
	}

	docs := &api.Documentation{Root: node}
	if err := reactor.Run(ctx, docs); err != nil {
		t.Errorf("failed with: %v", err)
	}

}
