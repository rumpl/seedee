package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseConfig_ValidConfig(t *testing.T) {
	yaml := `
pipeline:
  name: my-app-ci
  env:
    GO_VERSION: "1.22"
  jobs:
    build:
      image: golang:1.22
      env:
        GOOS: linux
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
	cfg, err := ParseConfig([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Pipeline.Name != "my-app-ci" {
		t.Errorf("expected pipeline name 'my-app-ci', got %q", cfg.Pipeline.Name)
	}
	if cfg.Pipeline.Env["GO_VERSION"] != "1.22" {
		t.Errorf("expected GO_VERSION=1.22, got %q", cfg.Pipeline.Env["GO_VERSION"])
	}
	if len(cfg.Pipeline.Jobs) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(cfg.Pipeline.Jobs))
	}

	build := cfg.Pipeline.Jobs["build"]
	if build.Image != "golang:1.22" {
		t.Errorf("expected build image 'golang:1.22', got %q", build.Image)
	}
	if build.Env["GOOS"] != "linux" {
		t.Errorf("expected GOOS=linux, got %q", build.Env["GOOS"])
	}
	if len(build.Steps) != 1 {
		t.Errorf("expected 1 step in build, got %d", len(build.Steps))
	}
	if build.Steps[0].Name != "Build" {
		t.Errorf("expected step name 'Build', got %q", build.Steps[0].Name)
	}
	if build.Steps[0].Run != "go build ./..." {
		t.Errorf("expected step run 'go build ./...', got %q", build.Steps[0].Run)
	}

	test := cfg.Pipeline.Jobs["test"]
	if len(test.DependsOn) != 1 || test.DependsOn[0] != "build" {
		t.Errorf("expected test depends_on [build], got %v", test.DependsOn)
	}
}

func TestParseConfig_MissingPipelineName(t *testing.T) {
	yaml := `
pipeline:
  jobs:
    build:
      image: golang:1.22
      steps:
        - name: Build
          run: go build ./...
`
	_, err := ParseConfig([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for missing pipeline name")
	}
	if !strings.Contains(err.Error(), "pipeline name is required") {
		t.Errorf("expected error about pipeline name, got: %v", err)
	}
}

func TestParseConfig_EmptyJobs(t *testing.T) {
	yaml := `
pipeline:
  name: my-pipeline
  jobs: {}
`
	_, err := ParseConfig([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for empty jobs")
	}
	if !strings.Contains(err.Error(), "at least one job") {
		t.Errorf("expected error about empty jobs, got: %v", err)
	}
}

func TestParseConfig_NoJobs(t *testing.T) {
	yaml := `
pipeline:
  name: my-pipeline
`
	_, err := ParseConfig([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for missing jobs")
	}
	if !strings.Contains(err.Error(), "at least one job") {
		t.Errorf("expected error about missing jobs, got: %v", err)
	}
}

func TestParseConfig_JobNoImage(t *testing.T) {
	yaml := `
pipeline:
  name: my-pipeline
  jobs:
    build:
      steps:
        - name: Build
          run: go build ./...
`
	_, err := ParseConfig([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for job with no image")
	}
	if !strings.Contains(err.Error(), "has no image") {
		t.Errorf("expected error about missing image, got: %v", err)
	}
}

func TestParseConfig_JobNoSteps(t *testing.T) {
	yaml := `
pipeline:
  name: my-pipeline
  jobs:
    lint:
      image: golangci/golangci-lint:latest
`
	_, err := ParseConfig([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for job with no steps")
	}
	if !strings.Contains(err.Error(), `job "lint" has no steps`) {
		t.Errorf("expected error about no steps, got: %v", err)
	}
}

func TestParseConfig_StepNoRunCommand(t *testing.T) {
	yaml := `
pipeline:
  name: my-pipeline
  jobs:
    build:
      image: golang:1.22
      steps:
        - name: Build
`
	_, err := ParseConfig([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for step with no run command")
	}
	if !strings.Contains(err.Error(), "has no run command") {
		t.Errorf("expected error about missing run command, got: %v", err)
	}
}

func TestParseConfig_DependsOnNonExistentJob(t *testing.T) {
	yaml := `
pipeline:
  name: my-pipeline
  jobs:
    deploy:
      image: alpine:latest
      depends_on:
        - staging
      steps:
        - name: Deploy
          run: echo deploy
`
	_, err := ParseConfig([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for depends_on non-existent job")
	}
	if !strings.Contains(err.Error(), `depends on "staging" which does not exist`) {
		t.Errorf("expected error about non-existent dependency, got: %v", err)
	}
}

func TestParseConfig_DependencyCycle(t *testing.T) {
	yaml := `
pipeline:
  name: my-pipeline
  jobs:
    a:
      image: alpine:latest
      depends_on:
        - b
      steps:
        - name: Step A
          run: echo a
    b:
      image: alpine:latest
      depends_on:
        - a
      steps:
        - name: Step B
          run: echo b
`
	_, err := ParseConfig([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for dependency cycle")
	}
	if !strings.Contains(err.Error(), "dependency cycle detected") {
		t.Errorf("expected cycle error, got: %v", err)
	}
}

func TestParseConfig_DiamondDependency(t *testing.T) {
	yaml := `
pipeline:
  name: my-pipeline
  jobs:
    a:
      image: alpine:latest
      depends_on:
        - b
        - c
      steps:
        - name: Step A
          run: echo a
    b:
      image: alpine:latest
      depends_on:
        - d
      steps:
        - name: Step B
          run: echo b
    c:
      image: alpine:latest
      depends_on:
        - d
      steps:
        - name: Step C
          run: echo c
    d:
      image: alpine:latest
      steps:
        - name: Step D
          run: echo d
`
	cfg, err := ParseConfig([]byte(yaml))
	if err != nil {
		t.Fatalf("diamond dependency should be valid, got error: %v", err)
	}
	if len(cfg.Pipeline.Jobs) != 4 {
		t.Errorf("expected 4 jobs, got %d", len(cfg.Pipeline.Jobs))
	}
}

func TestParseConfig_EnvMerging(t *testing.T) {
	yaml := `
pipeline:
  name: my-pipeline
  env:
    GLOBAL: "1"
    SHARED: global
  jobs:
    build:
      image: golang:1.22
      env:
        JOB_VAR: "2"
        SHARED: job
      steps:
        - name: Build
          run: go build ./...
          env:
            STEP_VAR: "3"
            SHARED: step
`
	cfg, err := ParseConfig([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Pipeline-level env
	if cfg.Pipeline.Env["GLOBAL"] != "1" {
		t.Errorf("expected GLOBAL=1, got %q", cfg.Pipeline.Env["GLOBAL"])
	}
	if cfg.Pipeline.Env["SHARED"] != "global" {
		t.Errorf("expected pipeline SHARED=global, got %q", cfg.Pipeline.Env["SHARED"])
	}

	// Job-level env
	build := cfg.Pipeline.Jobs["build"]
	if build.Env["JOB_VAR"] != "2" {
		t.Errorf("expected JOB_VAR=2, got %q", build.Env["JOB_VAR"])
	}
	if build.Env["SHARED"] != "job" {
		t.Errorf("expected job SHARED=job, got %q", build.Env["SHARED"])
	}

	// Step-level env
	if build.Steps[0].Env["STEP_VAR"] != "3" {
		t.Errorf("expected STEP_VAR=3, got %q", build.Steps[0].Env["STEP_VAR"])
	}
	if build.Steps[0].Env["SHARED"] != "step" {
		t.Errorf("expected step SHARED=step, got %q", build.Steps[0].Env["SHARED"])
	}
}

func TestParseConfig_LargePipeline(t *testing.T) {
	// Generate a pipeline with 15 jobs, chained linearly
	jobs := ""
	for i := 0; i < 15; i++ {
		dep := ""
		if i > 0 {
			dep = fmt.Sprintf(`
      depends_on:
        - job%d`, i-1)
		}
		jobs += fmt.Sprintf(`
    job%d:
      image: alpine:latest%s
      steps:
        - name: Step %d
          run: echo job%d
`, i, dep, i, i)
	}

	yaml := fmt.Sprintf(`
pipeline:
  name: large-pipeline
  jobs:%s`, jobs)

	cfg, err := ParseConfig([]byte(yaml))
	if err != nil {
		t.Fatalf("large pipeline should parse fine, got error: %v", err)
	}
	if len(cfg.Pipeline.Jobs) != 15 {
		t.Errorf("expected 15 jobs, got %d", len(cfg.Pipeline.Jobs))
	}
}

func TestParseConfig_EmptyYAML(t *testing.T) {
	_, err := ParseConfig([]byte(""))
	if err == nil {
		t.Fatal("expected error for empty YAML")
	}
}

func TestParseConfig_MalformedYAML(t *testing.T) {
	yaml := `
pipeline:
  name: [invalid
  jobs:
    this is broken
`
	_, err := ParseConfig([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
	if !strings.Contains(err.Error(), "parsing config") {
		t.Errorf("expected parsing error, got: %v", err)
	}
}

func TestParseConfig_DuplicateStepNames(t *testing.T) {
	yaml := `
pipeline:
  name: my-pipeline
  jobs:
    build:
      image: golang:1.22
      steps:
        - name: same-name
          run: echo first
        - name: same-name
          run: echo second
`
	_, err := ParseConfig([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for duplicate step names")
	}
	if !strings.Contains(err.Error(), `duplicate step name "same-name"`) {
		t.Errorf("expected duplicate step name error, got: %v", err)
	}
}

func TestParseConfig_ThreeNodeCycle(t *testing.T) {
	yaml := `
pipeline:
  name: my-pipeline
  jobs:
    a:
      image: alpine:latest
      depends_on:
        - b
      steps:
        - name: Step A
          run: echo a
    b:
      image: alpine:latest
      depends_on:
        - c
      steps:
        - name: Step B
          run: echo b
    c:
      image: alpine:latest
      depends_on:
        - a
      steps:
        - name: Step C
          run: echo c
`
	_, err := ParseConfig([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for three-node dependency cycle")
	}
	if !strings.Contains(err.Error(), "dependency cycle detected") {
		t.Errorf("expected cycle error, got: %v", err)
	}
}

func TestParseConfig_SelfDependency(t *testing.T) {
	yaml := `
pipeline:
  name: my-pipeline
  jobs:
    a:
      image: alpine:latest
      depends_on:
        - a
      steps:
        - name: Step A
          run: echo a
`
	_, err := ParseConfig([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for self-dependency")
	}
	if !strings.Contains(err.Error(), "dependency cycle detected") {
		t.Errorf("expected cycle error, got: %v", err)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/.seedee.yml")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "reading config") {
		t.Errorf("expected reading error, got: %v", err)
	}
}

func TestLoadConfig_ValidFile(t *testing.T) {
	content := `
pipeline:
  name: file-test
  jobs:
    build:
      image: golang:1.22
      steps:
        - name: Build
          run: go build ./...
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, ".seedee.yml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Pipeline.Name != "file-test" {
		t.Errorf("expected pipeline name 'file-test', got %q", cfg.Pipeline.Name)
	}
}

func TestParseConfig_StepWithoutName(t *testing.T) {
	yaml := `
pipeline:
  name: my-pipeline
  jobs:
    build:
      image: golang:1.22
      steps:
        - run: go build ./...
        - run: go test ./...
`
	cfg, err := ParseConfig([]byte(yaml))
	if err != nil {
		t.Fatalf("steps without names should be valid, got error: %v", err)
	}
	if len(cfg.Pipeline.Jobs["build"].Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(cfg.Pipeline.Jobs["build"].Steps))
	}
}

func TestParseConfig_JobEmptySteps(t *testing.T) {
	yaml := `
pipeline:
  name: my-pipeline
  jobs:
    build:
      image: golang:1.22
      steps: []
`
	_, err := ParseConfig([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for empty steps list")
	}
	if !strings.Contains(err.Error(), "has no steps") {
		t.Errorf("expected no steps error, got: %v", err)
	}
}

func TestParseConfig_MultipleDependsOn(t *testing.T) {
	yaml := `
pipeline:
  name: my-pipeline
  jobs:
    build:
      image: alpine:latest
      steps:
        - name: Build
          run: echo build
    test:
      image: alpine:latest
      steps:
        - name: Test
          run: echo test
    deploy:
      image: alpine:latest
      depends_on:
        - build
        - test
      steps:
        - name: Deploy
          run: echo deploy
`
	cfg, err := ParseConfig([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	deploy := cfg.Pipeline.Jobs["deploy"]
	if len(deploy.DependsOn) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(deploy.DependsOn))
	}
}
