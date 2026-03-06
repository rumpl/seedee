package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCmd_LoadsConfigAndRunsLocal(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".seedee.yml")
	content := `pipeline:
  name: test-pipeline
  jobs:
    build:
      image: golang:1.22
      steps:
        - name: Build
          run: go build ./...
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}

	root := NewRootCmd()
	root.SetArgs([]string{"run", "--config", configPath})
	err := root.Execute()
	if err == nil {
		// If Docker is running, this might succeed — that's fine.
		// In most test environments, Docker isn't available, so we expect an error.
		return
	}
	// Should get past config loading and into the execution path
	// (not "not yet implemented" anymore).
	if strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("runLocal should no longer return 'not yet implemented', got: %v", err)
	}
	// When Docker is not installed the error mentions "docker"; when the
	// daemon IS reachable the pipeline runs but fails (e.g. image pull),
	// producing a "pipeline failed" or "execution" error instead.
	isDockerErr := strings.Contains(err.Error(), "Docker") || strings.Contains(err.Error(), "docker")
	isPipelineErr := strings.Contains(err.Error(), "pipeline") || strings.Contains(err.Error(), "execution")
	if !isDockerErr && !isPipelineErr {
		t.Errorf("expected Docker or pipeline-related error, got: %v", err)
	}
}

func TestRunCmd_RemoteConnectionError(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".seedee.yml")
	content := `pipeline:
  name: test-pipeline
  jobs:
    build:
      image: golang:1.22
      steps:
        - name: Build
          run: go build ./...
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}

	root := NewRootCmd()
	root.SetArgs([]string{"run", "--config", configPath, "--server", "localhost:8080"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for remote execution against unavailable server")
	}
	// Should get a connection-related error
	if !strings.Contains(err.Error(), "connect") && !strings.Contains(err.Error(), "unavailable") && !strings.Contains(err.Error(), "server") {
		t.Errorf("expected connection error for remote, got: %v", err)
	}
}
