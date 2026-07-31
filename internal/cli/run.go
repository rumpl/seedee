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
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadPipelineConfig(configFile)
			if err != nil {
				return fmt.Errorf("loading pipeline config: %w", err)
			}
			pipeline, err := core.NewPipelineFromConfig(cfg)
			if err != nil {
				return fmt.Errorf("creating pipeline: %w", err)
			}
			if verbose {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Pipeline: %s (%d jobs)\n", pipeline.Name, len(pipeline.Jobs))
				for _, job := range pipeline.Jobs {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "  Job: %s (%s, %d steps)\n", job.Name, job.Image, len(job.Steps))
				}
			}
			if serverAddr != "" {
				return runRemote(cmd.Context(), pipeline, serverAddr)
			}
			return runLocal(cmd.Context(), pipeline)
		},
	}
}

// summaryPrinter is implemented by both event handler types so runLocal and
// runRemote can call PrintSummary regardless of which handler is active.
type summaryPrinter interface {
	core.EventHandler
	PrintSummary(result *core.PipelineResult)
}

func newEventHandler(isTTY bool) summaryPrinter {
	if verbose {
		return &terminalEventHandler{
			out:    os.Stdout,
			errOut: os.Stderr,
			isTTY:  isTTY,
		}
	}
	return newProgressHandler(os.Stdout, os.Stderr, isTTY)
}

// startLiveRefresh starts the live elapsed-time redraw on a TTY progress
// handler and returns a function to stop it. It is a no-op for other handler
// types and in non-TTY mode.
func startLiveRefresh(h summaryPrinter) func() {
	ph, ok := h.(*progressEventHandler)
	if !ok {
		return func() {}
	}
	ph.startLiveRefresh(0)
	return ph.stopLiveRefresh
}

func runLocal(ctx context.Context, pipeline *core.Pipeline) error {
	// 1. Create Docker client
	dockerClient, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("connecting to docker: %w\n\nmake sure Docker is running:\n  docker info", err)
	}
	defer func() { _ = dockerClient.Close() }()

	// 2. Verify Docker is reachable
	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	pingErr := dockerClient.Ping(pingCtx)
	pingCancel()

	if pingErr != nil {
		return fmt.Errorf("docker is not reachable: %w\n\nmake sure Docker daemon is running:\n  docker info\n\nor run against a remote server:\n  seedee run --server <addr>", pingErr)
	}

	// 3. Create Docker runner with current working directory as source
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	runner := docker.NewRunnerWithConfig(dockerClient, docker.RunnerConfig{
		SourceDir: cwd,
	})

	// 4. Create event handler for terminal output
	handler := newEventHandler(isTerminal())
	stopRefresh := startLiveRefresh(handler)
	defer stopRefresh()

	// 5. Create and run engine
	engine := &core.Engine{
		Runner:       runner,
		EventHandler: handler,
	}

	result, err := engine.Execute(ctx, pipeline)
	if err != nil {
		return fmt.Errorf("pipeline execution failed: %w", err)
	}

	// 6. Print summary
	handler.PrintSummary(result)

	// 7. Exit with appropriate code
	if result.Status != core.StatusSuccess {
		return fmt.Errorf("pipeline failed with status: %s", result.Status)
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

	// 4. Create event handler
	handler := newEventHandler(isTerminal())
	stopRefresh := startLiveRefresh(handler)
	defer stopRefresh()

	// 5. Read events from stream and display them
	var lastStatus seedeev1.Status
	var pipelineID string

	// If context is canceled while streaming, try to explicitly cancel
	// the remote pipeline before disconnecting.
	cancelDone := make(chan struct{})
	streamDone := make(chan struct{})
	go func() {
		defer close(cancelDone)
		select {
		case <-ctx.Done():
			if pipelineID != "" {
				cancelCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
				defer cancel()
				_, _ = client.CancelPipeline(cancelCtx, connect.NewRequest(&seedeev1.CancelPipelineRequest{
					PipelineId: pipelineID,
				}))
			}
		case <-streamDone:
			// Stream finished normally; exit the goroutine.
		}
	}()

	for stream.Receive() {
		event := stream.Msg()
		pipelineID = event.GetPipelineId()
		lastStatus = event.GetStatus()

		// Convert proto event to core event for the handler
		coreEvent := protoEventToCore(event)
		if err := handler.HandleEvent(coreEvent); err != nil {
			close(streamDone)
			<-cancelDone
			return fmt.Errorf("handling event: %w", err)
		}
	}

	// Signal the cancel goroutine that the stream is done.
	close(streamDone)
	<-cancelDone

	if err := stream.Err(); err != nil {
		return wrapConnectError(err, addr)
	}

	// 6. Print summary
	_, _ = fmt.Fprintf(os.Stdout, "\nPipeline %s completed: %s\n", pipelineID, formatProtoStatus(lastStatus))

	if lastStatus != seedeev1.Status_STATUS_SUCCESS {
		return fmt.Errorf("pipeline failed with status: %s", formatProtoStatus(lastStatus))
	}

	return nil
}
