package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/gardener/docode/pkg/replicator"
	"github.com/google/go-github/v32/github"
	"golang.org/x/oauth2"
)

var (
	source     string
	version    string
	sourcePath string
	targetDir  string
	token      string
)

func main() {

	flag.StringVar(&version, "sourceVersion", "", "sourceVersion is the version of the source, when using GitHub this has to be the branch, from which documentation would be replicated.")
	flag.StringVar(&targetDir, "targetDir", "target", "targetDir is where replicated content will take place. Default to \"target\".")
	flag.StringVar(&sourcePath, "sourcePath", "", "sourcePath is the path to the replicated content from the source.")
	flag.StringVar(&source, "sourceURL", "", "sourceURL is the URL to the source e.g github.com/gardener/gardener")
	flag.StringVar(&token, "authToken", "", "the authentication token used for OAuth")

	flag.Parse()

	validateFlags()

	var (
		ctx = context.Background()
		ts  = oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
	)

	// TODO: make the client metering instrumentaiton optional and controlled by config
	client := github.NewClient(metrics.InstrumentClientRoundTripperDuration(oauth2.NewClient(ctx, ts)))

	desc := &replicator.Description{
		Path:    sourcePath,
		Version: version,
		Source:  source,
		Target:  targetDir,
	}

	replicator.Replicate(ctx, client, desc)
}

func validateFlags() {
	errors := make([]string, 0)
	if token == "" {
		errors = append(errors, "-authToken")
	}
	if version == "" {
		errors = append(errors, "-sourceVersion")
	}
	if source == "" {
		errors = append(errors, "-sourceURL")
	}
	if sourcePath == "" {
		errors = append(errors, "-sourcePath")
	}
	if targetDir == "" {
		errors = append(errors, "-targetDir")
	}

	if len(errors) == 0 {
		return
	} else if len(errors) == 1 {
		panic(fmt.Sprintf("%s is not set", errors[0]))
	}

	panic(fmt.Sprintf("%v are not set", errors))
}
