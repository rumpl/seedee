package server

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	seedeev1 "github.com/rumpl/seedee/gen/seedee/v1"
	"github.com/rumpl/seedee/internal/core"
	"github.com/rumpl/seedee/internal/runner/docker"
)

// CIServiceHandler implements the seedee.v1.CIService ConnectRPC service.
type CIServiceHandler struct {
	logger    *slog.Logger
	mu        sync.RWMutex
	pipelines map[string]*pipelineRun // ID -> run state
}

// pipelineRun tracks the state of a running or completed pipeline.
type pipelineRun struct {
	pipeline *core.Pipeline
	cancel   context.CancelFunc
	done     chan struct{}
}

// NewCIServiceHandler creates a new CIServiceHandler with the given logger.
func NewCIServiceHandler(logger *slog.Logger) *CIServiceHandler {
	return &CIServiceHandler{
		logger:    logger,
		pipelines: make(map[string]*pipelineRun),
	}
}

// RunPipeline receives a pipeline definition from the client, converts it to
// core types, creates an Engine with a Docker Runner, executes the pipeline,
// and streams events back to the client in real-time.
func (h *CIServiceHandler) RunPipeline(
	ctx context.Context,
	req *connect.Request[seedeev1.RunPipelineRequest],
	stream *connect.ServerStream[seedeev1.RunPipelineEvent],
) error {
	// 1. Validate request
	if req.Msg.Pipeline == nil {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("pipeline definition is required"))
	}

	// 2. Convert protobuf → core types
	cfg := PipelineDefFromProto(req.Msg.Pipeline)
	if err := cfg.Validate(); err != nil {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid pipeline: %w", err))
	}

	pipeline, err := core.NewPipelineFromConfig(cfg)
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("creating pipeline: %w", err))
	}

	// 3. Create cancellable context
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// 4. Register pipeline run
	run := &pipelineRun{
		pipeline: pipeline,
		cancel:   cancel,
		done:     make(chan struct{}),
	}
	h.mu.Lock()
	h.pipelines[pipeline.ID] = run
	h.mu.Unlock()
	defer func() {
		close(run.done)
	}()

	// 5. Create log writer that streams to client
	logWriter := &streamLogWriter{
		stream:     stream,
		pipelineID: pipeline.ID,
	}

	// 6. Create Docker runner
	dockerClient, err := docker.NewClient()
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("creating docker client: %w", err))
	}
	defer dockerClient.Close()

	runner := docker.NewDockerRunner(dockerClient)

	// 7. Create and run engine
	engine := &core.Engine{
		Runner:    runner,
		LogWriter: logWriter,
	}

	// Send pipeline started event
	if err := stream.Send(&seedeev1.RunPipelineEvent{
		PipelineId: pipeline.ID,
		Type:       seedeev1.EventType_EVENT_TYPE_PIPELINE_STARTED,
		Timestamp:  timestamppb.Now(),
		Status:     seedeev1.Status_STATUS_RUNNING,
	}); err != nil {
		return err
	}

	result, err := engine.Execute(runCtx, pipeline)
	if err != nil {
		// Send pipeline finished with failure
		_ = stream.Send(&seedeev1.RunPipelineEvent{
			PipelineId: pipeline.ID,
			Type:       seedeev1.EventType_EVENT_TYPE_PIPELINE_FINISHED,
			Timestamp:  timestamppb.Now(),
			Status:     StatusToProto(core.StatusFailed),
			Error:      err.Error(),
		})
		return connect.NewError(connect.CodeInternal, fmt.Errorf("executing pipeline: %w", err))
	}

	// Send pipeline finished event
	finishedEvent := &seedeev1.RunPipelineEvent{
		PipelineId: pipeline.ID,
		Type:       seedeev1.EventType_EVENT_TYPE_PIPELINE_FINISHED,
		Timestamp:  timestamppb.Now(),
		Status:     StatusToProto(result.Status),
		Duration:   durationpb.New(result.Duration),
	}
	if result.Error != nil {
		finishedEvent.Error = result.Error.Error()
	}
	if err := stream.Send(finishedEvent); err != nil {
		return err
	}

	h.logger.Info("pipeline completed",
		"id", pipeline.ID,
		"status", result.Status,
		"duration", result.Duration,
	)

	return nil
}

// GetPipelineStatus is not yet implemented.
func (h *CIServiceHandler) GetPipelineStatus(
	_ context.Context,
	_ *connect.Request[seedeev1.GetPipelineStatusRequest],
) (*connect.Response[seedeev1.GetPipelineStatusResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errGetPipelineStatusNotImplemented)
}

// CancelPipeline is not yet implemented.
func (h *CIServiceHandler) CancelPipeline(
	_ context.Context,
	_ *connect.Request[seedeev1.CancelPipelineRequest],
) (*connect.Response[seedeev1.CancelPipelineResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errCancelPipelineNotImplemented)
}

// streamLogWriter implements core.LogWriter and routes log output to a
// ConnectRPC server stream as RunPipelineEvent messages.
type streamLogWriter struct {
	stream     *connect.ServerStream[seedeev1.RunPipelineEvent]
	pipelineID string
	mu         sync.Mutex // protect concurrent stream writes
}

func (w *streamLogWriter) WriteLog(jobName, stepName string, data []byte, isStderr bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.stream.Send(&seedeev1.RunPipelineEvent{
		PipelineId: w.pipelineID,
		Type:       seedeev1.EventType_EVENT_TYPE_STEP_LOG,
		Timestamp:  timestamppb.New(time.Now()),
		JobName:    jobName,
		StepName:   stepName,
		LogData:    data,
		IsStderr:   isStderr,
	})
}
