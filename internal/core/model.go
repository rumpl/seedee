// Package core contains the pipeline model, DAG scheduler, and execution engine.
package core

import "time"

// PipelineConfig is the top-level config parsed from .seedee.yml
type PipelineConfig struct {
	Pipeline PipelineDef `yaml:"pipeline"`
}

// PipelineDef describes the full pipeline: its name, global env, and jobs.
type PipelineDef struct {
	Name string            `yaml:"name"`
	Env  map[string]string `yaml:"env,omitempty"`
	Jobs map[string]JobDef `yaml:"jobs"`
}

// JobDef describes a single job within the pipeline.
type JobDef struct {
	Image     string            `yaml:"image"`
	DependsOn []string          `yaml:"depends_on,omitempty"`
	Env       map[string]string `yaml:"env,omitempty"`
	Steps     []StepDef         `yaml:"steps"`
}

// StepDef describes a single step within a job.
type StepDef struct {
	Name string            `yaml:"name"`
	Run  string            `yaml:"run"`
	Env  map[string]string `yaml:"env,omitempty"`
}

// Status represents the execution state of a job or step.
type Status string

const (
	StatusPending  Status = "pending"
	StatusRunning  Status = "running"
	StatusSuccess  Status = "success"
	StatusFailed   Status = "failed"
	StatusSkipped  Status = "skipped"
	StatusCanceled Status = "canceled"
)

// Pipeline is the runtime representation of a pipeline execution.
type Pipeline struct {
	ID        string
	Name      string
	Jobs      []*Job
	Env       map[string]string
	Status    Status
	StartedAt time.Time
	EndedAt   time.Time
	Error     error
}

// Job is the runtime representation of a single job.
type Job struct {
	Name      string
	Image     string
	Steps     []*Step
	DependsOn []string          // names of jobs this depends on
	Env       map[string]string // merged: pipeline env + job env
	Status    Status
	StartedAt time.Time
	EndedAt   time.Time
	Error     error
}

// Step is the runtime representation of a single step within a job.
type Step struct {
	Name      string
	Command   string            // the shell command to run
	Env       map[string]string // merged: pipeline env + job env + step env
	Status    Status
	StartedAt time.Time
	EndedAt   time.Time
	ExitCode  int
	Error     error
}

// StepResult is returned by the Runner after executing a step.
type StepResult struct {
	ExitCode int
	Error    error
}

// JobResult summarizes the outcome of a job.
type JobResult struct {
	JobName string
	Status  Status
	Steps   []StepResult
	Error   error
}

// PipelineResult summarizes the outcome of the entire pipeline.
type PipelineResult struct {
	PipelineID string
	Status     Status
	Jobs       []JobResult
	Duration   time.Duration
	Error      error
}
