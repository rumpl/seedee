package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version, GitCommit, and BuildDate are set at build time via ldflags.
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, _ []string) {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "seedee %s (commit: %s, built: %s)\n", Version, GitCommit, BuildDate)
		},
	}
}
