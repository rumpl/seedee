package cli

import (
	"bytes"
	"testing"
	"time"

	"github.com/rumpl/seedee/internal/core"
)

func TestPrintPipelineSummary(t *testing.T) {
	buf := &bytes.Buffer{}

	result := &core.PipelineResult{
		PipelineID: "test-1",
		Status:     core.StatusFailed,
		Duration:   5 * time.Second,
		Jobs: []core.JobResult{
			{JobName: "build", Status: core.StatusSuccess},
			{JobName: "test", Status: core.StatusFailed},
		},
	}

	printPipelineSummary(buf, result)

	output := buf.String()
	if !contains(output, "Pipeline Summary") {
		t.Errorf("expected 'Pipeline Summary' in output, got: %s", output)
	}
	if !contains(output, "failed") {
		t.Errorf("expected 'failed' status in output, got: %s", output)
	}
	if !contains(output, "5s") {
		t.Errorf("expected '5s' duration in output, got: %s", output)
	}
	if !contains(output, "build") {
		t.Errorf("expected 'build' job in output, got: %s", output)
	}
	if !contains(output, "test") {
		t.Errorf("expected 'test' job in output, got: %s", output)
	}
	if !contains(output, "✓") {
		t.Errorf("expected success icon '✓' in output, got: %s", output)
	}
	if !contains(output, "✗") {
		t.Errorf("expected failure icon '✗' in output, got: %s", output)
	}
}

func TestPrintPipelineSummary_AllSuccess(t *testing.T) {
	buf := &bytes.Buffer{}

	result := &core.PipelineResult{
		PipelineID: "test-2",
		Status:     core.StatusSuccess,
		Duration:   2*time.Second + 500*time.Millisecond,
		Jobs: []core.JobResult{
			{JobName: "build", Status: core.StatusSuccess},
			{JobName: "test", Status: core.StatusSuccess},
			{JobName: "deploy", Status: core.StatusSuccess},
		},
	}

	printPipelineSummary(buf, result)

	output := buf.String()
	if !contains(output, "success") {
		t.Errorf("expected 'success' status in output, got: %s", output)
	}
	if !contains(output, "2.5s") {
		t.Errorf("expected '2.5s' duration in output, got: %s", output)
	}
}

func TestPrintPipelineSummary_WithSkipped(t *testing.T) {
	buf := &bytes.Buffer{}

	result := &core.PipelineResult{
		PipelineID: "test-3",
		Status:     core.StatusFailed,
		Duration:   1 * time.Second,
		Jobs: []core.JobResult{
			{JobName: "build", Status: core.StatusFailed},
			{JobName: "test", Status: core.StatusSkipped},
		},
	}

	printPipelineSummary(buf, result)

	output := buf.String()
	if !contains(output, "⊘") {
		t.Errorf("expected skipped icon '⊘' in output, got: %s", output)
	}
}

func TestStatusIconCore(t *testing.T) {
	tests := []struct {
		status core.Status
		want   string
	}{
		{core.StatusSuccess, "✓"},
		{core.StatusFailed, "✗"},
		{core.StatusRunning, "●"},
		{core.StatusPending, "○"},
		{core.StatusSkipped, "⊘"},
		{core.StatusCanceled, "⊗"},
		{core.Status("unknown"), "?"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			got := statusIconCore(tt.status)
			if got != tt.want {
				t.Errorf("statusIconCore(%v) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}
