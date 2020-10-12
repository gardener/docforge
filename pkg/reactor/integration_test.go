package reactor

import (
	"context"
	"flag"
	"path/filepath"
	"testing"
	"time"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/hugo"
	"github.com/gardener/docforge/pkg/processors"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/resourcehandlers/github"
	"github.com/gardener/docforge/pkg/util/tests"
	"github.com/gardener/docforge/pkg/writers"

	githubapi "github.com/google/go-github/v32/github"
	"golang.org/x/oauth2"
)

// Run with:
// go test -timeout 30s -run ^_TestReactorWithGitHub$ -v -tags=integration --token=<your-token-here>

var ghToken = flag.String("token", "", "GitHub personal token for authenticating requests")

func init() {
	tests.SetKlogV(6)
}

func _TestReactorWithGitHub(t *testing.T) {
	timeout := 300 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	docs := &api.Documentation{
		Root: &api.Node{
			Name: "docs",
			NodeSelector: &api.NodeSelector{
				Path: "https://github.com/gardener/gardener/tree/v1.10.0/docs",
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
		},
		LocalityDomain: &api.LocalityDomain{
			LocalityDomainMap: map[string]*api.LocalityDomainValue{
				"github.com/gardener/gardener": &api.LocalityDomainValue{
					Version: "v1.10.0",
					Path:    "gardener/gardener/docs",
				},
				"github.com/gardener/gardener-extension-provider-aws": &api.LocalityDomainValue{
					Version: "master",
					Path:    "gardener/gardener-extension-provider-aws/docs",
				},
				"github.com/gardener/gardener-extension-networking-calico": &api.LocalityDomainValue{
					Version: "master",
					Path:    "gardener/gardener-extension-networking-calico/docs",
				},
			},
		},
	}

	destination := "../../dev"
	resourcesRoot := "/__resources"
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: *ghToken})
	ghClient := githubapi.NewClient(oauth2.NewClient(ctx, ts))
	gh := github.NewResourceHandler(ghClient, []string{"github.com"})
	writer := &writers.FSWriter{
		Root: destination,
	}
	hugoOptions := &hugo.Options{
		IndexFileNames: []string{"_index", "index", "readme", "read.me"},
		PrettyUrls:     true,
		Writer:         writer,
	}
	options := &Options{
		MaxWorkersCount:              10,
		MinWorkersCount:              5,
		FailFast:                     true,
		DestinationPath:              destination,
		ResourcesPath:                resourcesRoot,
		ResourceDownloadWorkersCount: 4,
		MarkdownFmt:                  true,
		Processor: &processors.ProcessorChain{
			Processors: []processors.Processor{
				&processors.FrontMatter{},
				hugo.NewProcessor(hugoOptions),
			},
		},
		ResourceDownloadWriter: &writers.FSWriter{
			Root: filepath.Join(destination, resourcesRoot),
		},
		Writer:           hugo.NewWriter(hugoOptions),
		ResourceHandlers: []resourcehandlers.ResourceHandler{gh},
	}
	r := NewReactor(options)

	if err := r.Run(ctx, docs, false); err != nil {
		t.Errorf("failed with: %v", err)
	}

}
