//go:build integration

package docker

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/rumpl/seedee/internal/core"
)

func newTestRunner(t *testing.T) *DockerRunner {
	t.Helper()

	c, err := NewClient()
	if err != nil {
		t.Fatalf("creating docker client: %v", err)
	}
	t.Cleanup(func() { c.Close() })

	return NewDockerRunner(c)
}

func TestRunnerSimpleStep(t *testing.T) {
	r := newTestRunner(t)
	ctx := context.Background()

	job := &core.Job{
		Name:  "simple",
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

	step := &core.Step{
		Name:    "echo",
		Command: "echo hello",
	}

	var stdout, stderr bytes.Buffer
	result, err := r.RunStep(ctx, job, step, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run step: %v", err)
	}

	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", result.ExitCode)
	}

	if got := strings.TrimSpace(stdout.String()); got != "hello" {
		t.Fatalf("expected stdout 'hello', got %q", got)
	}
}

func TestRunnerStepFailure(t *testing.T) {
	r := newTestRunner(t)
	ctx := context.Background()

	job := &core.Job{
		Name:  "failure",
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

	step := &core.Step{
		Name:    "fail",
		Command: "exit 1",
	}

	var stdout, stderr bytes.Buffer
	result, err := r.RunStep(ctx, job, step, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run step: %v", err)
	}

	if result.ExitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", result.ExitCode)
	}
}

func TestRunnerMultiStepSharedVolume(t *testing.T) {
	r := newTestRunner(t)
	ctx := context.Background()

	job := &core.Job{
		Name:  "shared-volume",
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

	// Step 1: write a file to the shared workspace volume
	step1 := &core.Step{
		Name:    "write",
		Command: "echo 'shared-data' > /workspace/test.txt",
	}

	var stdout1, stderr1 bytes.Buffer
	result1, err := r.RunStep(ctx, job, step1, &stdout1, &stderr1)
	if err != nil {
		t.Fatalf("run step 1: %v", err)
	}
	if result1.ExitCode != 0 {
		t.Fatalf("step 1 exit code: %d, stderr: %s", result1.ExitCode, stderr1.String())
	}

	// Step 2: read the file written in step 1
	step2 := &core.Step{
		Name:    "read",
		Command: "cat /workspace/test.txt",
	}

	var stdout2, stderr2 bytes.Buffer
	result2, err := r.RunStep(ctx, job, step2, &stdout2, &stderr2)
	if err != nil {
		t.Fatalf("run step 2: %v", err)
	}
	if result2.ExitCode != 0 {
		t.Fatalf("step 2 exit code: %d, stderr: %s", result2.ExitCode, stderr2.String())
	}

	if got := strings.TrimSpace(stdout2.String()); got != "shared-data" {
		t.Fatalf("expected 'shared-data', got %q", got)
	}
}

func TestRunnerEnvironmentVariables(t *testing.T) {
	r := newTestRunner(t)
	ctx := context.Background()

	job := &core.Job{
		Name:  "env-vars",
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

	step := &core.Step{
		Name:    "echo-env",
		Command: "echo $MY_VAR",
		Env: map[string]string{
			"MY_VAR": "test-value",
		},
	}

	var stdout, stderr bytes.Buffer
	result, err := r.RunStep(ctx, job, step, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run step: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", result.ExitCode)
	}

	if got := strings.TrimSpace(stdout.String()); got != "test-value" {
		t.Fatalf("expected 'test-value', got %q", got)
	}
}

func TestRunnerTeardownCleansVolume(t *testing.T) {
	r := newTestRunner(t)
	ctx := context.Background()

	job := &core.Job{
		Name:  "teardown-clean",
		Image: "alpine:latest",
	}

	if err := r.Setup(ctx, job); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Verify volume was stored
	volName, ok := r.volumes[job.Name]
	if !ok {
		t.Fatal("expected volume to be stored after Setup")
	}
	if volName == "" {
		t.Fatal("expected non-empty volume name")
	}

	// Teardown should remove the volume and clean up the map
	if err := r.Teardown(ctx, job); err != nil {
		t.Fatalf("teardown: %v", err)
	}

	if _, ok := r.volumes[job.Name]; ok {
		t.Fatal("expected volume to be removed from map after Teardown")
	}

	// Attempting to remove the same volume again should fail (it was already removed)
	err := r.client.RemoveVolume(ctx, volName)
	if err == nil {
		t.Fatal("expected error removing already-removed volume")
	}
}
