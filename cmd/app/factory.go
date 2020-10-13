package app

import (
	"context"
	"io"
	"path/filepath"

	"github.com/gardener/docforge/pkg/hugo"
	"github.com/gardener/docforge/pkg/metrics"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/writers"

	//"github.com/gardener/docforge/pkg/metrics"
	"github.com/gardener/docforge/pkg/processors"
	"github.com/gardener/docforge/pkg/reactor"
	ghrs "github.com/gardener/docforge/pkg/resourcehandlers/github"

	"github.com/google/go-github/v32/github"
	"golang.org/x/oauth2"
)

// Options is the set of parameters for creating
// reactor objects
type Options struct {
	MaxWorkersCount              int
	MinWorkersCount              int
	FailFast                     bool
	DestinationPath              string
	ResourcesPath                string
	ResourceDownloadWorkersCount int
	MarkdownFmt                  bool
	GitHubTokens                 map[string]string
	Metering                     *Metering
	DryRunWriter                 io.Writer
	Resolve                      bool
	Hugo                         *hugo.Options
}

// Metering encapsulates options for setting up client-side
// mettering
type Metering struct {
	Enabled bool
}

// NewReactor creates a Reactor from Options
func NewReactor(ctx context.Context, options *Options) *reactor.Reactor {
	dryRunWriters := writers.NewDryRunWritersFactory(options.DryRunWriter)
	o := &reactor.Options{
		MaxWorkersCount:              options.MaxWorkersCount,
		MinWorkersCount:              options.MinWorkersCount,
		FailFast:                     options.FailFast,
		DestinationPath:              options.DestinationPath,
		ResourcesPath:                options.ResourcesPath,
		ResourceDownloadWorkersCount: options.ResourceDownloadWorkersCount,
		MarkdownFmt:                  options.MarkdownFmt,
		Processor:                    nil,
		ResourceHandlers:             initResourceHanlders(ctx, options),
		DryRunWriter:                 dryRunWriters,
		Resolve:                      options.Resolve,
	}
	if options.DryRunWriter != nil {
		o.Writer = dryRunWriters.GetWriter(options.DestinationPath)
		o.ResourceDownloadWriter = dryRunWriters.GetWriter(filepath.Join(options.DestinationPath, options.ResourcesPath))
	} else {
		o.Writer = &writers.FSWriter{
			Root: options.DestinationPath,
		}
		o.ResourceDownloadWriter = &writers.FSWriter{
			Root: filepath.Join(options.DestinationPath, options.ResourcesPath),
		}
	}

	if options.Hugo != nil {
		WithHugo(o, options)
	}

	return reactor.NewReactor(o)
}

// WithHugo adapts the reactor.Options object with Hugo-specific
// settings for writer and processor
func WithHugo(reactorOptions *reactor.Options, o *Options) {
	hugoOptions := o.Hugo
	reactorOptions.Processor = &processors.ProcessorChain{
		Processors: []processors.Processor{
			&processors.FrontMatter{},
			hugo.NewProcessor(hugoOptions),
		},
	}
	if o.DryRunWriter != nil {
		hugoOptions.Writer = reactorOptions.Writer
	} else {
		hugoOptions.Writer = &writers.FSWriter{
			Root: filepath.Join(o.DestinationPath),
		}
	}
	reactorOptions.Writer = hugo.NewWriter(hugoOptions)
}

// initResourceHanlders initializes the resource handler
// objects used by reactors
func initResourceHanlders(ctx context.Context, o *Options) []resourcehandlers.ResourceHandler {
	rhs := []resourcehandlers.ResourceHandler{}
	if o.GitHubTokens != nil {
		if token, ok := o.GitHubTokens["github.com"]; ok {
			ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
			var client *github.Client
			if o.Metering != nil {
				client = github.NewClient(metrics.InstrumentClientRoundTripperDuration(oauth2.NewClient(ctx, ts)))
			} else {
				client = github.NewClient(oauth2.NewClient(ctx, ts))
			}
			gh := ghrs.NewResourceHandler(client, []string{"github.com", "raw.githubusercontent.com"})
			rhs = append(rhs, gh)
		}
	}
	return rhs
}
