package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Run a pipeline",
		Long: `Run the pipeline defined in .seedee.yml (or --config path).

Without --server, the pipeline runs locally using Docker.
With --server, the pipeline is sent to a remote seedee server.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("not yet implemented")
		},
	}
}
