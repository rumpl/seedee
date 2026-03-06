package core

import (
	"testing"
)

func TestStatusConstants(t *testing.T) {
	tests := []struct {
		status   Status
		expected string
	}{
		{StatusPending, "pending"},
		{StatusRunning, "running"},
		{StatusSuccess, "success"},
		{StatusFailed, "failed"},
		{StatusSkipped, "skipped"},
		{StatusCanceled, "canceled"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if string(tt.status) != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, string(tt.status))
			}
		})
	}
}

func TestPipelineStatusTransitions(t *testing.T) {
	p := &Pipeline{
		ID:     "test-id",
		Name:   "test",
		Status: StatusPending,
	}

	// Pending -> Running
	if p.Status != StatusPending {
		t.Errorf("expected initial status %q, got %q", StatusPending, p.Status)
	}

	p.Status = StatusRunning
	if p.Status != StatusRunning {
		t.Errorf("expected status %q, got %q", StatusRunning, p.Status)
	}

	// Running -> Success
	p.Status = StatusSuccess
	if p.Status != StatusSuccess {
		t.Errorf("expected status %q, got %q", StatusSuccess, p.Status)
	}

	// Test Running -> Failed path
	p2 := &Pipeline{
		ID:     "test-id-2",
		Name:   "test-2",
		Status: StatusRunning,
	}
	p2.Status = StatusFailed
	if p2.Status != StatusFailed {
		t.Errorf("expected status %q, got %q", StatusFailed, p2.Status)
	}

	// Test Pending -> Canceled path
	p3 := &Pipeline{
		ID:     "test-id-3",
		Name:   "test-3",
		Status: StatusPending,
	}
	p3.Status = StatusCanceled
	if p3.Status != StatusCanceled {
		t.Errorf("expected status %q, got %q", StatusCanceled, p3.Status)
	}
}

func TestStatusAsString(t *testing.T) {
	// Verify Status type can be used as a string
	s := StatusRunning
	got := string(s)
	if got != "running" {
		t.Errorf("expected 'running', got %q", got)
	}
}
