package server_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	seedeev1 "github.com/rumpl/seedee/gen/seedee/v1"
	"github.com/rumpl/seedee/gen/seedee/v1/seedeev1connect"
	"github.com/rumpl/seedee/internal/server"
)

func startTestServer(t *testing.T) (baseURL string, cancel context.CancelFunc) {
	t.Helper()

	// Find a free port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx, cancelFn := context.WithCancel(context.Background())

	cfg := server.Config{Addr: addr}
	srv := server.NewServer(cfg, logger)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
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

func TestServer_HealthCheck(t *testing.T) {
	baseURL, cancel := startTestServer(t)
	defer cancel()

	resp, err := http.Get(baseURL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}

	if got := strings.TrimSpace(string(body)); got != "ok" {
		t.Errorf("expected body %q, got %q", "ok", got)
	}
}

func TestServer_GracefulShutdown(t *testing.T) {
	baseURL, cancel := startTestServer(t)

	// Verify server is up.
	resp, err := http.Get(baseURL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz before shutdown: %v", err)
	}
	resp.Body.Close()

	// Cancel context to trigger graceful shutdown.
	cancel()

	// Wait for the server to stop accepting connections.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		_, dialErr := net.DialTimeout("tcp", strings.TrimPrefix(baseURL, "http://"), 50*time.Millisecond)
		if dialErr != nil {
			// Server shut down cleanly.
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Error("server did not shut down within timeout")
}

func TestServer_RunPipeline_NilPipeline(t *testing.T) {
	baseURL, cancel := startTestServer(t)
	defer cancel()

	client := seedeev1connect.NewCIServiceClient(http.DefaultClient, baseURL)

	stream, err := client.RunPipeline(context.Background(), connect.NewRequest(&seedeev1.RunPipelineRequest{}))
	if err != nil {
		// RunPipeline with no pipeline should return InvalidArgument.
		assertConnectCode(t, err, connect.CodeInvalidArgument)
		return
	}
	// Try to receive; the server should close with InvalidArgument.
	if stream.Receive() {
		t.Error("expected no messages from nil pipeline request")
	}
	assertConnectCode(t, stream.Err(), connect.CodeInvalidArgument)
}

func TestServer_UnimplementedRPCs(t *testing.T) {
	baseURL, cancel := startTestServer(t)
	defer cancel()

	client := seedeev1connect.NewCIServiceClient(http.DefaultClient, baseURL)

	t.Run("GetPipelineStatus", func(t *testing.T) {
		_, err := client.GetPipelineStatus(context.Background(), connect.NewRequest(&seedeev1.GetPipelineStatusRequest{
			PipelineId: "test-id",
		}))
		assertConnectCode(t, err, connect.CodeUnimplemented)
	})

	t.Run("CancelPipeline", func(t *testing.T) {
		_, err := client.CancelPipeline(context.Background(), connect.NewRequest(&seedeev1.CancelPipelineRequest{
			PipelineId: "test-id",
		}))
		assertConnectCode(t, err, connect.CodeUnimplemented)
	})
}

func assertConnectCode(t *testing.T, err error, want connect.Code) {
	t.Helper()

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var connectErr *connect.Error
	if ok := errors.As(err, &connectErr); !ok {
		t.Fatalf("expected *connect.Error, got %T: %v", err, err)
	}

	if connectErr.Code() != want {
		t.Errorf("expected code %v, got %v", want, connectErr.Code())
	}
}
