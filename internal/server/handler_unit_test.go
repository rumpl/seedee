package server

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"connectrpc.com/connect"

	seedeev1 "github.com/rumpl/seedee/gen/seedee/v1"
	"github.com/rumpl/seedee/internal/core"
)

func newTestHandler() *CIServiceHandler {
	return NewCIServiceHandler(slog.Default())
}

func addTestPipeline(h *CIServiceHandler, pipeline *core.Pipeline, done bool) context.CancelFunc {
	ctx, cancel := context.WithCancel(context.Background())
	run := &pipelineRun{
		pipeline: pipeline,
		cancel:   cancel,
		done:     make(chan struct{}),
	}
	if done {
		close(run.done)
	}
	h.mu.Lock()
	h.pipelines[pipeline.ID] = run
	h.mu.Unlock()
	return func() {
		cancel()
		if !done {
			// clean up the context if not already done
			_ = ctx
		}
	}
}

func assertConnectCodeUnit(t *testing.T, err error, want connect.Code) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var connectErr *connect.Error
	if ok := err.(*connect.Error); ok != nil {
		connectErr = ok
	}
	if connectErr == nil {
		t.Fatalf("expected *connect.Error, got %T: %v", err, err)
	}
	if connectErr.Code() != want {
		t.Errorf("expected code %v, got %v", want, connectErr.Code())
	}
}

func TestGetPipelineStatus_NotFound(t *testing.T) {
	h := newTestHandler()
	_, err := h.GetPipelineStatus(context.Background(), connect.NewRequest(&seedeev1.GetPipelineStatusRequest{
		PipelineId: "nonexistent",
	}))
	assertConnectCodeUnit(t, err, connect.CodeNotFound)
}

func TestGetPipelineStatus_EmptyID(t *testing.T) {
	h := newTestHandler()
	_, err := h.GetPipelineStatus(context.Background(), connect.NewRequest(&seedeev1.GetPipelineStatusRequest{
		PipelineId: "",
	}))
	assertConnectCodeUnit(t, err, connect.CodeInvalidArgument)
}

func TestGetPipelineStatus_RunningPipeline(t *testing.T) {
	h := newTestHandler()

	start := time.Now()
	pipeline := &core.Pipeline{
		ID:        "pipe-running",
		Name:      "running-pipeline",
		Status:    core.StatusRunning,
		StartedAt: start,
		Jobs: []*core.Job{
			{
				Name:      "build",
				Status:    core.StatusRunning,
				StartedAt: start,
				Steps: []*core.Step{
					{
						Name:      "compile",
						Status:    core.StatusRunning,
						StartedAt: start,
					},
				},
			},
		},
	}

	cleanup := addTestPipeline(h, pipeline, false)
	defer cleanup()

	resp, err := h.GetPipelineStatus(context.Background(), connect.NewRequest(&seedeev1.GetPipelineStatusRequest{
		PipelineId: "pipe-running",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msg := resp.Msg
	if msg.PipelineId != "pipe-running" {
		t.Errorf("PipelineId = %q, want %q", msg.PipelineId, "pipe-running")
	}
	if msg.PipelineName != "running-pipeline" {
		t.Errorf("PipelineName = %q, want %q", msg.PipelineName, "running-pipeline")
	}
	if msg.Status != seedeev1.Status_STATUS_RUNNING {
		t.Errorf("Status = %v, want STATUS_RUNNING", msg.Status)
	}
	if len(msg.Jobs) != 1 {
		t.Fatalf("Jobs count = %d, want 1", len(msg.Jobs))
	}
	if msg.Jobs[0].Status != seedeev1.Status_STATUS_RUNNING {
		t.Errorf("Job status = %v, want STATUS_RUNNING", msg.Jobs[0].Status)
	}
	if msg.StartedAt == nil {
		t.Error("StartedAt should not be nil")
	}
	if msg.Duration == nil {
		t.Error("Duration should not be nil for a running pipeline")
	}
}

func TestGetPipelineStatus_CompletedPipeline(t *testing.T) {
	h := newTestHandler()

	start := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	end := start.Add(15 * time.Second)

	pipeline := &core.Pipeline{
		ID:        "pipe-done",
		Name:      "completed-pipeline",
		Status:    core.StatusSuccess,
		StartedAt: start,
		EndedAt:   end,
		Jobs: []*core.Job{
			{
				Name:      "test-job",
				Status:    core.StatusSuccess,
				StartedAt: start,
				EndedAt:   end,
				Steps: []*core.Step{
					{
						Name:      "run-tests",
						Status:    core.StatusSuccess,
						ExitCode:  0,
						StartedAt: start,
						EndedAt:   end,
					},
				},
			},
		},
	}

	cleanup := addTestPipeline(h, pipeline, true)
	defer cleanup()

	resp, err := h.GetPipelineStatus(context.Background(), connect.NewRequest(&seedeev1.GetPipelineStatusRequest{
		PipelineId: "pipe-done",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msg := resp.Msg
	if msg.Status != seedeev1.Status_STATUS_SUCCESS {
		t.Errorf("Status = %v, want STATUS_SUCCESS", msg.Status)
	}
	if msg.Duration == nil {
		t.Fatal("Duration should not be nil")
	}
	if msg.Duration.AsDuration() != 15*time.Second {
		t.Errorf("Duration = %v, want 15s", msg.Duration.AsDuration())
	}
	if len(msg.Jobs) != 1 {
		t.Fatalf("Jobs count = %d, want 1", len(msg.Jobs))
	}
	if msg.Jobs[0].Status != seedeev1.Status_STATUS_SUCCESS {
		t.Errorf("Job status = %v, want STATUS_SUCCESS", msg.Jobs[0].Status)
	}
	if msg.Jobs[0].Duration.AsDuration() != 15*time.Second {
		t.Errorf("Job Duration = %v, want 15s", msg.Jobs[0].Duration.AsDuration())
	}
	if msg.Jobs[0].Steps[0].ExitCode != 0 {
		t.Errorf("Step ExitCode = %d, want 0", msg.Jobs[0].Steps[0].ExitCode)
	}
}

func TestCancelPipeline_Success(t *testing.T) {
	h := newTestHandler()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pipeline := &core.Pipeline{
		ID:        "pipe-cancel",
		Name:      "cancel-me",
		Status:    core.StatusRunning,
		StartedAt: time.Now(),
	}

	run := &pipelineRun{
		pipeline: pipeline,
		cancel:   cancel,
		done:     make(chan struct{}),
	}
	h.mu.Lock()
	h.pipelines[pipeline.ID] = run
	h.mu.Unlock()

	resp, err := h.CancelPipeline(context.Background(), connect.NewRequest(&seedeev1.CancelPipelineRequest{
		PipelineId: "pipe-cancel",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.Msg.Canceled {
		t.Error("expected Canceled=true")
	}
	if resp.Msg.Message == "" {
		t.Error("expected non-empty message")
	}

	// Verify the context was actually cancelled
	if ctx.Err() == nil {
		t.Error("expected context to be cancelled")
	}
}

func TestCancelPipeline_AlreadyCompleted(t *testing.T) {
	h := newTestHandler()

	pipeline := &core.Pipeline{
		ID:        "pipe-completed",
		Name:      "already-done",
		Status:    core.StatusSuccess,
		StartedAt: time.Now().Add(-10 * time.Second),
		EndedAt:   time.Now(),
	}

	cleanup := addTestPipeline(h, pipeline, true)
	defer cleanup()

	resp, err := h.CancelPipeline(context.Background(), connect.NewRequest(&seedeev1.CancelPipelineRequest{
		PipelineId: "pipe-completed",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Msg.Canceled {
		t.Error("expected Canceled=false for already completed pipeline")
	}
	if resp.Msg.Message != "pipeline already completed" {
		t.Errorf("Message = %q, want %q", resp.Msg.Message, "pipeline already completed")
	}
}

func TestCancelPipeline_NotFound(t *testing.T) {
	h := newTestHandler()
	_, err := h.CancelPipeline(context.Background(), connect.NewRequest(&seedeev1.CancelPipelineRequest{
		PipelineId: "nonexistent",
	}))
	assertConnectCodeUnit(t, err, connect.CodeNotFound)
}

func TestCancelPipeline_EmptyID(t *testing.T) {
	h := newTestHandler()
	_, err := h.CancelPipeline(context.Background(), connect.NewRequest(&seedeev1.CancelPipelineRequest{
		PipelineId: "",
	}))
	assertConnectCodeUnit(t, err, connect.CodeInvalidArgument)
}

func TestPruneOldRuns(t *testing.T) {
	h := newTestHandler()

	now := time.Now()

	// Old completed run (should be pruned)
	oldPipeline := &core.Pipeline{
		ID:        "pipe-old",
		Name:      "old-pipeline",
		Status:    core.StatusSuccess,
		StartedAt: now.Add(-3 * time.Hour),
		EndedAt:   now.Add(-2 * time.Hour),
	}
	addTestPipeline(h, oldPipeline, true)

	// Recent completed run (should be kept)
	recentPipeline := &core.Pipeline{
		ID:        "pipe-recent",
		Name:      "recent-pipeline",
		Status:    core.StatusSuccess,
		StartedAt: now.Add(-10 * time.Minute),
		EndedAt:   now.Add(-5 * time.Minute),
	}
	addTestPipeline(h, recentPipeline, true)

	// Running pipeline (should be kept, EndedAt is zero)
	runningPipeline := &core.Pipeline{
		ID:        "pipe-running",
		Name:      "running-pipeline",
		Status:    core.StatusRunning,
		StartedAt: now.Add(-4 * time.Hour),
	}
	addTestPipeline(h, runningPipeline, false)

	// Prune runs older than 1 hour
	h.PruneOldRuns(1 * time.Hour)

	h.mu.RLock()
	defer h.mu.RUnlock()

	if _, ok := h.pipelines["pipe-old"]; ok {
		t.Error("expected old pipeline to be pruned")
	}
	if _, ok := h.pipelines["pipe-recent"]; !ok {
		t.Error("expected recent pipeline to be kept")
	}
	if _, ok := h.pipelines["pipe-running"]; !ok {
		t.Error("expected running pipeline to be kept")
	}
}

func TestPruneOldRuns_Empty(t *testing.T) {
	h := newTestHandler()
	// Should not panic
	h.PruneOldRuns(1 * time.Hour)

	h.mu.RLock()
	defer h.mu.RUnlock()
	if len(h.pipelines) != 0 {
		t.Errorf("expected 0 pipelines, got %d", len(h.pipelines))
	}
}

func TestGetPipelineStatus_ConcurrentAccess(t *testing.T) {
	h := newTestHandler()

	// Add several pipelines
	for i := range 10 {
		pipeline := &core.Pipeline{
			ID:        fmt.Sprintf("pipe-%d", i),
			Name:      fmt.Sprintf("pipeline-%d", i),
			Status:    core.StatusRunning,
			StartedAt: time.Now(),
		}
		addTestPipeline(h, pipeline, false)
	}

	// Query them concurrently
	done := make(chan struct{})
	for i := range 10 {
		go func(id int) {
			defer func() { done <- struct{}{} }()
			for range 100 {
				_, _ = h.GetPipelineStatus(context.Background(), connect.NewRequest(&seedeev1.GetPipelineStatusRequest{
					PipelineId: fmt.Sprintf("pipe-%d", id),
				}))
			}
		}(i)
	}

	for range 10 {
		<-done
	}
}
