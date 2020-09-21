package app

import (
	"context"
	"path/filepath"

	"github.com/gardener/docode/pkg/jobs"
	//"github.com/gardener/docode/pkg/metrics"
	"github.com/gardener/docode/pkg/processors"
	"github.com/gardener/docode/pkg/reactor"
	"github.com/gardener/docode/pkg/resourcehandlers"
	ghrs "github.com/gardener/docode/pkg/resourcehandlers/github"
	"github.com/gardener/docode/pkg/writers"

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
	*Hugo
}

// Hugo is a set of parameters for creating
// reactor objects for Hugo builds
type Hugo struct {
	PrettyUrls bool
}

// NewReactor creates a Reactor from Options
func NewReactor(o *Options) *reactor.Reactor {

	downloadJob := reactor.NewResourceDownloadJob(nil, &writers.FSWriter{
		Root: filepath.Join(o.DestinationPath, o.ResourcesPath),
		//TMP
		Hugo: (o.Hugo != nil),
	}, o.ResourceDownloadWorkersCount, o.FailFast)

	r := &reactor.Reactor{
		ReplicateDocumentation: &jobs.Job{
			MinWorkers: o.MinWorkersCount,
			MaxWorkers: o.MaxWorkersCount,
			FailFast:   o.FailFast,
			Worker: &reactor.DocumentWorker{
				Writer: &writers.FSWriter{
					Root: o.DestinationPath,
				},
				Reader:               &reactor.GenericReader{},
				NodeContentProcessor: reactor.NewNodeContentProcessor("/"+o.ResourcesPath, nil, downloadJob, o.FailFast, o.MarkdownFmt),
			},
		},
		FailFast: o.FailFast,
	}

	if o.Hugo != nil {
		if worker, ok := r.ReplicateDocumentation.Worker.(*reactor.DocumentWorker); ok {
			worker.Processor = &processors.ProcessorChain{
				Processors: []processors.Processor{
					&processors.FrontMatter{},
					&processors.HugoProcessor{
						PrettyUrls: true,
					},
				},
			}
		}

	}

	return r
}

// InitResourceHanlders initializes the resource handler
// objects used by reactors
func InitResourceHanlders(ctx context.Context, githubToken string) {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: githubToken})
	// TODO: make the client metering instrumentation optional and controlled by config
	// client := github.NewClient(metrics.InstrumentClientRoundTripperDuration(oauth2.NewClient(ctx, ts)))
	client := github.NewClient(oauth2.NewClient(ctx, ts))
	gh := ghrs.NewResourceHandler(client)
	resourcehandlers.Load(gh)
}
