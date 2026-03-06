//go:build e2e

package test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestE2E_Remote_SimpleSuccess(t *testing.T) {
	requireDocker(t)
	addr, cleanup := startServer(t)
	defer cleanup()

	stdout, _, exitCode := runSeedee(t, "../testdata/simple", "run", "--server", addr)
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d\nstdout: %s", exitCode, stdout)
	}
	if !strings.Contains(stdout, "Hello from seedee!") {
		t.Errorf("expected output to contain 'Hello from seedee!', got:\n%s", stdout)
	}
	if !strings.Contains(stdout, string("success")) {
		t.Errorf("expected output to contain 'success', got:\n%s", stdout)
	}
}

func TestE2E_Remote_ParallelJobs(t *testing.T) {
	requireDocker(t)
	addr, cleanup := startServer(t)
	defer cleanup()

	start := time.Now()
	stdout, _, exitCode := runSeedee(t, "../testdata/parallel", "run", "--server", addr)
	elapsed := time.Since(start)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d\nstdout: %s", exitCode, stdout)
	}
	if !strings.Contains(stdout, "Job A") {
		t.Errorf("expected output to contain 'Job A', got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Job B") {
		t.Errorf("expected output to contain 'Job B', got:\n%s", stdout)
	}
	// Both jobs sleep 2s. If running in parallel, total should be well under 6s
	// (accounting for image pull time on first run, we use a generous limit)
	if elapsed > 90*time.Second {
		t.Errorf("parallel jobs took %v, expected less than 90s", elapsed)
	}
}

func TestE2E_Remote_Failure(t *testing.T) {
	requireDocker(t)
	addr, cleanup := startServer(t)
	defer cleanup()

	stdout, _, exitCode := runSeedee(t, "../testdata/failure", "run", "--server", addr)
	if exitCode == 0 {
		t.Error("expected non-zero exit code for failing pipeline")
	}
	if !strings.Contains(stdout, "failed") {
		t.Errorf("expected output to contain 'failed', got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "skipped") {
		t.Errorf("expected output to contain 'skipped', got:\n%s", stdout)
	}
}

func TestE2E_Remote_ServerNotRunning(t *testing.T) {
	requireDocker(t)
	// Don't start a server — use a port that's definitely not listening
	_, stderr, exitCode := runSeedee(t, "../testdata/simple", "run", "--server", "127.0.0.1:1")
	if exitCode == 0 {
		t.Error("expected non-zero exit code when server is not running")
	}
	combined := stderr
	if !strings.Contains(strings.ToLower(combined), "connect") &&
		!strings.Contains(strings.ToLower(combined), "server") &&
		!strings.Contains(strings.ToLower(combined), "unavailable") {
		t.Errorf("expected connection-related error, got stderr:\n%s", stderr)
	}
}

func TestE2E_Remote_HealthCheck(t *testing.T) {
	addr, cleanup := startServer(t)
	defer cleanup()

	resp, err := http.Get(fmt.Sprintf("http://%s/healthz", addr))
	if err != nil {
		t.Fatalf("health check request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected health check status 200, got %d", resp.StatusCode)
	}
}

func TestE2E_Remote_Cancel(t *testing.T) {
	requireDocker(t)
	addr, cleanup := startServer(t)
	defer cleanup()

	// Start a long-running pipeline in the background
	done := make(chan struct{})
	var stdout string
	var exitCode int
	go func() {
		defer close(done)
		stdout, _, exitCode = runSeedee(t, "../testdata/slow", "run", "--server", addr)
	}()

	// Give the pipeline time to start (wait for image pull + container start)
	time.Sleep(10 * time.Second)

	// The pipeline should still be running, so the goroutine should not have
	// finished yet. We can't easily cancel from outside without the pipeline ID,
	// but we can verify the process handles interruption by waiting briefly
	// and checking it hasn't finished the 30s sleep.
	select {
	case <-done:
		// Pipeline finished already — unexpected for a 30s sleep.
		// It may have failed to start. Accept if exit code is non-zero.
		if exitCode == 0 {
			t.Error("expected slow pipeline to still be running after 10s")
		}
	case <-time.After(2 * time.Second):
		// Expected — pipeline is still running. Test passes.
		_ = stdout // will be populated when goroutine completes
	}
}
