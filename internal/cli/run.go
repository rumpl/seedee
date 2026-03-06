package cli

import (
	"context"
	"fmt"

	"github.com/rumpl/seedee/internal/core"
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
			cfg, err := loadPipelineConfig(configFile)
			if err != nil {
				return err
			}
			pipeline, err := core.NewPipelineFromConfig(cfg)
			if err != nil {
				return fmt.Errorf("creating pipeline: %w", err)
			}
			if verbose {
				fmt.Fprintf(cmd.ErrOrStderr(), "Pipeline: %s (%d jobs)\n", pipeline.Name, len(pipeline.Jobs))
				for _, job := range pipeline.Jobs {
					fmt.Fprintf(cmd.ErrOrStderr(), "  Job: %s (%s, %d steps)\n", job.Name, job.Image, len(job.Steps))
				}
			}
			if serverAddr != "" {
				return runRemote(cmd.Context(), pipeline, serverAddr)
			}
			return runLocal(cmd.Context(), pipeline)
		},
	}
}

func runLocal(ctx context.Context, pipeline *core.Pipeline) error {
	return fmt.Errorf("local execution not yet implemented")
}

func runRemote(ctx context.Context, pipeline *core.Pipeline, addr string) error {
	return fmt.Errorf("remote execution not yet implemented")
}
