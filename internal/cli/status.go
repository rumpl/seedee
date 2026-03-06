package cli

import (
	"fmt"
	"io"

	"connectrpc.com/connect"
	seedeev1 "github.com/rumpl/seedee/gen/seedee/v1"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status [pipeline-id]",
		Short: "Get the status of a pipeline run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if serverAddr == "" {
				return fmt.Errorf("--server flag is required for status command")
			}

			client := newCIClient(serverAddr)
			resp, err := client.GetPipelineStatus(cmd.Context(), connect.NewRequest(&seedeev1.GetPipelineStatusRequest{
				PipelineId: args[0],
			}))
			if err != nil {
				return fmt.Errorf("getting pipeline status: %w", err)
			}

			printPipelineStatus(cmd.OutOrStdout(), resp.Msg)
			return nil
		},
	}
}

func statusIcon(s seedeev1.Status) string {
	switch s {
	case seedeev1.Status_STATUS_SUCCESS:
		return "✓"
	case seedeev1.Status_STATUS_FAILED:
		return "✗"
	case seedeev1.Status_STATUS_RUNNING:
		return "●"
	case seedeev1.Status_STATUS_PENDING:
		return "○"
	case seedeev1.Status_STATUS_SKIPPED:
		return "⊘"
	case seedeev1.Status_STATUS_CANCELED:
		return "⊗"
	default:
		return "?"
	}
}

func printPipelineStatus(w io.Writer, resp *seedeev1.GetPipelineStatusResponse) {
	_, _ = fmt.Fprintf(w, "%s Pipeline: %s (%s)\n", statusIcon(resp.GetStatus()), resp.GetPipelineName(), resp.GetPipelineId())
	_, _ = fmt.Fprintf(w, "  Status:   %s\n", resp.GetStatus())
	if resp.GetDuration() != nil {
		_, _ = fmt.Fprintf(w, "  Duration: %s\n", resp.GetDuration().AsDuration())
	}
	_, _ = fmt.Fprintln(w)

	for _, job := range resp.GetJobs() {
		_, _ = fmt.Fprintf(w, "  %s Job: %s\n", statusIcon(job.GetStatus()), job.GetName())
		if job.GetDuration() != nil {
			_, _ = fmt.Fprintf(w, "      Duration: %s\n", job.GetDuration().AsDuration())
		}
		for _, step := range job.GetSteps() {
			_, _ = fmt.Fprintf(w, "      %s Step: %s\n", statusIcon(step.GetStatus()), step.GetName())
			if step.GetDuration() != nil {
				_, _ = fmt.Fprintf(w, "          Duration: %s\n", step.GetDuration().AsDuration())
			}
		}
	}
}
