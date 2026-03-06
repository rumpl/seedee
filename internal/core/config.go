package core

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadConfig reads and parses a pipeline config from the given file path.
func LoadConfig(path string) (*PipelineConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}
	return ParseConfig(data)
}

// ParseConfig parses raw YAML bytes into a validated PipelineConfig.
func ParseConfig(data []byte) (*PipelineConfig, error) {
	var cfg PipelineConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Validate checks the PipelineConfig for structural correctness.
func (c *PipelineConfig) Validate() error {
	if c.Pipeline.Name == "" {
		return fmt.Errorf("config error: pipeline name is required")
	}

	if len(c.Pipeline.Jobs) == 0 {
		return fmt.Errorf("config error: pipeline %q must have at least one job", c.Pipeline.Name)
	}

	for jobName, job := range c.Pipeline.Jobs {
		if job.Image == "" {
			return fmt.Errorf("config error: job %q has no image", jobName)
		}

		if len(job.Steps) == 0 {
			return fmt.Errorf("config error: job %q has no steps", jobName)
		}

		stepNames := make(map[string]bool)
		for i, step := range job.Steps {
			if step.Run == "" {
				return fmt.Errorf("config error: step %d in job %q has no run command", i, jobName)
			}
			if step.Name != "" {
				if stepNames[step.Name] {
					return fmt.Errorf("config error: duplicate step name %q in job %q", step.Name, jobName)
				}
				stepNames[step.Name] = true
			}
		}

		for _, dep := range job.DependsOn {
			if _, exists := c.Pipeline.Jobs[dep]; !exists {
				return fmt.Errorf("config error: job %q depends on %q which does not exist", jobName, dep)
			}
		}
	}

	if err := c.detectCycles(); err != nil {
		return err
	}

	return nil
}

// detectCycles uses DFS with visited/in-stack tracking to find dependency cycles.
func (c *PipelineConfig) detectCycles() error {
	const (
		unvisited = iota
		inStack
		visited
	)

	state := make(map[string]int)
	parent := make(map[string]string)

	var dfs func(job string) error
	dfs = func(job string) error {
		state[job] = inStack

		jobDef := c.Pipeline.Jobs[job]
		for _, dep := range jobDef.DependsOn {
			switch state[dep] {
			case inStack:
				// Build the cycle path
				cycle := []string{dep, job}
				cur := job
				for cur != dep {
					cur = parent[cur]
					if cur == dep {
						break
					}
					cycle = append(cycle, cur)
				}
				// Reverse to get the correct order
				for i, j := 0, len(cycle)-1; i < j; i, j = i+1, j-1 {
					cycle[i], cycle[j] = cycle[j], cycle[i]
				}
				return fmt.Errorf("config error: dependency cycle detected: %s", strings.Join(cycle, " -> "))
			case unvisited:
				parent[dep] = job
				if err := dfs(dep); err != nil {
					return err
				}
			}
		}

		state[job] = visited
		return nil
	}

	for jobName := range c.Pipeline.Jobs {
		if state[jobName] == unvisited {
			if err := dfs(jobName); err != nil {
				return err
			}
		}
	}

	return nil
}
