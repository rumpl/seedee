//go:build integration

package docker

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestPullImage(t *testing.T) {
	c, err := NewClient()
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}
	defer c.Close()

	var buf bytes.Buffer
	err = c.PullImage(context.Background(), "alpine:latest", &buf)
	if err != nil {
		t.Fatalf("pulling image: %v", err)
	}

	if buf.Len() == 0 {
		t.Fatal("expected pull output, got nothing")
	}
}

func TestRunContainer_Success(t *testing.T) {
	c, err := NewClient()
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}
	defer c.Close()

	var stdout, stderr bytes.Buffer
	exitCode, err := c.RunContainer(context.Background(), &RunOptions{
		Image:   "alpine:latest",
		Command: "echo hello",
		Stdout:  &stdout,
		Stderr:  &stderr,
	})
	if err != nil {
		t.Fatalf("running container: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if got := strings.TrimSpace(stdout.String()); got != "hello" {
		t.Fatalf("expected stdout 'hello', got %q", got)
	}
}

func TestRunContainer_Failure(t *testing.T) {
	c, err := NewClient()
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}
	defer c.Close()

	var stdout, stderr bytes.Buffer
	exitCode, err := c.RunContainer(context.Background(), &RunOptions{
		Image:   "alpine:latest",
		Command: "exit 42",
		Stdout:  &stdout,
		Stderr:  &stderr,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exitCode != 42 {
		t.Fatalf("expected exit code 42, got %d", exitCode)
	}
}

func TestRunContainer_StderrCapture(t *testing.T) {
	c, err := NewClient()
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}
	defer c.Close()

	var stdout, stderr bytes.Buffer
	exitCode, err := c.RunContainer(context.Background(), &RunOptions{
		Image:   "alpine:latest",
		Command: "echo oops >&2",
		Stdout:  &stdout,
		Stderr:  &stderr,
	})
	if err != nil {
		t.Fatalf("running container: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if got := strings.TrimSpace(stderr.String()); got != "oops" {
		t.Fatalf("expected stderr 'oops', got %q", got)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}
}

func TestCreateAndRemoveVolume(t *testing.T) {
	c, err := NewClient()
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}
	defer c.Close()

	ctx := context.Background()
	name := "seedee-test-volume"

	if err := c.CreateVolume(ctx, name); err != nil {
		t.Fatalf("creating volume: %v", err)
	}

	// Clean up: remove the volume.
	if err := c.RemoveVolume(ctx, name); err != nil {
		t.Fatalf("removing volume: %v", err)
	}
}
