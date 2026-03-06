package core

import (
	"sort"

	"github.com/google/uuid"
)

// NewPipelineFromConfig converts a parsed PipelineConfig into a runtime Pipeline.
// It merges environment variables and generates a unique pipeline ID.
func NewPipelineFromConfig(cfg *PipelineConfig) (*Pipeline, error) {
	pipelineID := uuid.New().String()

	// Sort job names for deterministic ordering
	jobNames := make([]string, 0, len(cfg.Pipeline.Jobs))
	for name := range cfg.Pipeline.Jobs {
		jobNames = append(jobNames, name)
	}
	sort.Strings(jobNames)

	pipelineEnv := cfg.Pipeline.Env
	if pipelineEnv == nil {
		pipelineEnv = make(map[string]string)
	}

	jobs := make([]*Job, 0, len(jobNames))
	for _, jobName := range jobNames {
		jobDef := cfg.Pipeline.Jobs[jobName]

		// Merge pipeline env with job env (job wins)
		jobEnv := mergeEnv(pipelineEnv, jobDef.Env)

		steps := make([]*Step, 0, len(jobDef.Steps))
		for _, stepDef := range jobDef.Steps {
			// Merge job env with step env (step wins)
			stepEnv := mergeEnv(jobEnv, stepDef.Env)

			steps = append(steps, &Step{
				Name:    stepDef.Name,
				Command: stepDef.Run,
				Env:     stepEnv,
				Status:  StatusPending,
			})
		}

		jobs = append(jobs, &Job{
			Name:      jobName,
			Image:     jobDef.Image,
			Steps:     steps,
			DependsOn: jobDef.DependsOn,
			Env:       jobEnv,
			Status:    StatusPending,
		})
	}

	return &Pipeline{
		ID:     pipelineID,
		Name:   cfg.Pipeline.Name,
		Jobs:   jobs,
		Env:    pipelineEnv,
		Status: StatusPending,
	}, nil
}

// mergeEnv merges two env maps. Values in override take precedence.
// Returns a new map; never returns nil.
func mergeEnv(base, override map[string]string) map[string]string {
	merged := make(map[string]string, len(base)+len(override))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range override {
		merged[k] = v
	}
	return merged
}
