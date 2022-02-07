// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gardener/docforge/cmd/configuration"
	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
)

const (
	// DefaultConfigFileName default configuration filename under docforge home folder
	DefaultConfigFileName = "config"
	// DocforgeHomeDir defines the docforge home location
	DocforgeHomeDir = ".docforge"
)

type loadedConfiguration struct {
	baseConfig `mapstructure:",squash"`

	GhOAuthToken  string            `mapstructure:"github-oauth-token"`
	GhOAuthTokens map[string]string `mapstructure:"github-oauth-token-map"`
	//credentials loaded from config file
	Credidential  []configuration.Credential `mapstructure:"credidential"`
	LastNVersions map[string]string          `mapstructure:"lastNVersions"`
}

type baseConfig struct {
	DocumentWorkersCount         int               `mapstructure:"document-workers"`
	ValidationWorkersCount       int               `mapstructure:"validation-workers"`
	FailFast                     bool              `mapstructure:"fail-fast"`
	DestinationPath              string            `mapstructure:"destination"`
	DocumentationManifestPath    string            `mapstructure:"manifest"`
	ResourcesPath                string            `mapstructure:"resources-download-path"`
	ResourceDownloadWorkersCount int               `mapstructure:"download-workers"`
	RewriteEmbedded              bool              `mapstructure:"rewrite-embedded-to-raw"`
	Variables                    map[string]string `mapstructure:"variables"`
	GhInfoDestination            string            `mapstructure:"github-info-destination"`
	GhThrottling                 bool              `mapstructure:"github-throttling"`
	DryRun                       bool              `mapstructure:"dry-run"`
	Resolve                      bool              `mapstructure:"resolve"`
	Hugo                         bool              `mapstructure:"hugo"`
	HugoPrettyUrls               bool              `mapstructure:"hugo-pretty-urls"`
	FlagsHugoSectionFiles        []string          `mapstructure:"hugo-section-files"`
	HugoBaseURL                  string            `mapstructure:"hugo-base-url"`
	UseGit                       bool              `mapstructure:"use-git"`
	CacheHomeDir                 string            `mapstructure:"cache-dir"`

	ResourceMappings map[string]string `mapstructure:"resourceMappings"`
	DefaultBranches  map[string]string `mapstructure:"defaultBranches"`
}

// Options data structure with all the options for docforge
type Options struct {
	baseConfig

	Hugo          *configuration.Hugo
	Credentials   []*configuration.Credential
	LastNVersions map[string]int
}

var vip *viper.Viper

// NewCommand creates a new root command and propagates
// the context and cancel function to its Run callback closure
func NewCommand(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docforge",
		Short: "Forge a documentation bundle",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			var (
				doc     *api.Documentation
				rhs     []resourcehandlers.ResourceHandler
				err     error
				options *Options
			)

			options, err = NewOptions()
			if err != nil {
				return err
			}
			if rhs, err = initResourceHandlers(ctx, options); err != nil {
				return err
			}
			// TODO: HOW API CAN CONSUME CONFIGURATION
			api.SetFlagsVariables(options.Variables)
			api.SetDefaultBranches(options.DefaultBranches)
			api.SetNVersions(options.LastNVersions)
			if doc, err = manifest(ctx, options.DocumentationManifestPath, rhs); err != nil {
				return err
			}
			reactor, err := NewReactor(options, rhs)
			if err != nil {
				return err
			}
			if err = reactor.Run(ctx, doc, options.DryRun); err != nil {
				return err
			}
			return nil
		},
	}

	Configure(cmd)

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
func Configure(command *cobra.Command) {
	//set delimiter to be ::
	vip = viper.NewWithOptions(viper.KeyDelimiter("::"))
	vip.SetDefault("chart::values", map[string]interface{}{
		"ingress": map[string]interface{}{
			"annotations": map[string]interface{}{
				"traefik.frontend.rule.type":                 "PathPrefix",
				"traefik.ingress.kubernetes.io/ssl-redirect": "true",
			},
		},
	})

	configureFlags(command)
	configureConfigFile()
}

func configureFlags(command *cobra.Command) {
	command.Flags().StringP("destination", "d", "",
		"Destination path.")
	command.MarkFlagRequired("destination")
	vip.BindPFlag("destination", command.Flags().Lookup("destination"))

	command.Flags().StringP("manifest", "f", "",
		"Manifest path.")
	command.MarkFlagRequired("manifest")
	vip.BindPFlag("manifest", command.Flags().Lookup("manifest"))

	command.Flags().String("resources-download-path", "__resources",
		"Resources download path.")
	vip.BindPFlag("resources-download-path", command.Flags().Lookup("resources-download-path"))

	command.Flags().String("github-oauth-token", "",
		"GitHub personal token authorizing read access from GitHub.com repositories. For authorization credentials for multiple GitHub instances, see --github-oauth-token-map")
	vip.BindPFlag("github-oauth-token", command.Flags().Lookup("github-oauth-token"))

	command.Flags().StringToString("github-oauth-token-map", map[string]string{},
		"GitHub personal tokens authorizing read access from repositories per GitHub instance. Note that if the GitHub token is already provided by `github-oauth-token` it will be overridden by it.")
	vip.BindPFlag("github-oauth-token-map", command.Flags().Lookup("github-oauth-token-map"))

	command.Flags().Bool("github-throttling", false,
		"Enable throttling of requests to GitHub API. The throttling is adaptive and will slow down execution with the approaching rate limit. Use to improve continuity. Disable to maximise performance.")
	vip.BindPFlag("github-throttling", command.Flags().Lookup("github-throttling"))

	command.Flags().String("github-info-destination", "",
		"If specified, docforge will download also additional github info for the files from the documentation structure into this destination.")
	vip.BindPFlag("github-info-destination", command.Flags().Lookup("github-info-destination"))

	command.Flags().Bool("rewrite-embedded-to-raw", true,
		"Rewrites absolute link destinations for embedded resources (images) to reference embeddable media (e.g. for GitHub - reference to a 'raw' version of an image).")
	vip.BindPFlag("rewrite-embedded-to-raw", command.Flags().Lookup("rewrite-embedded-to-raw"))

	command.Flags().StringToString("variables", map[string]string{},
		"Variables applied to parameterized (using Go template) manifest.")
	vip.BindPFlag("variables", command.Flags().Lookup("variables"))

	command.Flags().Bool("fail-fast", false,
		"Fail-fast vs fault tolerant operation.")
	vip.BindPFlag("fail-fast", command.Flags().Lookup("fail-fast"))

	command.Flags().Bool("dry-run", false,
		"Runs the command end-to-end but instead of writing files, it will output the projected file/folder hierarchy to the standard output and statistics for the processing of each file.")
	vip.BindPFlag("dry-run", command.Flags().Lookup("dry-run"))

	command.Flags().Bool("resolve", false,
		"Resolves the documentation structure and prints it to the standard output. The resolution expands nodeSelector constructs into node hierarchies.")
	vip.BindPFlag("resolve", command.Flags().Lookup("resolve"))

	command.Flags().Int("document-workers", 25,
		"Number of parallel workers for document processing.")
	vip.BindPFlag("document-workers", command.Flags().Lookup("document-workers"))

	command.Flags().Int("validation-workers", 50,
		"Number of parallel workers to validate the markdown links")
	vip.BindPFlag("validation-workers", command.Flags().Lookup("validation-workers"))

	command.Flags().Int("download-workers", 10,
		"Number of workers downloading document resources in parallel.")
	vip.BindPFlag("download-workers", command.Flags().Lookup("download-workers"))

	command.Flags().Bool("hugo", false,
		"Build documentation bundle for hugo.")
	vip.BindPFlag("hugo", command.Flags().Lookup("hugo"))

	command.Flags().Bool("hugo-pretty-urls", true,
		"Build documentation bundle for hugo with pretty URLs (./sample.md -> ../sample). Only useful with --hugo=true")
	vip.BindPFlag("hugo-pretty-urls", command.Flags().Lookup("hugo-pretty-urls"))

	command.Flags().String("hugo-base-url", "",
		"Rewrites the relative links of documentation files to root-relative where possible.")
	vip.BindPFlag("hugo-base-url", command.Flags().Lookup("hugo-base-url"))

	command.Flags().Bool("use-git", false,
		"Use Git for replication")
	vip.BindPFlag("use-git", command.Flags().Lookup("use-git"))

	command.Flags().StringSlice("hugo-section-files", []string{"readme.md", "readme", "read.me", "index.md", "index"},
		"When building a Hugo-compliant documentation bundle, files with filename matching one form this list (in that order) will be renamed to _index.md. Only useful with --hugo=true")
	vip.BindPFlag("hugo-section-files", command.Flags().Lookup("hugo-section-files"))

	userHomeDir, err := os.UserHomeDir()
	if err == nil {
		// default value $HOME/.docforge/cache
		command.Flags().String("cache-dir", filepath.Join(userHomeDir, DocforgeHomeDir),
			"Cache directory, used for repository cache.")
	} else {
		command.Flags().String("cache-dir", "",
			"Cache directory, used for repository cache.")
	}
	vip.BindPFlag("cache-dir", command.Flags().Lookup("cache-dir"))

	command.Flags().StringToString("lastNVersions", map[string]string{},
		"Specify default number of versions and per uri that will be supported")
	vip.BindPFlag("lastNVersions", command.Flags().Lookup("lastNVersions"))

	command.Flags().StringToString("defaultBranches", map[string]string{},
		"Specify default main branch and per uri")
	vip.BindPFlag("defaultBranches", command.Flags().Lookup("defaultBranches"))

}

func configureConfigFile() {
	userHomerDir, err := os.UserHomeDir()
	if err != nil {
		klog.Warningf("Non-fatal error in loading configuration: %s. No configuration file will be used", err.Error())
		return
	}
	vip.AddConfigPath(filepath.Join(userHomerDir, DocforgeHomeDir))
	vip.SetConfigName(DefaultConfigFileName)
	vip.SetConfigType("yaml")
	err = vip.ReadInConfig()
	if err != nil {
		klog.Warningf("Non-fatal error in loading configuration: %s. No configuration file will be used", err.Error())
	} else {
		klog.Warningf("Config file %s with path %s will be used", DefaultConfigFileName, filepath.Join(userHomerDir, DocforgeHomeDir))
	}
}

// NewOptions creates a configuration.Options object from flags and configuration file
// flags overwrites values from configuration file
func NewOptions() (*Options, error) {
	loadedOptions := &loadedConfiguration{}
	err := vip.Unmarshal(loadedOptions)
	if err != nil {
		return nil, err
	}

	hugo := configuration.Hugo{
		Enabled:        vip.GetBool("hugo"),
		PrettyURLs:     vip.GetBool("hugo-pretty-urls"),
		BaseURL:        vip.GetString("hugo-base-url"),
		IndexFileNames: vip.GetStringSlice("hugo-section-files"),
	}

	interfaceMap := vip.GetStringMapString("lastNVersions")
	converted := make(map[string]int)
	var toInt int
	for key, value := range interfaceMap {
		toInt, err = strconv.Atoi(value)
		if err == nil {
			converted[key] = toInt
		} else {
			klog.Warningf(`for key %s in lastNVersions provided %s while expecting a int type. Skipping it`, key, value)
		}
	}

	return &Options{
		baseConfig:    loadedOptions.baseConfig,
		Credentials:   gatherCredentials(),
		LastNVersions: converted,
		Hugo:          &hugo,
	}, nil
}

// AddFlags adds go flags to rootCmd
func AddFlags(rootCmd *cobra.Command) {
	flag.CommandLine.VisitAll(func(gf *flag.Flag) {
		rootCmd.Flags().AddGoFlag(gf)
	})
}

func gatherCredentials() []*configuration.Credential {
	configCredentials := []configuration.Credential{}
	err := vip.UnmarshalKey("credidential", &configCredentials)
	if err != nil {
		klog.Warningf("error in unmarshaling credidentails from config: %s", err.Error())
	}
	ghOAuthTokens := vip.GetStringMapString("github-oauth-token-map")
	ghOAuthToken := vip.GetString("github-oauth-token")

	credentialsByHost := make(map[string]*configuration.Credential)

	// when no token specified consider the configuration incorrect
	for _, cred := range configCredentials {
		if cred.OAuthToken == "" {
			klog.Warningf("configuration is considered incorrect because of missing oauth token for host: %s\n", cred.Host)
			continue
		}
		credentialsByHost[cred.Host] = &cred
	}
	// tokens provided by flags will override the config
	for instance, credentials := range ghOAuthTokens {
		var username string
		token := credentials
		// for cases where user credentials are in the format `username:token`
		usernameAndToken := strings.Split(credentials, ":")
		if len(usernameAndToken) == 2 {
			username = usernameAndToken[0]
			token = usernameAndToken[1]
		}
		if _, ok := credentialsByHost[instance]; ok {
			klog.Warningf("%s token is overridden by the provided token with `--github-oauth-token-map flag`\n", instance)
		}
		credentialsByHost[instance] = &configuration.Credential{
			Host:       instance,
			Username:   username,
			OAuthToken: token,
		}
	}

	if len(ghOAuthToken) > 0 {
		//provided ghOAuthToken may override credentialsByHost. This is the default logic
		var username string
		token := ghOAuthToken
		if _, ok := credentialsByHost["github.com"]; ok {
			klog.Warning("github.com token is overridden by the provided token with `--github-oauth-token flag`\n")
		}
		usernameAndToken := strings.Split(ghOAuthToken, ":")
		if len(usernameAndToken) == 2 {
			username = usernameAndToken[0]
			token = usernameAndToken[1]
		}

		credentialsByHost["github.com"] = &configuration.Credential{
			Host:       "github.com",
			Username:   username,
			OAuthToken: token,
		}
	} else {
		if _, ok := credentialsByHost["github.com"]; !ok {
			klog.Infof("using unauthenticated github access`\n")
			//credentialByHost at github.com is not set and should be set to empty string
			credentialsByHost["github.com"] = &configuration.Credential{
				Host:       "github.com",
				Username:   "",
				OAuthToken: "",
			}
		}
	}
	var credentials = make([]*configuration.Credential, 0, len(credentialsByHost))
	for _, cred := range credentialsByHost {
		credentials = append(credentials, cred)
	}
	return credentials
}
