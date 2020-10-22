package app

import (
	"context"
	"flag"
	"io"
	"os"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/hugo"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

type cmdFlags struct {
	maxWorkersCount              int
	minWorkersCount              int
	failFast                     bool
	destinationPath              string
	documentationManifestPath    string
	resourcesPath                string
	resourceDownloadWorkersCount int
	markdownFmt                  bool
	rewriteEmbedded              bool
	ghOAuthToken                 string
	ghInfoDestination            string
	dryRun                       bool
	resolve                      bool
	clientMetering               bool
	hugo                         bool
	hugoPrettyUrls               bool
	hugoSectionFiles             []string
}

// NewCommand creates a new root command and propagates
// the context and cancel function to its Run callback closure
func NewCommand(ctx context.Context, cancel context.CancelFunc) *cobra.Command {
	flags := &cmdFlags{}
	cmd := &cobra.Command{
		Use:   "docforge",
		Short: "Build documentation bundle",
		RunE: func(cmd *cobra.Command, args []string) error {
			options := NewOptions(flags)
			doc := Manifest(flags.documentationManifestPath)
			if err := api.ValidateManifest(doc); err != nil {
				return err
			}
			reactor := NewReactor(ctx, options, doc.Links)
			if err := reactor.Run(ctx, doc, flags.dryRun); err != nil {
				return err
			}
			return nil
		},
	}

	flags.Configure(cmd)

	version := NewVersionCmd()
	cmd.AddCommand(version)

	klog.InitFlags(nil)
	AddFlags(cmd)

	return cmd
}

// Configure configures flags for command
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
		"GitHub personal token authorizing reading from GitHub repositories.")
	command.Flags().StringVar(&flags.ghInfoDestination, "github-info-destination", "",
		"If specified, docforge will download also additional github info for the files from the documentation structure into this destination.")
	command.Flags().BoolVar(&flags.rewriteEmbedded, "rewrite-embedded-to-raw", true,
		"Rewrites absolute link destinations for embedded resources (images) to reference embedable media (e.g. for GitHub - reference to a 'raw' version of an image).")
	command.Flags().BoolVar(&flags.failFast, "fail-fast", false,
		"Fail-fast vs fault tolerant operation.")
	command.Flags().BoolVar(&flags.markdownFmt, "markdownfmt", true,
		"Applies formatting rules to source markdown.")
	command.Flags().BoolVar(&flags.dryRun, "dry-run", false,
		"Runs the command end-to-end but instead of writing files, it will output the projected file/folder hierarchy to the standard output and statistics for the processing of each file.")
	command.Flags().BoolVar(&flags.resolve, "resolve", false,
		"Resolves the documentation structure and prints it to the standard output. The resolution expands nodeSelector constructs into node hierarchies.")
	command.Flags().IntVar(&flags.minWorkersCount, "min-workers", 10,
		"Minimum number of parallel workers.")
	command.Flags().IntVar(&flags.maxWorkersCount, "max-workers", 25,
		"Maximum number of parallel workers.")
	command.Flags().IntVar(&flags.resourceDownloadWorkersCount, "download-workers", 10,
		"Number of workers downloading document resources in parallel.")
	// Disabled until "fatal error: concurrent map writes" is fixed
	// command.Flags().BoolVar(&flags.clientMetering, "metering", false,
	// 	"Enables client-side networking metering to produce Prometheus compliant series.")
	command.Flags().BoolVar(&flags.hugo, "hugo", false,
		"Build documentation bundle for hugo.")
	command.Flags().BoolVar(&flags.hugoPrettyUrls, "hugo-pretty-urls", true,
		"Build documentation bundle for hugo with pretty URLs (./sample.md -> ../sample). Only useful with --hugo=true")
	command.Flags().StringSliceVar(&flags.hugoSectionFiles, "hugo-section-files", []string{"readme", "read.me", "index"},
		"When building a Hugo-compliant documentaton bundle, files with filename matching one form this list (in that order) will be renamed to _index.md. Only useful with --hugo=true")
}

// NewOptions creates an options object from flags
func NewOptions(f *cmdFlags) *Options {
	var (
		tokens       map[string]string
		metering     *Metering
		hugoOptions  *hugo.Options
		dryRunWriter io.Writer
	)
	if len(f.ghOAuthToken) > 0 {
		tokens = map[string]string{
			// TODO: Currently only github is passed and hardcoded, because there is no flag format supporting multiple tokens
			"github.com": f.ghOAuthToken,
		}
	}
	if f.clientMetering {
		metering = &Metering{
			Enabled: f.clientMetering,
		}
	}
	if f.hugo {
		hugoOptions = &hugo.Options{
			PrettyUrls:     f.hugoPrettyUrls,
			IndexFileNames: f.hugoSectionFiles,
			Writer:         nil,
		}
	}

	if f.dryRun {
		dryRunWriter = os.Stdout
	}

	return &Options{
		DestinationPath:              f.destinationPath,
		FailFast:                     f.failFast,
		MaxWorkersCount:              f.maxWorkersCount,
		MinWorkersCount:              f.minWorkersCount,
		ResourceDownloadWorkersCount: f.resourceDownloadWorkersCount,
		ResourcesPath:                f.resourcesPath,
		MarkdownFmt:                  f.markdownFmt,
		RewriteEmbedded:              f.rewriteEmbedded,
		GitHubTokens:                 tokens,
		Metering:                     metering,
		DryRunWriter:                 dryRunWriter,
		Resolve:                      f.resolve,
		GitHubInfoPath:               f.ghInfoDestination,
		Hugo:                         hugoOptions,
	}
}

// AddFlags adds go flags to rootCmd
func AddFlags(rootCmd *cobra.Command) {
	flag.CommandLine.VisitAll(func(gf *flag.Flag) {
		rootCmd.Flags().AddGoFlag(gf)
	})
}
