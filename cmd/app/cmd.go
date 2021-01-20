// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"flag"
	"io"
	"os"
	"path/filepath"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/hugo"
	"github.com/gardener/docforge/pkg/resourcehandlers"
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
	rewriteEmbedded              bool
	variables                    map[string]string
	ghOAuthToken                 string
	ghOAuthTokens                map[string]string
	ghInfoDestination            string
	ghThrottling                 bool
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
		Short: "Forge a documentation bundle",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			var (
				doc *api.Documentation
				rhs []resourcehandlers.ResourceHandler
				err error
			)
			options := NewOptions(flags)
			if rhs, err = initResourceHandlers(ctx, options.GitHubTokens, options.GitHubClientThrottling, options.Metering); err != nil {
				return err
			}
			if doc, err = manifest(ctx, flags.documentationManifestPath, rhs, flags.variables); err != nil {
				return err
			}
			if err := api.ValidateManifest(doc); err != nil {
				return err
			}
			reactor, err := NewReactor(ctx, options, doc.Links)
			if err != nil {
				return err
			}
			if err := reactor.Run(ctx, doc, flags.dryRun); err != nil {
				return err
			}
			return nil
		},
	}

	flags.Configure(cmd)

	version := NewVersionCmd()
	cmd.AddCommand(version)

	completion := NewCompletionCmd()
	cmd.AddCommand(completion)
	genCmdDocs := NewGenCmdDocs()
	cmd.AddCommand(genCmdDocs)

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
		"GitHub personal token authorizing read access from GitHub.com repositories. For authorization credentials for multiple GitHub instances, see --github-oauth-token-map")
	command.Flags().StringToStringVar(&flags.ghOAuthTokens, "github-oauth-token-map", map[string]string{},
		"GitHub personal tokens authorizing read access from repositories per GitHub instance. Note that if the GitHub token is already provided by `github-oauth-token` it will be overridden by it.")
	command.Flags().BoolVar(&flags.ghThrottling, "github-throttling", false,
		"Enable throttling of requests to GitHub API. The throttling is adaptive and will slow down execution with the approaching rate limit. Use to improve continuity. Disable to maximise performance.")
	command.Flags().StringVar(&flags.ghInfoDestination, "github-info-destination", "",
		"If specified, docforge will download also additional github info for the files from the documentation structure into this destination.")
	command.Flags().BoolVar(&flags.rewriteEmbedded, "rewrite-embedded-to-raw", true,
		"Rewrites absolute link destinations for embedded resources (images) to reference embedable media (e.g. for GitHub - reference to a 'raw' version of an image).")
	command.Flags().StringToStringVar(&flags.variables, "variables", map[string]string{},
		"Variables applied to parameterized (using Go template) manifest.")
	command.Flags().BoolVar(&flags.failFast, "fail-fast", false,
		"Fail-fast vs fault tolerant operation.")
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
		hugoOptions  *hugo.Options
		dryRunWriter io.Writer
		metering     *Metering
	)

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

	path, err := os.Getwd()
	if err != nil {
		//TODO:
	}

	manifestAbsPath := filepath.Join(path, (f.documentationManifestPath))

	return &Options{
		DestinationPath:              f.destinationPath,
		FailFast:                     f.failFast,
		MaxWorkersCount:              f.maxWorkersCount,
		MinWorkersCount:              f.minWorkersCount,
		ResourceDownloadWorkersCount: f.resourceDownloadWorkersCount,
		ResourcesPath:                f.resourcesPath,
		RewriteEmbedded:              f.rewriteEmbedded,
		GitHubTokens:                 gatherTokens(f),
		GitHubClientThrottling:       f.ghThrottling,
		Metering:                     metering,
		DryRunWriter:                 dryRunWriter,
		Resolve:                      f.resolve,
		GitHubInfoPath:               f.ghInfoDestination,
		Hugo:                         hugoOptions,
		ManifestAbsPath:              manifestAbsPath,
	}
}

// AddFlags adds go flags to rootCmd
func AddFlags(rootCmd *cobra.Command) {
	flag.CommandLine.VisitAll(func(gf *flag.Flag) {
		rootCmd.Flags().AddGoFlag(gf)
	})
}

func gatherTokens(flags *cmdFlags) map[string]string {
	tokens := make(map[string]string)
	for instance, token := range flags.ghOAuthTokens {
		tokens[instance] = token
	}
	if len(flags.ghOAuthToken) > 0 {
		if _, ok := tokens["github.com"]; ok {
			klog.Warning("github.com token is overridden by the provided token with `--github-oauth-token flag` ")
		}
		tokens["github.com"] = flags.ghOAuthToken
	}

	return tokens
}
