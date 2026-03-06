// Package core contains the pipeline model, DAG scheduler, and execution engine.
package core

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
