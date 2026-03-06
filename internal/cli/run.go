package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"connectrpc.com/connect"

	seedeev1 "github.com/rumpl/seedee/gen/seedee/v1"
	"github.com/rumpl/seedee/internal/core"
	"github.com/rumpl/seedee/internal/runner/docker"
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
	// 1. Create Docker client
	dockerClient, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("connecting to Docker: %w\n\nMake sure Docker is running:\n  docker info", err)
	}
	defer dockerClient.Close()

	// 2. Verify Docker is reachable
	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()

	if err := dockerClient.Ping(pingCtx); err != nil {
		return fmt.Errorf("Docker is not reachable: %w\n\nMake sure Docker daemon is running:\n  docker info\n\nOr run against a remote server:\n  seedee run --server <addr>", err)
	}

	// 3. Create Docker runner with current working directory as source
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	runner := docker.NewDockerRunnerWithConfig(dockerClient, docker.DockerRunnerConfig{
		SourceDir: cwd,
	})

	// 4. Create log writer for terminal output
	logWriter := &terminalLogWriter{
		out:     os.Stdout,
		errOut:  os.Stderr,
		verbose: verbose,
	}

	// 5. Create and run engine
	engine := &core.Engine{
		Runner:    runner,
		LogWriter: logWriter,
	}

	fmt.Fprintf(os.Stdout, "▶ Pipeline %q started\n", pipeline.Name)
	for _, job := range pipeline.Jobs {
		fmt.Fprintf(os.Stdout, "  ▶ Job %q (%s, %d steps)\n", job.Name, job.Image, len(job.Steps))
	}

	result, err := engine.Execute(ctx, pipeline)
	if err != nil {
		return fmt.Errorf("pipeline execution failed: %w", err)
	}

	// 6. Print summary
	printPipelineSummary(os.Stdout, result)

	// 7. Exit with appropriate code
	if result.Status != core.StatusSuccess {
		os.Exit(1)
	}

	return nil
}

func runRemote(ctx context.Context, pipeline *core.Pipeline, addr string) error {
	// 1. Create Connect client
	client := newCIClient(addr)

	// 2. Convert pipeline to protobuf request
	req := pipelineToProtoRequest(pipeline)

	// 3. Call RunPipeline (server-streaming)
	stream, err := client.RunPipeline(ctx, connect.NewRequest(req))
	if err != nil {
		return wrapConnectError(err, addr)
	}

	// 4. Create terminal event handler
	handler := &terminalEventHandler{
		out:     os.Stdout,
		errOut:  os.Stderr,
		verbose: verbose,
	}

	// 5. Read events from stream and display them
	var lastStatus seedeev1.Status
	var pipelineID string

	// If context is canceled while streaming, try to explicitly cancel
	// the remote pipeline before disconnecting.
	cancelDone := make(chan struct{})
	go func() {
		defer close(cancelDone)
		<-ctx.Done()
		if pipelineID != "" {
			cancelCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, _ = client.CancelPipeline(cancelCtx, connect.NewRequest(&seedeev1.CancelPipelineRequest{
				PipelineId: pipelineID,
			}))
		}
	}()

	for stream.Receive() {
		event := stream.Msg()
		pipelineID = event.GetPipelineId()
		lastStatus = event.GetStatus()

		// Convert proto event to core event for the handler
		coreEvent := protoEventToCore(event)
		if err := handler.HandleEvent(coreEvent); err != nil {
			return fmt.Errorf("handling event: %w", err)
		}
	}

	if err := stream.Err(); err != nil {
		return wrapConnectError(err, addr)
	}

	// 6. Print summary
	fmt.Fprintf(os.Stdout, "\nPipeline %s completed: %s\n", pipelineID, formatProtoStatus(lastStatus))

	if lastStatus != seedeev1.Status_STATUS_SUCCESS {
		os.Exit(1)
	}

	return nil
}
