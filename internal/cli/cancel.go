// Package cli implements the command-line interface for seedee.
package cli

import (
	"fmt"

	"connectrpc.com/connect"
	seedeev1 "github.com/rumpl/seedee/gen/seedee/v1"
	"github.com/spf13/cobra"
)

func newCancelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cancel [pipeline-id]",
		Short: "Cancel a running pipeline",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if serverAddr == "" {
				return fmt.Errorf("--server flag is required for cancel command")
			}

			client := newCIClient(serverAddr)
			resp, err := client.CancelPipeline(cmd.Context(), connect.NewRequest(&seedeev1.CancelPipelineRequest{
				PipelineId: args[0],
			}))
			if err != nil {
				return fmt.Errorf("canceling pipeline: %w", err)
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), resp.Msg.GetMessage())
			return nil
		},
	}
}
