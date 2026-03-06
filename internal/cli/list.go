package cli

import (
	"fmt"
	"io"
	"text/tabwriter"

	"connectrpc.com/connect"
	seedeev1 "github.com/rumpl/seedee/gen/seedee/v1"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var statusFilter string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all pipeline runs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if serverAddr == "" {
				return fmt.Errorf("--server flag is required for list command")
			}

			req := &seedeev1.ListPipelinesRequest{}
			if statusFilter != "" {
				s, err := parseStatusFilter(statusFilter)
				if err != nil {
					return err
				}
				req.StatusFilter = s
			}

			client := newCIClient(serverAddr)
			resp, err := client.ListPipelines(cmd.Context(), connect.NewRequest(req))
			if err != nil {
				return fmt.Errorf("listing pipelines: %w", err)
			}

			printPipelineList(cmd.OutOrStdout(), resp.Msg.GetPipelines())
			return nil
		},
	}

	cmd.Flags().StringVar(&statusFilter, "status", "", "filter by status (pending, running, success, failed, skipped, canceled)")

	return cmd
}

func parseStatusFilter(s string) (seedeev1.Status, error) {
	switch s {
	case "pending":
		return seedeev1.Status_STATUS_PENDING, nil
	case "running":
		return seedeev1.Status_STATUS_RUNNING, nil
	case "success":
		return seedeev1.Status_STATUS_SUCCESS, nil
	case "failed":
		return seedeev1.Status_STATUS_FAILED, nil
	case "skipped":
		return seedeev1.Status_STATUS_SKIPPED, nil
	case "canceled":
		return seedeev1.Status_STATUS_CANCELED, nil
	default:
		return seedeev1.Status_STATUS_UNSPECIFIED, fmt.Errorf("unknown status %q: use pending, running, success, failed, skipped, or canceled", s)
	}
}

func printPipelineList(w io.Writer, pipelines []*seedeev1.PipelineSummary) {
	if len(pipelines) == 0 {
		_, _ = fmt.Fprintln(w, "No pipelines found.")
		return
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tNAME\tSTATUS\tDURATION")

	for _, p := range pipelines {
		dur := "-"
		if p.GetDuration() != nil {
			dur = p.GetDuration().AsDuration().String()
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s %s\t%s\n",
			p.GetPipelineId(),
			p.GetName(),
			statusIcon(p.GetStatus()),
			p.GetStatus(),
			dur,
		)
	}

	_ = tw.Flush()
}
