package docker

import (
	"testing"

	"github.com/rumpl/seedee/internal/runner"
)

// TestDockerRunnerImplementsRunner is a compile-time check that DockerRunner
// implements the runner.Runner interface.
func TestDockerRunnerImplementsRunner(t *testing.T) {
	var _ runner.Runner = (*DockerRunner)(nil)
}

func TestNewDockerRunner(t *testing.T) {
	r := NewDockerRunner(nil)
	if r == nil {
		t.Fatal("expected non-nil runner")
	}
	if r.volumes == nil {
		t.Fatal("expected non-nil volumes map")
	}
	if len(r.volumes) != 0 {
		t.Fatalf("expected empty volumes map, got %d entries", len(r.volumes))
	}
}

func TestRandomSuffix_Length(t *testing.T) {
	s := randomSuffix()
	// 4 bytes = 8 hex characters
	if len(s) != 8 {
		t.Fatalf("expected 8 hex chars, got %d: %q", len(s), s)
	}
}

func TestRandomSuffix_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for range 100 {
		s := randomSuffix()
		if seen[s] {
			t.Fatalf("duplicate suffix %q after <100 calls", s)
		}
		seen[s] = true
	}
}

func TestRandomSuffix_IsHex(t *testing.T) {
	s := randomSuffix()
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Fatalf("unexpected character %c in suffix %q", c, s)
		}
	}
}
