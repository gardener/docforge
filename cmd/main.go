package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/gardener/docode/pkg/api"
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

var (
	configPath  string
	token       string
	destination string
	timeout     int
	dryRun	    bool
)

func main() {

	flag.StringVar(&configPath, "config", "", "path to configuration file")
	flag.StringVar(&destination, "destination", "", "path to write documentaiton bundle to")
	flag.StringVar(&token, "authToken", "", "the authentication token used for GitHub OAuth")
	flag.IntVar(&timeout, "timeout", 50, "timeout for replicating")
	flag.BoolVar(&dryRun, "dry-run", false, "simulates documentation structure resolution and download, printing the donwload sources and destinations")

	flag.Parse()

	validateFlags()

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	t := time.Duration(timeout) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), t)
	defer cancel()

	// TODO: make the client metering instrumentation optional and controlled by config
	// client := github.NewClient(metrics.InstrumentClientRoundTripperDuration(oauth2.NewClient(ctx, ts)))
	client := github.NewClient(oauth2.NewClient(ctx, ts))
	gh := ghrs.NewResourceHandler(client)
	resourcehandlers.Load(gh)

	reactor := reactor.Reactor{
		ReplicateDocumentation: &jobs.Job{
			MaxWorkers: 75,
			FailFast:   false,
			Worker: &reactor.DocumentWorker{
				Writer: &writers.FSWriter{
					Root: destination,
				},
				RdCh:   make(chan *reactor.ResourceData),
				Reader: &reactor.GenericReader{},
				Processor: &processors.ProcessorChain{
					Processors: []processors.Processor{
						&processors.FrontMatter{},
						&processors.HugoProcessor{
							PrettyUrls: true,
						},
					},
				},
				ContentProcessor: &reactor.ContentProcessor{
					ResourceAbsLink: make(map[string]string),
					LocalityDomain: reactor.LocalityDomain{},
				},
			},
		},
		LinkedResourceWorker: &reactor.LinkedResourceWorker{
			Reader: &reactor.GenericReader{},
			Writer: &writers.FSWriter{
				Root: destination + "/__resources",
			},
		},
	}

	var (
		docs *api.Documentation
	)
	configBytes, err := ioutil.ReadFile(configPath)
	if err != nil {
		panic(fmt.Sprintf("failed with: %v", err))
	}
	if docs, err = api.Parse(configBytes); err != nil {
		panic(fmt.Sprintf("failed with: %v", err))
	}
	if err = reactor.Run(ctx, docs, dryRun); err != nil {
		panic(fmt.Sprintf("failed with: %v", err))
	}

}

func validateFlags() {
	errors := make([]string, 0)
	if token == "" {
		errors = append(errors, "-authToken")
	}
	if configPath == "" {
		errors = append(errors, "-config")
	}
	if destination == "" {
		errors = append(errors, "-destination")
	}

	if len(errors) == 0 {
		return
	} else if len(errors) == 1 {
		panic(fmt.Sprintf("%s is not set", errors[0]))
	}

	panic(fmt.Sprintf("%v are not set", errors))
}
