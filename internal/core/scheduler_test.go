package core

import (
	"strings"
	"testing"
)

// helper to build a pipeline from a simple map of name -> depends_on
func buildPipeline(jobs map[string][]string) *Pipeline {
	p := &Pipeline{
		ID:   "test-pipeline",
		Name: "test",
	}
	for name, deps := range jobs {
		j := &Job{
			Name:      name,
			DependsOn: deps,
		}
		p.Jobs = append(p.Jobs, j)
	}
	return p
}

// helper to extract job names from execution groups
func groupNames(groups []ExecutionGroup) [][]string {
	var result [][]string
	for _, g := range groups {
		var names []string
		for _, j := range g.Jobs {
			names = append(names, j.Name)
		}
		result = append(result, names)
	}
	return result
}

func TestSchedule_AllParallel(t *testing.T) {
	p := buildPipeline(map[string][]string{
		"lint":  nil,
		"test":  nil,
		"build": nil,
	})

	groups, err := Schedule(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}

	if len(groups[0].Jobs) != 3 {
		t.Fatalf("expected 3 jobs in group 0, got %d", len(groups[0].Jobs))
	}

	names := groupNames(groups)
	// Should be alphabetically sorted
	expected := []string{"build", "lint", "test"}
	for i, name := range names[0] {
		if name != expected[i] {
			t.Errorf("group 0 job %d: expected %q, got %q", i, expected[i], name)
		}
	}
}

func TestSchedule_LinearChain(t *testing.T) {
	p := buildPipeline(map[string][]string{
		"A": nil,
		"B": {"A"},
		"C": {"B"},
	})

	groups, err := Schedule(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}

	names := groupNames(groups)
	expected := [][]string{{"A"}, {"B"}, {"C"}}
	for i, g := range names {
		if len(g) != 1 || g[0] != expected[i][0] {
			t.Errorf("group %d: expected %v, got %v", i, expected[i], g)
		}
	}
}

func TestSchedule_Diamond(t *testing.T) {
	p := buildPipeline(map[string][]string{
		"A": nil,
		"B": {"A"},
		"C": {"A"},
		"D": {"B", "C"},
	})

	groups, err := Schedule(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}

	names := groupNames(groups)
	// Group 0: [A]
	if len(names[0]) != 1 || names[0][0] != "A" {
		t.Errorf("group 0: expected [A], got %v", names[0])
	}
	// Group 1: [B, C] (alphabetically sorted)
	if len(names[1]) != 2 || names[1][0] != "B" || names[1][1] != "C" {
		t.Errorf("group 1: expected [B C], got %v", names[1])
	}
	// Group 2: [D]
	if len(names[2]) != 1 || names[2][0] != "D" {
		t.Errorf("group 2: expected [D], got %v", names[2])
	}
}

func TestSchedule_ComplexDAG(t *testing.T) {
	// A→B,C; B→D; C→E; D,E→F
	p := buildPipeline(map[string][]string{
		"A": nil,
		"B": {"A"},
		"C": {"A"},
		"D": {"B"},
		"E": {"C"},
		"F": {"D", "E"},
	})

	groups, err := Schedule(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(groups) != 4 {
		t.Fatalf("expected 4 groups, got %d", len(groups))
	}

	names := groupNames(groups)
	// [A], [B,C], [D,E], [F]
	if len(names[0]) != 1 || names[0][0] != "A" {
		t.Errorf("group 0: expected [A], got %v", names[0])
	}
	if len(names[1]) != 2 || names[1][0] != "B" || names[1][1] != "C" {
		t.Errorf("group 1: expected [B C], got %v", names[1])
	}
	if len(names[2]) != 2 || names[2][0] != "D" || names[2][1] != "E" {
		t.Errorf("group 2: expected [D E], got %v", names[2])
	}
	if len(names[3]) != 1 || names[3][0] != "F" {
		t.Errorf("group 3: expected [F], got %v", names[3])
	}
}

func TestSchedule_MultipleRoots(t *testing.T) {
	p := buildPipeline(map[string][]string{
		"A": nil,
		"B": nil,
		"C": {"A"},
		"D": {"B"},
	})

	groups, err := Schedule(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}

	names := groupNames(groups)
	// Group 0: [A, B]
	if len(names[0]) != 2 || names[0][0] != "A" || names[0][1] != "B" {
		t.Errorf("group 0: expected [A B], got %v", names[0])
	}
	// Group 1: [C, D]
	if len(names[1]) != 2 || names[1][0] != "C" || names[1][1] != "D" {
		t.Errorf("group 1: expected [C D], got %v", names[1])
	}
}

func TestSchedule_CycleDetection(t *testing.T) {
	p := buildPipeline(map[string][]string{
		"A": {"C"},
		"B": {"A"},
		"C": {"B"},
	})

	_, err := Schedule(p)
	if err == nil {
		t.Fatal("expected error for cycle, got nil")
	}

	if !strings.Contains(err.Error(), "dependency cycle detected") {
		t.Errorf("expected cycle error, got: %v", err)
	}

	// Verify the error contains a cycle path with arrows
	if !strings.Contains(err.Error(), "->") {
		t.Errorf("expected cycle path with arrows, got: %v", err)
	}
}

func TestSchedule_SelfDependency(t *testing.T) {
	p := buildPipeline(map[string][]string{
		"A": {"A"},
	})

	_, err := Schedule(p)
	if err == nil {
		t.Fatal("expected error for self-dependency, got nil")
	}

	if !strings.Contains(err.Error(), "dependency cycle detected") {
		t.Errorf("expected cycle error, got: %v", err)
	}
}

func TestSchedule_MissingDependency(t *testing.T) {
	p := buildPipeline(map[string][]string{
		"A": {"ghost"},
	})

	_, err := Schedule(p)
	if err == nil {
		t.Fatal("expected error for missing dependency, got nil")
	}

	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("expected error mentioning 'ghost', got: %v", err)
	}

	if !strings.Contains(err.Error(), "unknown job") {
		t.Errorf("expected error mentioning 'unknown job', got: %v", err)
	}
}

func TestSchedule_SingleJob(t *testing.T) {
	p := buildPipeline(map[string][]string{
		"only": nil,
	})

	groups, err := Schedule(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}

	if len(groups[0].Jobs) != 1 {
		t.Fatalf("expected 1 job in group, got %d", len(groups[0].Jobs))
	}

	if groups[0].Jobs[0].Name != "only" {
		t.Errorf("expected job name 'only', got %q", groups[0].Jobs[0].Name)
	}
}

func TestSchedule_DeterministicOrder(t *testing.T) {
	// Run multiple times to verify determinism
	for i := 0; i < 10; i++ {
		p := buildPipeline(map[string][]string{
			"zebra":    nil,
			"alpha":    nil,
			"middle":   nil,
			"beta":     {"alpha"},
			"gamma":    {"alpha"},
			"delta":    {"beta", "gamma"},
		})

		groups, err := Schedule(p)
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}

		names := groupNames(groups)

		// Group 0 should be alphabetically sorted
		if len(names[0]) != 3 || names[0][0] != "alpha" || names[0][1] != "middle" || names[0][2] != "zebra" {
			t.Errorf("iteration %d: group 0 not sorted: %v", i, names[0])
		}

		// Group 1
		if len(names[1]) != 2 || names[1][0] != "beta" || names[1][1] != "gamma" {
			t.Errorf("iteration %d: group 1 not sorted: %v", i, names[1])
		}

		// Group 2
		if len(names[2]) != 1 || names[2][0] != "delta" {
			t.Errorf("iteration %d: group 2 unexpected: %v", i, names[2])
		}
	}
}
