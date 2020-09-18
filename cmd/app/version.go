package app

import (
	"fmt"

	"github.com/gardener/docode/pkg/version"
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
			fmt.Println(version.Version)
		},
	}
}
