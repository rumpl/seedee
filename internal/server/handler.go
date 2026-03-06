package server

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"connectrpc.com/connect"

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

	// 5. Create event handler that streams events to the client
	eventHandler := &streamEventHandler{
		stream: stream,
	}

	// 6. Create Docker runner
	dockerClient, err := docker.NewClient()
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("creating docker client: %w", err))
	}
	defer func() { _ = dockerClient.Close() }()

	runner := docker.NewRunner(dockerClient)

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

// ListPipelines returns summaries of all known pipeline runs, optionally
// filtered by status.
func (h *CIServiceHandler) ListPipelines(
	_ context.Context,
	req *connect.Request[seedeev1.ListPipelinesRequest],
) (*connect.Response[seedeev1.ListPipelinesResponse], error) {
	filter := req.Msg.GetStatusFilter()

	h.mu.RLock()
	defer h.mu.RUnlock()

	summaries := make([]*seedeev1.PipelineSummary, 0, len(h.pipelines))
	for _, run := range h.pipelines {
		p := run.pipeline
		protoStatus := StatusToProto(p.Status)

		if filter != seedeev1.Status_STATUS_UNSPECIFIED && protoStatus != filter {
			continue
		}

		summaries = append(summaries, PipelineSummaryToProto(p))
	}

	return connect.NewResponse(&seedeev1.ListPipelinesResponse{
		Pipelines: summaries,
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
	stream *connect.ServerStream[seedeev1.RunPipelineEvent]
	mu     sync.Mutex // protect concurrent stream writes
}

// HandleEvent converts a core.Event to protobuf and sends it immediately
// on the gRPC stream. No buffering — each event is flushed as a separate
// HTTP/2 data frame.
func (h *streamEventHandler) HandleEvent(event *core.Event) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	protoEvent := EventToProto(event)
	return h.stream.Send(protoEvent)
}
