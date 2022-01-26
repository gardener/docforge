// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"flag"
	"k8s.io/utils/pointer"
	"os"
	"path/filepath"
	"strings"

	"github.com/gardener/docforge/cmd/configuration"
	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

type cmdFlags struct {
	documentWorkersCount         int
	validationWorkersCount       int
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
	hugo                         bool
	hugoPrettyUrls               bool
	hugoSectionFiles             []string
	hugoBaseURL                  string
	useGit                       bool
	cacheHomeDir                 string
	lastNVersions                map[string]int
	mainBranch                   map[string]string
}

// NewCommand creates a new root command and propagates
// the context and cancel function to its Run callback closure
func NewCommand(ctx context.Context) *cobra.Command {
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

			options := NewOptions(flags, new(configuration.DefaultConfigurationLoader))
			if rhs, err = initResourceHandlers(ctx, flags, options); err != nil {
				return err
			}
			// TODO: HOW API CAN CONSUME CONFIGURATION
			api.SetFlagsVariables(flags.variables)
			api.SetDefaultBranches(flags.mainBranch, options.DefaultBranches)
			api.SetNVersions(flags.lastNVersions, options.LastNVersions)
			if doc, err = manifest(ctx, flags.documentationManifestPath, rhs); err != nil {
				return err
			}
			reactor, err := NewReactor(flags, options, rhs)
			if err != nil {
				return err
			}
			if err = reactor.Run(ctx, doc, flags.dryRun); err != nil {
				return err
			}
			return nil
		},
	}

	flags.Configure(cmd)

	version := NewVersionCmd()
	cmd.AddCommand(version)

	completion := newCompletionCmd()
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
		"Rewrites absolute link destinations for embedded resources (images) to reference embeddable media (e.g. for GitHub - reference to a 'raw' version of an image).")
	command.Flags().StringToStringVar(&flags.variables, "variables", map[string]string{},
		"Variables applied to parameterized (using Go template) manifest.")
	command.Flags().BoolVar(&flags.failFast, "fail-fast", false,
		"Fail-fast vs fault tolerant operation.")
	command.Flags().BoolVar(&flags.dryRun, "dry-run", false,
		"Runs the command end-to-end but instead of writing files, it will output the projected file/folder hierarchy to the standard output and statistics for the processing of each file.")
	command.Flags().BoolVar(&flags.resolve, "resolve", false,
		"Resolves the documentation structure and prints it to the standard output. The resolution expands nodeSelector constructs into node hierarchies.")
	command.Flags().IntVar(&flags.documentWorkersCount, "document-workers", 25,
		"Number of parallel workers for document processing.")
	command.Flags().IntVar(&flags.validationWorkersCount, "validation-workers", 50,
		"Number of parallel workers to validate the markdown links")
	command.Flags().IntVar(&flags.resourceDownloadWorkersCount, "download-workers", 10,
		"Number of workers downloading document resources in parallel.")
	command.Flags().BoolVar(&flags.hugo, "hugo", false,
		"Build documentation bundle for hugo.")
	command.Flags().BoolVar(&flags.hugoPrettyUrls, "hugo-pretty-urls", true,
		"Build documentation bundle for hugo with pretty URLs (./sample.md -> ../sample). Only useful with --hugo=true")
	command.Flags().StringVar(&flags.hugoBaseURL, "hugo-base-url", "",
		"Rewrites the relative links of documentation files to root-relative where possible.")
	command.Flags().BoolVar(&flags.useGit, "use-git", false,
		"Use Git for replication")
	command.Flags().StringSliceVar(&flags.hugoSectionFiles, "hugo-section-files", []string{"readme.md", "readme", "read.me", "index.md", "index"},
		"When building a Hugo-compliant documentation bundle, files with filename matching one form this list (in that order) will be renamed to _index.md. Only useful with --hugo=true")
	command.Flags().StringVar(&flags.cacheHomeDir, "cache-dir", "",
		"Cache directory, used for repository cache.")
	command.Flags().StringToIntVar(&flags.lastNVersions, "versions", map[string]int{},
		"Specify default number of versions and per uri that will be supported")
	command.Flags().StringToStringVar(&flags.mainBranch, "main-branches", map[string]string{},
		"Specify default main branch and per uri")
}

// AddFlags adds go flags to rootCmd
func AddFlags(rootCmd *cobra.Command) {
	flag.CommandLine.VisitAll(func(gf *flag.Flag) {
		rootCmd.Flags().AddGoFlag(gf)
	})
}

// NewOptions creates a configuration.Options object from flags and configuration file
// flags overwrites values from configuration file
func NewOptions(f *cmdFlags, c configuration.Loader) *Options {
	config, err := c.Load()
	if err != nil {
		panic(err)
	}
	var (
		defaultBranches = config.DefaultBranches
		lastNVersions   = config.LastNVersions
	)
	if f.mainBranch != nil {
		if defaultBranches == nil {
			defaultBranches = f.mainBranch
		} else {
			for k, v := range f.mainBranch {
				defaultBranches[k] = v
			}
		}
	}
	if f.lastNVersions != nil {
		if lastNVersions == nil {
			lastNVersions = f.lastNVersions
		} else {
			for k, v := range f.lastNVersions {
				lastNVersions[k] = v
			}
		}
	}
	return &Options{
		Credentials:     gatherCredentials(f, config),
		Hugo:            hugoOptions(f, config),
		HomeDir:         cacheHomeDir(f, config),
		LocalMappings:   config.ResourceMappings,
		DefaultBranches: defaultBranches,
		LastNVersions:   lastNVersions,
	}
}

func gatherCredentials(flags *cmdFlags, config *configuration.Config) []*configuration.Credentials {
	credentialsByHost := make(map[string]*configuration.Credentials)
	if config != nil {
		// when no token specified consider the configuration incorrect
		for _, cred := range config.Credentials {
			if cred.OAuthToken == nil {
				klog.Warningf("configuration is considered incorrect because of missing oauth token for host: %s\n", cred.Host)
				continue
			}
			credentialsByHost[cred.Host] = cred
		}
	}

	// tokens provided by flags will override the config
	for instance, credentials := range flags.ghOAuthTokens {
		var username string
		// for cases where user credentials are in the format `username:token`
		usernameAndToken := strings.Split(credentials, ":")
		if len(usernameAndToken) == 2 {
			username = usernameAndToken[0]
			credentials = usernameAndToken[1]
		}
		if _, ok := credentialsByHost[instance]; ok {
			klog.Warningf("%s token is overridden by the provided token with `--github-oauth-token-map flag`\n", instance)
		}
		credentialsByHost[instance] = &configuration.Credentials{
			Host:       instance,
			Username:   &username,
			OAuthToken: &credentials,
		}
	}

	if len(flags.ghOAuthToken) > 0 {
		//provided ghOAuthToken may override credentialsByHost. This is the default logic
		var username string
		token := flags.ghOAuthToken
		if _, ok := credentialsByHost["github.com"]; ok {
			klog.Warning("github.com token is overridden by the provided token with `--github-oauth-token flag`\n")
		}
		usernameAndToken := strings.Split(flags.ghOAuthToken, ":")
		if len(usernameAndToken) == 2 {
			username = usernameAndToken[0]
			token = usernameAndToken[1]
		}

		credentialsByHost["github.com"] = &configuration.Credentials{
			Host:       "github.com",
			Username:   &username,
			OAuthToken: &token,
		}
	} else {
		if _, ok := credentialsByHost["github.com"]; !ok {
			klog.Infof("using unauthenticated github access`\n")
			//credentialByHost at github.com is not set and should be set to empty string
			credentialsByHost["github.com"] = &configuration.Credentials{
				Host:       "github.com",
				Username:   pointer.StringPtr(""),
				OAuthToken: pointer.StringPtr(""),
			}
		}
	}
	var credentials = make([]*configuration.Credentials, 0, len(credentialsByHost))
	for _, cred := range credentialsByHost {
		credentials = append(credentials, cred)
	}
	return credentials
}

func cacheHomeDir(f *cmdFlags, config *configuration.Config) string {
	if cacheDir, found := os.LookupEnv("DOCFORGE_CACHE_DIR"); found {
		if cacheDir == "" {
			klog.Warning("DOCFORGE_CACHE_DIR is set to empty string. Docforge will use the current dir for the cache\n")
		}
		return cacheDir
	}

	if f.cacheHomeDir != "" {
		return f.cacheHomeDir
	}

	if config != nil && config.CacheHome != nil {
		return *config.CacheHome
	}

	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	// default value $HOME/.docforge/cache
	return filepath.Join(userHomeDir, configuration.DocforgeHomeDir)
}

func hugoOptions(f *cmdFlags, config *configuration.Config) *configuration.Hugo {
	hugo := config.Hugo
	if hugo == nil {
		hugo = &configuration.Hugo{}
	}
	hugo.Enabled = f.hugo
	if !hugo.Enabled {
		return hugo
	}
	// overwrites with flags
	hugo.PrettyURLs = f.hugoPrettyUrls

	if f.hugoBaseURL != "" {
		hugo.BaseURL = f.hugoBaseURL
	}
	if len(f.hugoSectionFiles) > 0 || len(hugo.IndexFileNames) > 0 {
		var files []string
		fileSet := make(map[string]struct{})
		// merge
		for _, fn := range f.hugoSectionFiles {
			if _, ok := fileSet[fn]; !ok {
				files = append(files, fn)
				fileSet[fn] = struct{}{}
			}
		}
		for _, fn := range f.hugoSectionFiles {
			if _, ok := fileSet[fn]; !ok {
				files = append(files, fn)
				fileSet[fn] = struct{}{}
			}
		}
		hugo.IndexFileNames = files
	}
	return hugo
}
