package reactor

import (
	"context"
	"flag"
	"paths/filepath"
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

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: *ghToken})
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

	resourcesRoot:= "__resources"
	downloadJob:= reactor.NewResourceDownloadJob(nil, &writers.FSWriter{
		Root: filepath.Join("../../example/hugo/content/", resourcesRoot)
	}, 5, failFast)
	failFast:= false
	reactor := Reactor{
		ReplicateDocumentation: &jobs.Job{
			MaxWorkers: 50,
			FailFast:   failFast,
			Worker: &DocumentWorker{
				Writer: &writers.FSWriter{
					Root: "../../example/hugo/content",
				},
				Reader:           &GenericReader{},
				Processor:        Processors: []processors.Processor{
						&processors.FrontMatter{},
						&processors.HugoProcessor{
							PrettyUrls: true,
						},
				},
				NodeContentProcessor: reactor.NewNodeContentProcessor("/" + resourcesRoot, nil, downloadJob, failFast),
			},
		},
	}

	docs := &api.Documentation{Root: node}
	if err := reactor.Run(ctx, docs, false); err != nil {
		t.Errorf("failed with: %v", err)
	}

}
