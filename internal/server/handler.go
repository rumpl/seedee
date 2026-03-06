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

	// 5. Create event handler that streams to client
	eventHandler := &streamEventHandler{
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
		Runner:       runner,
		EventHandler: eventHandler,
	}

	// Send events via the engine's EventHandler — no manual sends needed
	result, err := engine.Execute(runCtx, pipeline)
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("executing pipeline: %w", err))
	}

	h.logger.Info("pipeline completed",
		"id", pipeline.ID,
		"status", result.Status,
		"duration", result.Duration,
	)

	return nil
}

// GetPipelineStatus returns the current state of a running or completed pipeline.
func (h *CIServiceHandler) GetPipelineStatus(
	_ context.Context,
	req *connect.Request[seedeev1.GetPipelineStatusRequest],
) (*connect.Response[seedeev1.GetPipelineStatusResponse], error) {
	if req.Msg.PipelineId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("pipeline_id is required"))
	}

	h.mu.RLock()
	run, ok := h.pipelines[req.Msg.PipelineId]
	h.mu.RUnlock()

	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("pipeline %q not found", req.Msg.PipelineId))
	}

	resp := PipelineStatusToProto(run.pipeline)
	return connect.NewResponse(resp), nil
}

// CancelPipeline requests cancellation of a running pipeline.
func (h *CIServiceHandler) CancelPipeline(
	_ context.Context,
	req *connect.Request[seedeev1.CancelPipelineRequest],
) (*connect.Response[seedeev1.CancelPipelineResponse], error) {
	if req.Msg.PipelineId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("pipeline_id is required"))
	}

	h.mu.RLock()
	run, ok := h.pipelines[req.Msg.PipelineId]
	h.mu.RUnlock()

	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("pipeline %q not found", req.Msg.PipelineId))
	}

	// Check if already finished
	select {
	case <-run.done:
		return connect.NewResponse(&seedeev1.CancelPipelineResponse{
			Canceled: false,
			Message:  "pipeline already completed",
		}), nil
	default:
	}

	// Cancel the pipeline's context
	run.cancel()

	return connect.NewResponse(&seedeev1.CancelPipelineResponse{
		Canceled: true,
		Message:  fmt.Sprintf("pipeline %q cancellation requested", req.Msg.PipelineId),
	}), nil
}

// PruneOldRuns removes completed pipeline runs older than maxAge.
func (h *CIServiceHandler) PruneOldRuns(maxAge time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for id, run := range h.pipelines {
		if !run.pipeline.EndedAt.IsZero() && run.pipeline.EndedAt.Before(cutoff) {
			delete(h.pipelines, id)
		}
	}
}

// streamEventHandler implements core.EventHandler and routes events to a
// ConnectRPC server stream as RunPipelineEvent messages.
type streamEventHandler struct {
	stream     *connect.ServerStream[seedeev1.RunPipelineEvent]
	pipelineID string
	mu         sync.Mutex // protect concurrent stream writes
}

func (h *streamEventHandler) HandleEvent(event core.Event) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	protoEvent := &seedeev1.RunPipelineEvent{
		PipelineId: h.pipelineID,
		Timestamp:  timestamppb.New(event.Timestamp),
		JobName:    event.JobName,
		StepName:   event.StepName,
		LogData:    event.LogData,
		IsStderr:   event.IsStderr,
		Status:     StatusToProto(event.Status),
		ExitCode:   int32(event.ExitCode),
		Error:      event.Error,
	}

	if event.Duration > 0 {
		protoEvent.Duration = durationpb.New(event.Duration)
	}

	switch event.Type {
	case core.EventPipelineStarted:
		protoEvent.Type = seedeev1.EventType_EVENT_TYPE_PIPELINE_STARTED
	case core.EventPipelineFinished:
		protoEvent.Type = seedeev1.EventType_EVENT_TYPE_PIPELINE_FINISHED
	case core.EventJobStarted:
		protoEvent.Type = seedeev1.EventType_EVENT_TYPE_JOB_STARTED
	case core.EventJobFinished:
		protoEvent.Type = seedeev1.EventType_EVENT_TYPE_JOB_FINISHED
	case core.EventJobSkipped:
		protoEvent.Type = seedeev1.EventType_EVENT_TYPE_JOB_SKIPPED
	case core.EventStepStarted:
		protoEvent.Type = seedeev1.EventType_EVENT_TYPE_STEP_STARTED
	case core.EventStepFinished:
		protoEvent.Type = seedeev1.EventType_EVENT_TYPE_STEP_FINISHED
	case core.EventStepLog:
		protoEvent.Type = seedeev1.EventType_EVENT_TYPE_STEP_LOG
	}

	return h.stream.Send(protoEvent)
}
