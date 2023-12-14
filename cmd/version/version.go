// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package version

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewVersionCmd creates a version command printing
// the binary version as reported by the pkg/version/Version
// variable
func NewVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(Version)
		},
	}
}

// Version is a global variable which is set during compile time via -ld-flags in the `go build` process.
// It stores the version of the Gardener and has either the form <X> or <X.Y>, where <X> denominates
// the current 'major' version, and <Y> (if present) denominates the current 'hotfix' version.
var Version = "binary was not built properly"
