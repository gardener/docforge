// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gendocs

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"k8s.io/klog/v2"
)

const (
	genDocsMarkdown genDocsFormat = iota
	genDocsManPages
)

type genDocsCmdFlags struct {
	format      string
	destination string
}

type genDocsFormat int

func newGenDocsFormat(formatString string) (genDocsFormat, error) {
	switch formatString {
	case "md":
		return genDocsMarkdown, nil
	case "man":
		return genDocsManPages, nil
	}
	return 0, fmt.Errorf("unknown format '%s'. Must be one of %v", formatString, []string{"md", "man"})
}

// NewGenCmdDocs generates commands reference documentation
// in Markdown format
func NewGenCmdDocs() *cobra.Command {
	flags := &genDocsCmdFlags{}
	command := &cobra.Command{
		Use:   "gen-cmd-docs",
		Short: "Generates commands reference documentation",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cmd.Root()
			c.DisableAutoGenTag = true
			destination := filepath.Clean(flags.destination)
			if _, err := os.Stat(destination); err != nil {
				if os.IsNotExist(err) {
					if err := os.MkdirAll(destination, os.ModePerm); err != nil {
						klog.Error(err)
						return err
					}
				} else {
					klog.Error(err)
					return err
				}
			}
			format, err := newGenDocsFormat(flags.format)
			if err != nil {
				klog.Error(err)
				return err
			}
			switch format {
			case genDocsManPages:
				{
					header := &doc.GenManHeader{
						Title:   "DOCFORGE",
						Manual:  "Docforge Command Reference",
						Section: "1",
					}
					if err := doc.GenManTree(c, header, destination); err != nil {
						klog.Fatal(err)
					}
				}
			default:
				{
					if err := doc.GenMarkdownTree(c, destination); err != nil {
						klog.Fatal(err)
					}
				}
			}
			return nil
		},
	}
	command.Flags().StringVarP(&flags.format, "format", "f", "md",
		"Specifies the generated documentation format. Must be one of: `md` (for markdown) or `man` (for man pages).")
	command.Flags().StringVarP(&flags.destination, "destination", "d", "",
		"Path to directory where the documentation will be generated. If it does not exist, it will be created. Required flag.")
	command.MarkFlagRequired("destination")
	return command
}
