// Package core contains the pipeline model, DAG scheduler, and execution engine.
package core

import (
	"fmt"
	"sort"
)

// ExecutionGroup is a set of jobs that can all run in parallel.
type ExecutionGroup struct {
	Jobs []*Job
}

// Schedule takes a pipeline and returns an ordered list of execution groups.
// Jobs within the same group have no dependencies on each other and can run concurrently.
// Uses Kahn's algorithm (BFS topological sort) to produce level-grouped output.
func Schedule(p *Pipeline) ([]ExecutionGroup, error) {
	// 1. Build a map of jobName -> *Job for quick lookup
	jobMap := make(map[string]*Job, len(p.Jobs))
	for _, job := range p.Jobs {
		jobMap[job.Name] = job
	}

	// 2. Validate all depends_on references exist
	for _, job := range p.Jobs {
		for _, dep := range job.DependsOn {
			if _, ok := jobMap[dep]; !ok {
				return nil, fmt.Errorf("job %q depends on unknown job %q", job.Name, dep)
			}
		}
	}

	// 3. Compute in-degree for each job
	inDegree := make(map[string]int, len(p.Jobs))
	for _, job := range p.Jobs {
		// Ensure every job has an entry, even if in-degree is 0
		if _, ok := inDegree[job.Name]; !ok {
			inDegree[job.Name] = 0
		}
		for _, dep := range job.DependsOn {
			inDegree[job.Name]++
			// Ensure dependency also has an entry
			if _, ok := inDegree[dep]; !ok {
				inDegree[dep] = 0
			}
			_ = dep // dep already validated above
		}
	}

	// 4. Kahn's algorithm collecting levels
	var groups []ExecutionGroup
	scheduled := 0
	total := len(p.Jobs)

	for scheduled < total {
		// Collect all jobs with in-degree 0
		var ready []string
		for _, job := range p.Jobs {
			if deg, ok := inDegree[job.Name]; ok && deg == 0 {
				ready = append(ready, job.Name)
			}
		}

		if len(ready) == 0 {
			// Cycle detected — find and report the cycle path
			return nil, findCycle(p.Jobs)
		}

		// Sort for deterministic ordering
		sort.Strings(ready)

		group := ExecutionGroup{}
		for _, name := range ready {
			group.Jobs = append(group.Jobs, jobMap[name])
			// Remove this node from the graph
			delete(inDegree, name)
			scheduled++
		}

		// Decrement in-degrees for dependents of the removed nodes
		for _, job := range p.Jobs {
			if _, removed := inDegree[job.Name]; !removed {
				continue // already scheduled
			}
			for _, dep := range job.DependsOn {
				for _, name := range ready {
					if dep == name {
						inDegree[job.Name]--
					}
				}
			}
		}

		groups = append(groups, group)
	}

	return groups, nil
}

// findCycle performs DFS to find an actual cycle path among the remaining jobs
// and returns an error with a human-readable cycle description.
func findCycle(jobs []*Job) error {
	// Build adjacency: job -> jobs that depend on it (forward edges from dependency to dependent)
	// But for cycle detection we want: job -> its dependencies
	depMap := make(map[string][]string)
	nameSet := make(map[string]bool)
	for _, job := range jobs {
		nameSet[job.Name] = true
		depMap[job.Name] = job.DependsOn
	}

	const (
		white = 0 // unvisited
		gray  = 1 // in current recursion stack
		black = 2 // fully processed
	)

	color := make(map[string]int)
	parent := make(map[string]string)

	// Sort job names for deterministic cycle reporting
	var names []string
	for _, job := range jobs {
		names = append(names, job.Name)
	}
	sort.Strings(names)

	var dfs func(node string) (string, bool)
	dfs = func(node string) (string, bool) {
		color[node] = gray
		deps := depMap[node]
		// Sort deps for deterministic traversal
		sorted := make([]string, len(deps))
		copy(sorted, deps)
		sort.Strings(sorted)

		for _, dep := range sorted {
			if color[dep] == gray {
				// Found cycle: reconstruct path
				// dep is already in the recursion stack, and node -> dep closes the cycle
				parent[dep] = node
				return dep, true
			}
			if color[dep] == white {
				parent[dep] = node
				if cycleStart, found := dfs(dep); found {
					return cycleStart, true
				}
			}
		}
		color[node] = black
		return "", false
	}

	for _, name := range names {
		if color[name] == white {
			if cycleStart, found := dfs(name); found {
				// Reconstruct the cycle path
				path := []string{cycleStart}
				cur := parent[cycleStart]
				for cur != cycleStart {
					path = append(path, cur)
					cur = parent[cur]
				}
				path = append(path, cycleStart)
				// Reverse to get the cycle in forward order
				for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
					path[i], path[j] = path[j], path[i]
				}
				return fmt.Errorf("dependency cycle detected: %s", formatCyclePath(path))
			}
		}
	}

	return fmt.Errorf("dependency cycle detected")
}

// formatCyclePath formats a cycle path as "A -> B -> C -> A"
func formatCyclePath(path []string) string {
	result := ""
	for i, name := range path {
		if i > 0 {
			result += " -> "
		}
		result += name
	}
	return result
}
