// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"github.com/gardener/docforge/cmd/app"
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

func main() {
	if len(os.Getenv("GOMAXPROCS")) == 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

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

	command := app.NewCommand(ctx)
	if err := flag.CommandLine.Parse([]string{}); err != nil {
		panic(err.Error())
	}
	if err := command.Execute(); err != nil {
		os.Exit(-1)
	}
}
