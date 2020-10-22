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
		Structure: []*api.Node{
			&api.Node{
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
		},
		Links: &api.Links{
			Rewrites: map[string]*api.LinkRewriteRule{
				"gardener/gardener/(blob|tree|raw)": &api.LinkRewriteRule{
					Version: tests.StrPtr("v1.10.0"),
				},
				"gardener/gardener-extension-provider-aws/(blob|tree|raw)": &api.LinkRewriteRule{
					Version: tests.StrPtr("v1.15.3"),
				},
				"gardener/gardener-extension-networking-calico/(blob|tree|raw)": &api.LinkRewriteRule{
					Version: tests.StrPtr("v1.10.0"),
				},
			},
			Downloads: &api.Downloads{
				Scope: map[string]api.ResourceRenameRules{
					"gardener/gardener/(blob|tree|raw)/v1.10.0/docs":                             nil,
					"gardener/gardener-extension-provider-aws/(blob|tree|raw)/v1.15.3/docs":      nil,
					"gardener/gardener-extension-networking-calico/(blob|tree|raw)/v1.10.0/docs": nil,
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
