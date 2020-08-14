package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"

	"github.com/gardener/docode/pkg/api"
	"github.com/gardener/docode/pkg/jobs"
	"github.com/gardener/docode/pkg/metrics"
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
)

func main() {

	flag.StringVar(&configPath, "config", "", "path to configuration file")
	flag.StringVar(&destination, "destination", "", "path to write documentaiton bundle to")
	flag.StringVar(&token, "authToken", "", "the authentication token used for GitHub OAuth")

	flag.Parse()

	validateFlags()

	var (
		ctx = context.Background()
		ts  = oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
	)

	// TODO: make the client metering instrumentation optional and controlled by config
	client := github.NewClient(metrics.InstrumentClientRoundTripperDuration(oauth2.NewClient(ctx, ts)))
	resourcehandlers.Load(ghrs.NewResourceHandler(client))

	reactor := reactor.Reactor{
		ReplicateDocumentation: &jobs.Job{
			MaxWorkers: 50,
			FailFast:   false,
			Worker: &reactor.DocumentWorker{
				Writer: &writers.FSWriter{
					Root: "target",
				},
				RdCh:      make(chan *reactor.ResourceData),
				Reader:    &reactor.GenericReader{},
				Processor: &processors.FrontMatter{},
			},
		},
		ReplicateDocResources: &jobs.Job{
			MaxWorkers: 50,
			FailFast:   false,
			Worker: &reactor.LinkedResourceWorker{
				Reader: &reactor.GenericReader{},
				Writer: &writers.FSWriter{
					Root: destination,
				},
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
	if err = reactor.Run(ctx, docs); err != nil {
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
