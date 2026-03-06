package cli

import (
	"bytes"
	"testing"
	"time"

	"github.com/rumpl/seedee/internal/core"
)

func TestTerminalLogWriter_WriteLog_Stdout(t *testing.T) {
	buf := &bytes.Buffer{}
	w := &terminalLogWriter{out: buf, errOut: &bytes.Buffer{}}

	err := w.WriteLog("build", "compile", []byte("building...\n"), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !contains(output, "[build/compile]") {
		t.Errorf("expected [build/compile] prefix, got: %s", output)
	}
	if !contains(output, "building...") {
		t.Errorf("expected 'building...' in output, got: %s", output)
	}
}

func TestTerminalLogWriter_WriteLog_Stderr(t *testing.T) {
	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}
	w := &terminalLogWriter{out: stdoutBuf, errOut: stderrBuf}

	err := w.WriteLog("test", "run", []byte("error occurred\n"), true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stdoutBuf.Len() != 0 {
		t.Errorf("expected no stdout output, got: %s", stdoutBuf.String())
	}

	output := stderrBuf.String()
	if !contains(output, "[test/run]") {
		t.Errorf("expected [test/run] prefix in stderr, got: %s", output)
	}
	if !contains(output, "error occurred") {
		t.Errorf("expected 'error occurred' in stderr output, got: %s", output)
	}
}

func TestTerminalLogWriter_MultipleLines(t *testing.T) {
	buf := &bytes.Buffer{}
	w := &terminalLogWriter{out: buf, errOut: &bytes.Buffer{}}

	err := w.WriteLog("build", "compile", []byte("line1\nline2\nline3\n"), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// Each line should have its own prefix
	expected := "    [build/compile] line1\n    [build/compile] line2\n    [build/compile] line3\n"
	if output != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, output)
	}
}

func TestTerminalLogWriter_ThreadSafe(t *testing.T) {
	buf := &bytes.Buffer{}
	w := &terminalLogWriter{out: buf, errOut: buf}

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = w.WriteLog("job", "step", []byte("data\n"), false)
			}
			done <- struct{}{}
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// If we got here without a race condition, the test passes
	if buf.Len() == 0 {
		t.Error("expected some output")
	}
}

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
