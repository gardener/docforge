package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"os"
	"os/signal"
	"syscall"
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
	hugo	    bool
)

func main() {

	flag.StringVar(&configPath, "config", "", "path to configuration file")
	flag.StringVar(&destination, "destination", "", "path to write documentaiton bundle to")
	flag.StringVar(&token, "authToken", "", "the authentication token used for GitHub OAuth")
	flag.IntVar(&timeout, "timeout", 50, "timeout for replicating")
	flag.BoolVar(&dryRun, "dry-run", false, "simulates documentation structure resolution and download, printing the donwload sources and destinations")
	flag.BoolVar(&hugo, "hugo", false, "enables post-processing for Hugo")

	t := time.Duration(timeout) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), t)
	c := make(chan os.Signal, 2)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer func() {
		signal.Stop(c)
		cancel()
	}()
	go func() {
		<-c
		cancel()
		<-c
		os.Exit(1)
	}()

	flag.Parse()
	validateFlags()

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	// TODO: make the client metering instrumentation optional and controlled by config
	// client := github.NewClient(metrics.InstrumentClientRoundTripperDuration(oauth2.NewClient(ctx, ts)))
	client := github.NewClient(oauth2.NewClient(ctx, ts))
	gh := ghrs.NewResourceHandler(client)
	resourcehandlers.Load(gh)

	resourcesRoot:= "__resources"
	failFast:= false
	downloadJob:= reactor.NewResourceDownloadJob(nil, &writers.FSWriter{
		Root: filepath.Join(destination, resourcesRoot),
		//TMP
		Hugo: hugo,
	}, 5, failFast)
	var processor  processors.Processor
	if hugo {
		processor = &processors.ProcessorChain{
			Processors: []processors.Processor{
				&processors.FrontMatter{},
				&processors.HugoProcessor{
					PrettyUrls: true,
				},
			},
		}
	}
	reactor:= reactor.Reactor{
		ReplicateDocumentation: &jobs.Job{
			MinWorkers: 75,
			MaxWorkers: 75,
			FailFast:   false,
			Worker: &reactor.DocumentWorker{
				Writer: &writers.FSWriter{
					Root: destination,
				},
				Reader: &reactor.GenericReader{},
				Processor: processor,
				NodeContentProcessor: reactor.NewNodeContentProcessor("/" + resourcesRoot, nil, downloadJob, failFast),
			},
		},
	}

	var (
		docs *api.Documentation
	)
	configBytes, err := ioutil.ReadFile(configPath)
	if err != nil {
		panic(fmt.Sprintf("%v\n", err))
	}
	if docs, err = api.Parse(configBytes); err != nil {
		panic(fmt.Sprintf("%v\n", err))
	}
	if err = reactor.Run(ctx, docs, dryRun); err != nil {
		panic(fmt.Sprintf("%v\n", err))
		os.Exit(1)
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
