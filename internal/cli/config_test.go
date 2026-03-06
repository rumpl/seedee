package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPipelineConfig_ValidFile(t *testing.T) {
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
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}

	cfg, err := loadPipelineConfig(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Pipeline.Name != "test-pipeline" {
		t.Errorf("expected pipeline name 'test-pipeline', got %q", cfg.Pipeline.Name)
	}
	if len(cfg.Pipeline.Jobs) != 1 {
		t.Errorf("expected 1 job, got %d", len(cfg.Pipeline.Jobs))
	}
	job, ok := cfg.Pipeline.Jobs["build"]
	if !ok {
		t.Fatal("expected 'build' job to exist")
	}
	if job.Image != "golang:1.22" {
		t.Errorf("expected image 'golang:1.22', got %q", job.Image)
	}
	if len(job.Steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(job.Steps))
	}
}

func TestLoadPipelineConfig_FileNotFound(t *testing.T) {
	_, err := loadPipelineConfig("/nonexistent/path/.seedee.yml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "config file not found") {
		t.Errorf("expected 'config file not found' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "--config") {
		t.Errorf("expected '--config' hint in error, got: %v", err)
	}
}

func TestLoadPipelineConfig_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".seedee.yml")
	content := `{{{not valid yaml:::}`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}

	_, err := loadPipelineConfig(configPath)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
	if !strings.Contains(err.Error(), "loading config") {
		t.Errorf("expected 'loading config' in error, got: %v", err)
	}
}

func TestLoadPipelineConfig_ValidationFailure(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".seedee.yml")
	// Valid YAML but missing required pipeline name
	content := `pipeline:
  jobs:
    build:
      image: golang:1.22
      steps:
        - run: go build ./...
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}

	_, err := loadPipelineConfig(configPath)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "pipeline name is required") {
		t.Errorf("expected 'pipeline name is required' in error, got: %v", err)
	}
}

func TestRunCmd_LoadsConfig(t *testing.T) {
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
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}

	root := NewRootCmd()
	root.SetArgs([]string{"run", "--config", configPath})
	err := root.Execute()
	if err == nil {
		// If Docker is available, this might succeed
		return
	}
	// Should get past config loading — should not return "not yet implemented"
	if strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("runLocal should no longer return 'not yet implemented', got: %v", err)
	}
}

func TestRunCmd_VerboseShowsPipeline(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".seedee.yml")
	content := `pipeline:
  name: verbose-test
  jobs:
    build:
      image: golang:1.22
      steps:
        - name: Build
          run: go build ./...
    test:
      image: golang:1.22
      depends_on:
        - build
      steps:
        - name: Test
          run: go test ./...
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}

	root := NewRootCmd()
	stderr := &bytes.Buffer{}
	root.SetErr(stderr)
	root.SetArgs([]string{"run", "--config", configPath, "--verbose"})
	_ = root.Execute() // will error with "not yet implemented", that's fine

	output := stderr.String()
	if !strings.Contains(output, "Pipeline: verbose-test") {
		t.Errorf("expected verbose output to contain pipeline name, got: %s", output)
	}
	if !strings.Contains(output, "2 jobs") {
		t.Errorf("expected verbose output to show '2 jobs', got: %s", output)
	}
	if !strings.Contains(output, "Job: build") {
		t.Errorf("expected verbose output to contain 'Job: build', got: %s", output)
	}
	if !strings.Contains(output, "Job: test") {
		t.Errorf("expected verbose output to contain 'Job: test', got: %s", output)
	}
}
