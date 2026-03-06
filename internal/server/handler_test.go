//go:build integration

package server_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"

	seedeev1 "github.com/rumpl/seedee/gen/seedee/v1"
	"github.com/rumpl/seedee/gen/seedee/v1/seedeev1connect"
	"github.com/rumpl/seedee/internal/server"
)

func startIntegrationServer(t *testing.T) (baseURL string, cancel context.CancelFunc) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx, cancelFn := context.WithCancel(context.Background())

	srv := server.NewServer(addr, logger)

	go func() {
		_ = srv.Start(ctx)
	}()

	// Wait for the server to be ready.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, dialErr := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if dialErr == nil {
			conn.Close()
			return "http://" + addr, cancelFn
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancelFn()
	t.Fatal("server failed to start within timeout")
	return "", nil
}

func assertConnectCodeIntegration(t *testing.T, err error, want connect.Code) {
	t.Helper()

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		t.Fatalf("expected *connect.Error, got %T: %v", err, err)
	}

	if connectErr.Code() != want {
		t.Errorf("expected code %v, got %v", want, connectErr.Code())
	}
}

func TestRunPipeline_Success(t *testing.T) {
	baseURL, cancel := startIntegrationServer(t)
	defer cancel()

	client := seedeev1connect.NewCIServiceClient(http.DefaultClient, baseURL)

	stream, err := client.RunPipeline(context.Background(), connect.NewRequest(&seedeev1.RunPipelineRequest{
		Pipeline: &seedeev1.PipelineDefinition{
			Name: "test-pipeline",
			Jobs: map[string]*seedeev1.JobDefinition{
				"echo-job": {
					Image: "alpine:latest",
					Steps: []*seedeev1.StepDefinition{
						{
							Name: "say-hello",
							Run:  "echo hello world",
						},
					},
				},
			},
		},
	}))
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	var events []*seedeev1.RunPipelineEvent
	for stream.Receive() {
		events = append(events, stream.Msg())
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("stream error: %v", err)
	}

	if len(events) < 2 {
		t.Fatalf("expected at least 2 events (started + finished), got %d", len(events))
	}

	// First event should be PipelineStarted
	if events[0].Type != seedeev1.EventType_EVENT_TYPE_PIPELINE_STARTED {
		t.Errorf("first event type = %v, want PIPELINE_STARTED", events[0].Type)
	}

	// Last event should be PipelineFinished with success
	last := events[len(events)-1]
	if last.Type != seedeev1.EventType_EVENT_TYPE_PIPELINE_FINISHED {
		t.Errorf("last event type = %v, want PIPELINE_FINISHED", last.Type)
	}
	if last.Status != seedeev1.Status_STATUS_SUCCESS {
		t.Errorf("last event status = %v, want STATUS_SUCCESS", last.Status)
	}

	// All events should have the same pipeline ID
	pipelineID := events[0].PipelineId
	if pipelineID == "" {
		t.Fatal("expected non-empty pipeline ID")
	}
	for i, ev := range events {
		if ev.PipelineId != pipelineID {
			t.Errorf("event[%d].PipelineId = %q, want %q", i, ev.PipelineId, pipelineID)
		}
	}

	// Check that we got at least one log event
	gotLog := false
	for _, ev := range events {
		if ev.Type == seedeev1.EventType_EVENT_TYPE_STEP_LOG {
			gotLog = true
			if ev.JobName != "echo-job" {
				t.Errorf("log event JobName = %q, want %q", ev.JobName, "echo-job")
			}
			if ev.StepName != "say-hello" {
				t.Errorf("log event StepName = %q, want %q", ev.StepName, "say-hello")
			}
		}
	}
	if !gotLog {
		t.Error("expected at least one STEP_LOG event")
	}
}

func TestRunPipeline_InvalidRequest(t *testing.T) {
	baseURL, cancel := startIntegrationServer(t)
	defer cancel()

	client := seedeev1connect.NewCIServiceClient(http.DefaultClient, baseURL)

	// nil pipeline
	stream, err := client.RunPipeline(context.Background(), connect.NewRequest(&seedeev1.RunPipelineRequest{}))
	if err != nil {
		assertConnectCodeIntegration(t, err, connect.CodeInvalidArgument)
		return
	}
	// Try to receive; should get error
	if stream.Receive() {
		t.Error("expected no messages from invalid request")
	}
	assertConnectCodeIntegration(t, stream.Err(), connect.CodeInvalidArgument)
}

func TestRunPipeline_InvalidPipeline(t *testing.T) {
	baseURL, cancel := startIntegrationServer(t)
	defer cancel()

	client := seedeev1connect.NewCIServiceClient(http.DefaultClient, baseURL)

	// Pipeline with missing image
	stream, err := client.RunPipeline(context.Background(), connect.NewRequest(&seedeev1.RunPipelineRequest{
		Pipeline: &seedeev1.PipelineDefinition{
			Name: "bad-pipeline",
			Jobs: map[string]*seedeev1.JobDefinition{
				"bad-job": {
					// Missing image
					Steps: []*seedeev1.StepDefinition{
						{
							Name: "step",
							Run:  "echo hello",
						},
					},
				},
			},
		},
	}))
	if err != nil {
		assertConnectCodeIntegration(t, err, connect.CodeInvalidArgument)
		return
	}
	if stream.Receive() {
		t.Error("expected no messages from invalid pipeline")
	}
	assertConnectCodeIntegration(t, stream.Err(), connect.CodeInvalidArgument)
}

func TestRunPipeline_StepFailure(t *testing.T) {
	baseURL, cancel := startIntegrationServer(t)
	defer cancel()

	client := seedeev1connect.NewCIServiceClient(http.DefaultClient, baseURL)

	stream, err := client.RunPipeline(context.Background(), connect.NewRequest(&seedeev1.RunPipelineRequest{
		Pipeline: &seedeev1.PipelineDefinition{
			Name: "fail-pipeline",
			Jobs: map[string]*seedeev1.JobDefinition{
				"fail-job": {
					Image: "alpine:latest",
					Steps: []*seedeev1.StepDefinition{
						{
							Name: "will-fail",
							Run:  "exit 1",
						},
					},
				},
			},
		},
	}))
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	var events []*seedeev1.RunPipelineEvent
	for stream.Receive() {
		events = append(events, stream.Msg())
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("stream error: %v", err)
	}

	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}

	// Last event should be PipelineFinished with failure
	last := events[len(events)-1]
	if last.Type != seedeev1.EventType_EVENT_TYPE_PIPELINE_FINISHED {
		t.Errorf("last event type = %v, want PIPELINE_FINISHED", last.Type)
	}
	if last.Status != seedeev1.Status_STATUS_FAILED {
		t.Errorf("last event status = %v, want STATUS_FAILED", last.Status)
	}
}

func TestRunPipeline_Cancellation(t *testing.T) {
	baseURL, cancel := startIntegrationServer(t)
	defer cancel()

	client := seedeev1connect.NewCIServiceClient(http.DefaultClient, baseURL)

	ctx, clientCancel := context.WithCancel(context.Background())

	stream, err := client.RunPipeline(ctx, connect.NewRequest(&seedeev1.RunPipelineRequest{
		Pipeline: &seedeev1.PipelineDefinition{
			Name: "cancel-pipeline",
			Jobs: map[string]*seedeev1.JobDefinition{
				"slow-job": {
					Image: "alpine:latest",
					Steps: []*seedeev1.StepDefinition{
						{
							Name: "slow-step",
							Run:  "sleep 60",
						},
					},
				},
			},
		},
	}))
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	// Read first event, then cancel
	if stream.Receive() {
		clientCancel()
	} else {
		t.Fatal("expected at least one event before cancel")
	}

	// Should return quickly (not 60s)
	done := make(chan struct{})
	go func() {
		for stream.Receive() {
			// drain
		}
		close(done)
	}()

	select {
	case <-done:
		// ok
	case <-time.After(10 * time.Second):
		t.Error("cancellation took too long (>10s)")
	}
}
