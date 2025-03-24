// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"flag"
	"os"
	"path/filepath"

	"github.com/gardener/docforge/cmd/alias"
	"github.com/gardener/docforge/cmd/docsy"
	"github.com/gardener/docforge/cmd/gendocs"
	"github.com/gardener/docforge/cmd/hugo"
	"github.com/gardener/docforge/cmd/markdown"
	"github.com/gardener/docforge/cmd/persona"
	"github.com/gardener/docforge/cmd/version"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
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

// options data structure with all the options for docforge
type options struct {
	Options                    `mapstructure:",squash"`
	hugo.Hugo                  `mapstructure:",squash"`
	docsy.Docsy                `mapstructure:",squash"`
	persona.Persona            `mapstructure:",squash"`
	markdown.Markdown          `mapstructure:",squash"`
	alias.Alias                `mapstructure:",squash"`
	repositoryhost.InitOptions `mapstructure:",squash"`
}

// NewCommand creates a new root command and propagates
// the context and cancel function to its Run callback closure
func NewCommand(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docforge",
		Short: "Forge a documentation bundle",
	}

	vip := configure(cmd)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return exec(ctx, vip)
	}

	version := version.NewVersionCmd()
	cmd.AddCommand(version)

	genCmdDocs := gendocs.NewGenCmdDocs()
	cmd.AddCommand(genCmdDocs)

	klog.InitFlags(nil)
	addFlags(cmd)

	return cmd
}

func configure(command *cobra.Command) *viper.Viper {
	//set delimiter to be ::
	vip := viper.NewWithOptions(viper.KeyDelimiter("::"))
	vip.SetDefault("chart::values", map[string]interface{}{
		"ingress": map[string]interface{}{
			"annotations": map[string]interface{}{
				"traefik.frontend.rule.type":                 "PathPrefix",
				"traefik.ingress.kubernetes.io/ssl-redirect": "true",
			},
		},
	})
	configureFlags(command, vip)
	configureConfigFile(vip)
	return vip
}

func configureConfigFile(vip *viper.Viper) {
	vip.AutomaticEnv()
	cfgFile := os.Getenv("DOCFORGE_CONFIG")
	if cfgFile == "" {
		userHomerDir, _ := os.UserHomeDir()
		cfgFile = filepath.Join(userHomerDir, DocforgeHomeDir, DefaultConfigFileName)
		if _, err := os.Lstat(cfgFile); os.IsNotExist(err) {
			// default configuration file doesn't exists -> nothing to configure
			return
		}
	}
	vip.AddConfigPath(filepath.Dir(cfgFile))
	vip.SetConfigName(filepath.Base(cfgFile))
	vip.SetConfigType("yaml")
	err := vip.ReadInConfig()
	if err != nil {
		klog.Warningf("Non-fatal error in loading configuration file %s. No configuration file will be used: %v\n", cfgFile, err)
	}
	klog.Infof("Configuration file %s will be used\n", cfgFile)
}

func addFlags(rootCmd *cobra.Command) {
	flag.CommandLine.VisitAll(func(gf *flag.Flag) {
		rootCmd.Flags().AddGoFlag(gf)
	})
}
