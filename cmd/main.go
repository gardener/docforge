package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/gardener/docode/cmd/app"
)

func main() {
	if len(os.Getenv("GOMAXPROCS")) == 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	// t := time.Duration(timeout) * time.Second
	// ctx, cancel := context.WithTimeout(context.Background(), t)
	ctx, cancel := context.WithCancel(context.Background())
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

	// resourcesRoot := "__resources"
	// failFast := false
	// downloadJob := reactor.NewResourceDownloadJob(nil, &writers.FSWriter{
	// 	Root: filepath.Join(destination, resourcesRoot),
	// 	//TMP
	// 	Hugo: hugo,
	// }, 5, failFast)
	// var processor processors.Processor
	// if hugo {
	// 	processor = &processors.ProcessorChain{
	// 		Processors: []processors.Processor{
	// 			&processors.FrontMatter{},
	// 			&processors.HugoProcessor{
	// 				PrettyUrls: true,
	// 			},
	// 		},
	// 	}
	// }
	// reactor := reactor.Reactor{
	// 	ReplicateDocumentation: &jobs.Job{
	// 		MinWorkers: 75,
	// 		MaxWorkers: 75,
	// 		FailFast:   false,
	// 		Worker: &reactor.DocumentWorker{
	// 			Writer: &writers.FSWriter{
	// 				Root: destination,
	// 			},
	// 			Reader:               &reactor.GenericReader{},
	// 			Processor:            processor,
	// 			NodeContentProcessor: reactor.NewNodeContentProcessor("/"+resourcesRoot, nil, downloadJob, failFast),
	// 		},
	// 	},
	// }

	command := app.NewCommand(ctx, cancel)
	flag.CommandLine.Parse([]string{})
	if err := command.Execute(); err != nil {
		os.Exit(-1)
	}
}
