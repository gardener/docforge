package app

import (
	"context"
	"flag"

	"github.com/spf13/cobra"
)

type cmdFlags struct {
	maxWorkersCount              int
	minWorkersCount              int
	failFast                     bool
	destinationPath              string
	documentationManifestPath    string
	resourcesPath                string
	resourceDownloadWorkersCount int
	ghOAuthToken                 string
	dryRun                       bool
	hugo                         bool
	prettyUrls                   bool
}

func NewCommand(ctx context.Context, cancel context.CancelFunc) *cobra.Command {
	flags := &cmdFlags{}
	cmd := &cobra.Command{
		Use:   "docode",
		Short: "Build documentation bundle",
		Run: func(cmd *cobra.Command, args []string) {
			options := NewOptions(flags)
			doc := Manifest(flags.documentationManifestPath)
			InitResourceHanlders(ctx, flags.ghOAuthToken)
			reactor := NewReactor(options)
			if err := reactor.Run(ctx, doc, flags.dryRun); err != nil {
				panic(err)
			}
		},
	}

	flags.Configure(cmd)

	version := NewVersionCmd()
	cmd.AddCommand(version)

	AddFlags(cmd)

	return cmd
}

func (flags *cmdFlags) Configure(command *cobra.Command) {
	command.Flags().StringVarP(&flags.destinationPath, "destination", "d", "",
		"Destination path.")
	command.MarkFlagRequired("destination")
	command.Flags().StringVarP(&flags.documentationManifestPath, "manifest", "f", "",
		"Manifest path.")
	command.MarkFlagRequired("manifest")
	command.Flags().StringVar(&flags.resourcesPath, "resources-download-path", "__resources",
		"Resources download path.")
	command.Flags().StringVar(&flags.ghOAuthToken, "github-oauth-token", "",
		"GitHub personal token authorizing reading from GitHub repos.")
	command.Flags().BoolVar(&flags.failFast, "fail-fast", false,
		"Fail-fast vs fault tolerant operation.")
	command.Flags().BoolVar(&flags.dryRun, "dry-run", false,
		"Resolves and prints the resolved documentation structure without downloading anything.")
	command.Flags().IntVar(&flags.minWorkersCount, "min-workers", 10,
		"Minimum number of parallel workers.")
	command.Flags().IntVar(&flags.maxWorkersCount, "max-workers", 25,
		"Maximum number of parallel workers.")
	command.Flags().IntVar(&flags.resourceDownloadWorkersCount, "download-workers", 10,
		"Number of workers downloading document resources in parallel.")

	command.Flags().BoolVar(&flags.hugo, "hugo", false,
		"Build documentation bundle for hugo.")
	command.Flags().BoolVar(&flags.prettyUrls, "pretty-urls", true,
		"Build documentation bundle for hugo with pretty URLs (./sample.md -> ../sample).")
}

func NewOptions(f *cmdFlags) *Options {
	var hugoOptions *Hugo
	if f.hugo {
		hugoOptions = &Hugo{
			PrettyUrls: f.prettyUrls,
		}
	}
	return &Options{
		DestinationPath:              f.destinationPath,
		FailFast:                     f.failFast,
		MaxWorkersCount:              f.maxWorkersCount,
		MinWorkersCount:              f.minWorkersCount,
		ResourceDownloadWorkersCount: f.resourceDownloadWorkersCount,
		ResourcesPath:                f.resourcesPath,
		Hugo:                         hugoOptions,
	}
}

func AddFlags(rootCmd *cobra.Command) {
	flag.CommandLine.VisitAll(func(gf *flag.Flag) {
		rootCmd.Flags().AddGoFlag(gf)
	})
}
