package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/gardener/docforge/cmd/app"
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

	command := app.NewCommand(ctx, cancel)
	if err := flag.CommandLine.Parse([]string{}); err != nil {
		panic(err.Error())
	}
	if err := command.Execute(); err != nil {
		os.Exit(-1)
	}
}
