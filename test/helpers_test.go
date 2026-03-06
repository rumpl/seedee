//go:build e2e

package test

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// seedeeBin returns the path to the seedee CLI binary.
// Skips the test if the binary is not found.
func seedeeBin(t *testing.T) string {
	t.Helper()
	bin := filepath.Join("..", "bin", "seedee")
	if _, err := os.Stat(bin); err != nil {
		t.Skip("seedee binary not found — run `make build` first")
	}
	return bin
}

// seedeedBin returns the path to the seedeed server binary.
// Skips the test if the binary is not found.
func seedeedBin(t *testing.T) string {
	t.Helper()
	bin := filepath.Join("..", "bin", "seedeed")
	if _, err := os.Stat(bin); err != nil {
		t.Skip("seedeed binary not found — run `make build` first")
	}
	return bin
}

// requireDocker skips the test if Docker is not available.
func requireDocker(t *testing.T) {
	t.Helper()
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("Docker is not available — skipping E2E test")
	}
}

// startServer starts a seedeed server on a random port and returns the address.
// It returns a cleanup function that stops the server.
func startServer(t *testing.T) (addr string, cleanup func()) {
	t.Helper()

	// Find a free port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("finding free port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("closing listener: %v", err)
	}

	addr = fmt.Sprintf("127.0.0.1:%d", port)

	// Start server process
	cmd := exec.Command(seedeedBin(t), "--addr", addr)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting server: %v", err)
	}

	// Wait for server to be ready
	ready := false
	for i := 0; i < 50; i++ { // 5 seconds max
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			ready = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !ready {
		cmd.Process.Kill()
		cmd.Wait()
		t.Fatal("server did not start within 5 seconds")
	}

	cleanup = func() {
		cmd.Process.Signal(syscall.SIGTERM)
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			cmd.Process.Kill()
			<-done
		}
	}

	return addr, cleanup
}

// runSeedee runs the seedee CLI with the given arguments.
// workDir is the working directory for the command.
// Returns stdout, stderr, and exit code.
func runSeedee(t *testing.T, workDir string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	return runSeedeeCtx(t, ctx, workDir, args...)
}

// runSeedeeCtx runs the seedee CLI with an explicit context.
func runSeedeeCtx(t *testing.T, ctx context.Context, workDir string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()

	cmd := exec.CommandContext(ctx, seedeeBin(t), args...)
	cmd.Dir = workDir

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	stdout = stdoutBuf.String()
	stderr = stderrBuf.String()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			// Context canceled, timeout, etc.
			exitCode = -1
		}
	}

	return stdout, stderr, exitCode
}
