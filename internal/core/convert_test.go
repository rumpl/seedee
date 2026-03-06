package core

import (
	"testing"
)

func TestNewPipelineFromConfig_Simple(t *testing.T) {
	cfg := &PipelineConfig{
		Pipeline: PipelineDef{
			Name: "test-pipeline",
			Jobs: map[string]JobDef{
				"build": {
					Image: "golang:1.22",
					Steps: []StepDef{
						{Name: "compile", Run: "go build ./..."},
						{Name: "vet", Run: "go vet ./..."},
					},
				},
				"test": {
					Image: "golang:1.22",
					DependsOn: []string{"build"},
					Steps: []StepDef{
						{Name: "unit", Run: "go test ./..."},
					},
				},
			},
		},
	}

	p, err := NewPipelineFromConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.Name != "test-pipeline" {
		t.Errorf("expected pipeline name 'test-pipeline', got %q", p.Name)
	}
	if len(p.Jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(p.Jobs))
	}

	// Jobs should be sorted alphabetically
	if p.Jobs[0].Name != "build" {
		t.Errorf("expected first job 'build', got %q", p.Jobs[0].Name)
	}
	if p.Jobs[1].Name != "test" {
		t.Errorf("expected second job 'test', got %q", p.Jobs[1].Name)
	}

	// Check build job
	buildJob := p.Jobs[0]
	if buildJob.Image != "golang:1.22" {
		t.Errorf("expected image 'golang:1.22', got %q", buildJob.Image)
	}
	if len(buildJob.Steps) != 2 {
		t.Fatalf("expected 2 steps in build, got %d", len(buildJob.Steps))
	}
	if buildJob.Steps[0].Name != "compile" {
		t.Errorf("expected step name 'compile', got %q", buildJob.Steps[0].Name)
	}
	if buildJob.Steps[0].Command != "go build ./..." {
		t.Errorf("expected command 'go build ./...', got %q", buildJob.Steps[0].Command)
	}

	// Check test job depends_on
	testJob := p.Jobs[1]
	if len(testJob.DependsOn) != 1 || testJob.DependsOn[0] != "build" {
		t.Errorf("expected depends_on [build], got %v", testJob.DependsOn)
	}
}

func TestNewPipelineFromConfig_EnvMerging(t *testing.T) {
	cfg := &PipelineConfig{
		Pipeline: PipelineDef{
			Name: "env-test",
			Env: map[string]string{
				"GLOBAL":    "from-pipeline",
				"SHARED":    "pipeline-value",
				"PIPE_ONLY": "pipe",
			},
			Jobs: map[string]JobDef{
				"build": {
					Image: "alpine:latest",
					Env: map[string]string{
						"SHARED":   "job-value",
						"JOB_ONLY": "job",
					},
					Steps: []StepDef{
						{
							Name: "step1",
							Run:  "echo hello",
							Env: map[string]string{
								"SHARED":    "step-value",
								"STEP_ONLY": "step",
							},
						},
					},
				},
			},
		},
	}

	p, err := NewPipelineFromConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	job := p.Jobs[0]

	// Job env should have pipeline env merged with job env (job wins)
	if job.Env["GLOBAL"] != "from-pipeline" {
		t.Errorf("expected job GLOBAL='from-pipeline', got %q", job.Env["GLOBAL"])
	}
	if job.Env["SHARED"] != "job-value" {
		t.Errorf("expected job SHARED='job-value', got %q", job.Env["SHARED"])
	}
	if job.Env["PIPE_ONLY"] != "pipe" {
		t.Errorf("expected job PIPE_ONLY='pipe', got %q", job.Env["PIPE_ONLY"])
	}
	if job.Env["JOB_ONLY"] != "job" {
		t.Errorf("expected job JOB_ONLY='job', got %q", job.Env["JOB_ONLY"])
	}

	// Step env should have job env merged with step env (step wins)
	step := job.Steps[0]
	if step.Env["GLOBAL"] != "from-pipeline" {
		t.Errorf("expected step GLOBAL='from-pipeline', got %q", step.Env["GLOBAL"])
	}
	if step.Env["SHARED"] != "step-value" {
		t.Errorf("expected step SHARED='step-value', got %q", step.Env["SHARED"])
	}
	if step.Env["PIPE_ONLY"] != "pipe" {
		t.Errorf("expected step PIPE_ONLY='pipe', got %q", step.Env["PIPE_ONLY"])
	}
	if step.Env["JOB_ONLY"] != "job" {
		t.Errorf("expected step JOB_ONLY='job', got %q", step.Env["JOB_ONLY"])
	}
	if step.Env["STEP_ONLY"] != "step" {
		t.Errorf("expected step STEP_ONLY='step', got %q", step.Env["STEP_ONLY"])
	}
}

func TestNewPipelineFromConfig_AllStatusesPending(t *testing.T) {
	cfg := &PipelineConfig{
		Pipeline: PipelineDef{
			Name: "status-test",
			Jobs: map[string]JobDef{
				"a": {
					Image: "alpine:latest",
					Steps: []StepDef{
						{Name: "s1", Run: "echo a"},
						{Name: "s2", Run: "echo b"},
					},
				},
			},
		},
	}

	p, err := NewPipelineFromConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.Status != StatusPending {
		t.Errorf("expected pipeline status %q, got %q", StatusPending, p.Status)
	}

	for _, job := range p.Jobs {
		if job.Status != StatusPending {
			t.Errorf("expected job %q status %q, got %q", job.Name, StatusPending, job.Status)
		}
		for _, step := range job.Steps {
			if step.Status != StatusPending {
				t.Errorf("expected step %q status %q, got %q", step.Name, StatusPending, step.Status)
			}
		}
	}
}

func TestNewPipelineFromConfig_UniqueIDs(t *testing.T) {
	cfg := &PipelineConfig{
		Pipeline: PipelineDef{
			Name: "id-test",
			Jobs: map[string]JobDef{
				"build": {
					Image: "alpine:latest",
					Steps: []StepDef{
						{Name: "step", Run: "echo hi"},
					},
				},
			},
		},
	}

	p1, err := NewPipelineFromConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p2, err := NewPipelineFromConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p1.ID == "" {
		t.Error("expected non-empty pipeline ID")
	}
	if p1.ID == p2.ID {
		t.Errorf("expected unique IDs, both got %q", p1.ID)
	}
}

func TestNewPipelineFromConfig_DeterministicJobOrder(t *testing.T) {
	cfg := &PipelineConfig{
		Pipeline: PipelineDef{
			Name: "order-test",
			Jobs: map[string]JobDef{
				"zebra": {
					Image: "alpine:latest",
					Steps: []StepDef{{Name: "s", Run: "echo z"}},
				},
				"apple": {
					Image: "alpine:latest",
					Steps: []StepDef{{Name: "s", Run: "echo a"}},
				},
				"mango": {
					Image: "alpine:latest",
					Steps: []StepDef{{Name: "s", Run: "echo m"}},
				},
			},
		},
	}

	// Run multiple times to verify determinism
	for i := 0; i < 10; i++ {
		p, err := NewPipelineFromConfig(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.Jobs[0].Name != "apple" {
			t.Errorf("iteration %d: expected first job 'apple', got %q", i, p.Jobs[0].Name)
		}
		if p.Jobs[1].Name != "mango" {
			t.Errorf("iteration %d: expected second job 'mango', got %q", i, p.Jobs[1].Name)
		}
		if p.Jobs[2].Name != "zebra" {
			t.Errorf("iteration %d: expected third job 'zebra', got %q", i, p.Jobs[2].Name)
		}
	}
}

func TestNewPipelineFromConfig_EmptyEnvMaps(t *testing.T) {
	cfg := &PipelineConfig{
		Pipeline: PipelineDef{
			Name: "empty-env-test",
			Jobs: map[string]JobDef{
				"build": {
					Image: "alpine:latest",
					Steps: []StepDef{
						{Name: "step", Run: "echo hi"},
					},
				},
			},
		},
	}

	p, err := NewPipelineFromConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Pipeline env should not be nil
	if p.Env == nil {
		t.Error("expected non-nil pipeline env")
	}

	// Job env should not be nil
	if p.Jobs[0].Env == nil {
		t.Error("expected non-nil job env")
	}

	// Step env should not be nil
	if p.Jobs[0].Steps[0].Env == nil {
		t.Error("expected non-nil step env")
	}
}

func TestMergeEnv(t *testing.T) {
	tests := []struct {
		name     string
		base     map[string]string
		override map[string]string
		expected map[string]string
	}{
		{
			name:     "nil base and override",
			base:     nil,
			override: nil,
			expected: map[string]string{},
		},
		{
			name:     "nil base",
			base:     nil,
			override: map[string]string{"A": "1"},
			expected: map[string]string{"A": "1"},
		},
		{
			name:     "nil override",
			base:     map[string]string{"A": "1"},
			override: nil,
			expected: map[string]string{"A": "1"},
		},
		{
			name:     "override wins",
			base:     map[string]string{"A": "base", "B": "base"},
			override: map[string]string{"A": "override"},
			expected: map[string]string{"A": "override", "B": "base"},
		},
		{
			name:     "disjoint maps",
			base:     map[string]string{"A": "1"},
			override: map[string]string{"B": "2"},
			expected: map[string]string{"A": "1", "B": "2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeEnv(tt.base, tt.override)
			if result == nil {
				t.Fatal("mergeEnv returned nil")
			}
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d entries, got %d", len(tt.expected), len(result))
			}
			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("expected %s=%q, got %q", k, v, result[k])
				}
			}
		})
	}
}
