//go:build integration

package docker

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rumpl/seedee/internal/core"
)

func TestInjectSource(t *testing.T) {
	c, err := NewClient()
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}
	defer c.Close()

	ctx := context.Background()

	// Create a temp dir with files.
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "hello.txt"), []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create volume and inject source.
	volName := "seedee-test-inject-source"
	if err := c.CreateVolume(ctx, volName); err != nil {
		t.Fatalf("creating volume: %v", err)
	}
	defer func() { _ = c.RemoveVolume(ctx, volName) }()

	wm := NewWorkspaceManager(c)
	if err := wm.InjectSource(ctx, volName, tmpDir); err != nil {
		t.Fatalf("inject source: %v", err)
	}

	// Run a container that lists /workspace and verify files.
	var stdout, stderr bytes.Buffer
	exitCode, err := c.RunContainer(ctx, &RunOptions{
		Image:   "alpine:latest",
		Command: "cat /workspace/hello.txt",
		Binds:   []string{volName + ":/workspace"},
		Stdout:  &stdout,
		Stderr:  &stderr,
	})
	if err != nil {
		t.Fatalf("running container: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", exitCode, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "hello world" {
		t.Fatalf("expected 'hello world', got %q", got)
	}
}

func TestInjectSource_Subdirectories(t *testing.T) {
	c, err := NewClient()
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}
	defer c.Close()

	ctx := context.Background()

	// Source dir with nested dirs.
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "src", "pkg")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "root.txt"), []byte("root"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "deep.txt"), []byte("deep content"), 0o644); err != nil {
		t.Fatal(err)
	}

	volName := "seedee-test-inject-subdirs"
	if err := c.CreateVolume(ctx, volName); err != nil {
		t.Fatalf("creating volume: %v", err)
	}
	defer func() { _ = c.RemoveVolume(ctx, volName) }()

	wm := NewWorkspaceManager(c)
	if err := wm.InjectSource(ctx, volName, tmpDir); err != nil {
		t.Fatalf("inject source: %v", err)
	}

	// Verify tree structure is preserved.
	var stdout, stderr bytes.Buffer
	exitCode, err := c.RunContainer(ctx, &RunOptions{
		Image:   "alpine:latest",
		Command: "cat /workspace/src/pkg/deep.txt && echo '---' && cat /workspace/root.txt",
		Binds:   []string{volName + ":/workspace"},
		Stdout:  &stdout,
		Stderr:  &stderr,
	})
	if err != nil {
		t.Fatalf("running container: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", exitCode, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "deep content") {
		t.Fatalf("expected 'deep content' in output, got %q", output)
	}
	if !strings.Contains(output, "root") {
		t.Fatalf("expected 'root' in output, got %q", output)
	}
}

func TestInjectSource_EmptyDir(t *testing.T) {
	c, err := NewClient()
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}
	defer c.Close()

	ctx := context.Background()

	tmpDir := t.TempDir()

	volName := "seedee-test-inject-empty"
	if err := c.CreateVolume(ctx, volName); err != nil {
		t.Fatalf("creating volume: %v", err)
	}
	defer func() { _ = c.RemoveVolume(ctx, volName) }()

	wm := NewWorkspaceManager(c)
	if err := wm.InjectSource(ctx, volName, tmpDir); err != nil {
		t.Fatalf("inject source should not error on empty dir: %v", err)
	}

	// Verify /workspace is empty.
	var stdout, stderr bytes.Buffer
	exitCode, err := c.RunContainer(ctx, &RunOptions{
		Image:   "alpine:latest",
		Command: "ls -A /workspace | wc -l",
		Binds:   []string{volName + ":/workspace"},
		Stdout:  &stdout,
		Stderr:  &stderr,
	})
	if err != nil {
		t.Fatalf("running container: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", exitCode, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "0" {
		t.Fatalf("expected 0 files, got %q", got)
	}
}

func TestCleanupRegistry_Integration(t *testing.T) {
	c, err := NewClient()
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}
	defer c.Close()

	ctx := context.Background()

	// Register 3 volumes, cleanup all.
	registry := NewCleanupRegistry()
	names := []string{"seedee-test-cleanup-1", "seedee-test-cleanup-2", "seedee-test-cleanup-3"}

	for _, name := range names {
		if err := c.CreateVolume(ctx, name); err != nil {
			t.Fatalf("creating volume %s: %v", name, err)
		}
		registry.RegisterVolume(name)
	}

	if err := registry.CleanupAll(ctx, c); err != nil {
		t.Fatalf("CleanupAll: %v", err)
	}

	// Verify all volumes removed.
	for _, name := range names {
		err := c.RemoveVolume(ctx, name)
		if err == nil {
			t.Fatalf("expected error removing already-cleaned volume %s", name)
		}
	}
}

func TestCleanupRegistry_PartialFailure(t *testing.T) {
	c, err := NewClient()
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}
	defer c.Close()

	ctx := context.Background()

	// Create 2 volumes, manually remove one, then cleanup.
	registry := NewCleanupRegistry()
	vol1 := "seedee-test-partial-1"
	vol2 := "seedee-test-partial-2"

	if err := c.CreateVolume(ctx, vol1); err != nil {
		t.Fatalf("creating volume: %v", err)
	}
	if err := c.CreateVolume(ctx, vol2); err != nil {
		t.Fatalf("creating volume: %v", err)
	}

	registry.RegisterVolume(vol1)
	registry.RegisterVolume(vol2)

	// Manually remove vol1 first.
	if err := c.RemoveVolume(ctx, vol1); err != nil {
		t.Fatalf("manually removing vol1: %v", err)
	}

	// CleanupAll should return an error for vol1 but still clean vol2.
	err = registry.CleanupAll(ctx, c)
	if err == nil {
		t.Fatal("expected error from CleanupAll when one volume already removed")
	}

	// vol2 should have been cleaned.
	err = c.RemoveVolume(ctx, vol2)
	if err == nil {
		t.Fatal("expected error removing vol2 (should already be cleaned)")
	}
}

func TestRunnerWithSourceInjection(t *testing.T) {
	c, err := NewClient()
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}
	defer c.Close()

	// Create a temp dir with source.
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := NewRunnerWithConfig(c, RunnerConfig{
		SourceDir:  tmpDir,
		PipelineID: "test-runner-inject",
	})

	ctx := context.Background()
	job := &core.Job{
		Name:  "build",
		Image: "alpine:latest",
	}

	if err := r.Setup(ctx, job); err != nil {
		t.Fatalf("setup: %v", err)
	}
	defer func() {
		if err := r.Teardown(ctx, job); err != nil {
			t.Fatalf("teardown: %v", err)
		}
	}()

	// Verify the injected file is present.
	step := &core.Step{
		Name:    "check-source",
		Command: "cat /workspace/main.go",
	}

	var stdout, stderr bytes.Buffer
	result, err := r.RunStep(ctx, job, step, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run step: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", result.ExitCode, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "package main" {
		t.Fatalf("expected 'package main', got %q", got)
	}
}
