// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gardener/docforge/pkg/gentoc"
	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

type genTocFlags struct {
	manifestURL    string
	output         string
	githubOAuthMap map[string]string
	cacheDir       string
	stripRoot      bool
}

// NewGenTocCmd returns the gen-toc subcommand.
func NewGenTocCmd(ctx context.Context) *cobra.Command {
	f := &genTocFlags{}

	cmd := &cobra.Command{
		Use:   "gen-toc",
		Short: "Generate a navigation YAML from a Docforge manifest",
		Long: `Reads a Docforge manifest and derives a navigation structure from it.

The generated YAML reflects the dir / file / fileTree hierarchy defined in the
manifest. It can be used as input for VitePress, SAP portal (toc.yaml), MkDocs,
or any other site generator that consumes a navigation file.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			return runGenToc(ctx, f)
		},
	}

	userHomeDir, _ := os.UserHomeDir()
	defaultCacheDir := filepath.Join(userHomeDir, DocforgeHomeDir)

	cmd.Flags().StringVarP(&f.manifestURL, "manifest", "f", "", "Manifest URL (required).")
	cmd.Flags().StringVarP(&f.output, "output", "o", "", "Output file path. Prints to stdout when omitted.")
	cmd.Flags().StringToStringVar(&f.githubOAuthMap, "github-oauth-env-map", map[string]string{},
		"Map between GitHub instances and ENV variable names that hold access tokens.")
	cmd.Flags().StringVar(&f.cacheDir, "cache-dir", defaultCacheDir, "Cache directory for repository HTTP cache.")
	cmd.Flags().BoolVar(&f.stripRoot, "strip-root", false, "Strip the top-level directory prefix from all filenames.")

	if err := cmd.MarkFlagRequired("manifest"); err != nil {
		klog.Error(err)
	}

	return cmd
}

func runGenToc(ctx context.Context, f *genTocFlags) error {
	initOpts := repositoryhost.InitOptions{
		EnvCredentials: f.githubOAuthMap,
		CacheHomeDir:   f.cacheDir,
	}

	rhs, err := initRepositoryHosts(ctx, initOpts)
	if err != nil {
		return fmt.Errorf("failed to initialise repository hosts: %w", err)
	}

	rhRegistry := registry.NewRegistry(rhs...)

	nodes, err := manifest.ResolveManifest(f.manifestURL, rhRegistry)
	if err != nil {
		return fmt.Errorf("failed to resolve manifest %s: %w", f.manifestURL, err)
	}

	nav := gentoc.FromNodes(nodes, f.stripRoot)
	out, err := gentoc.Marshal(nav)
	if err != nil {
		return fmt.Errorf("failed to marshal navigation YAML: %w", err)
	}

	if f.output == "" {
		fmt.Print(string(out))
		return nil
	}

	if err := os.WriteFile(f.output, out, 0644); err != nil {
		return fmt.Errorf("failed to write output file %s: %w", f.output, err)
	}
	klog.Infof("Navigation written to %s\n", f.output)
	return nil
}
