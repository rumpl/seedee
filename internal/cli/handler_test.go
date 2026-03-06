package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/rumpl/seedee/internal/core"
)

func TestTerminalEventHandler_PipelineStarted(t *testing.T) {
	out := &bytes.Buffer{}
	h := &terminalEventHandler{out: out, errOut: out, isTTY: false}
	err := h.HandleEvent(core.Event{
		Type:         core.EventPipelineStarted,
		PipelineID:   "pipe-123",
		PipelineName: "my-pipeline",
	})
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "my-pipeline") {
		t.Errorf("output missing pipeline name, got: %s", output)
	}
	if !strings.Contains(output, "started") {
		t.Errorf("output missing 'started', got: %s", output)
	}
	if !strings.Contains(output, "▶") {
		t.Errorf("output missing '▶', got: %s", output)
	}
}

func TestTerminalEventHandler_PipelineStarted_FallbackToID(t *testing.T) {
	out := &bytes.Buffer{}
	h := &terminalEventHandler{out: out, errOut: out, isTTY: false}
	err := h.HandleEvent(core.Event{
		Type:       core.EventPipelineStarted,
		PipelineID: "pipe-456",
	})
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if !strings.Contains(out.String(), "pipe-456") {
		t.Errorf("output missing pipeline ID, got: %s", out.String())
	}
}

func TestTerminalEventHandler_JobStarted(t *testing.T) {
	out := &bytes.Buffer{}
	h := &terminalEventHandler{out: out, errOut: out, isTTY: false}
	err := h.HandleEvent(core.Event{
		Type:    core.EventJobStarted,
		JobName: "build",
	})
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "build") {
		t.Errorf("output missing job name, got: %s", output)
	}
	if !strings.Contains(output, "▶") {
		t.Errorf("output missing '▶', got: %s", output)
	}
}

func TestTerminalEventHandler_JobFinished(t *testing.T) {
	out := &bytes.Buffer{}
	h := &terminalEventHandler{out: out, errOut: out, isTTY: false}
	err := h.HandleEvent(core.Event{
		Type:     core.EventJobFinished,
		JobName:  "build",
		Status:   core.StatusSuccess,
		Duration: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "build") {
		t.Errorf("output missing job name, got: %s", output)
	}
	if !strings.Contains(output, "✓") {
		t.Errorf("output missing success icon, got: %s", output)
	}
	if !strings.Contains(output, "5s") {
		t.Errorf("output missing duration, got: %s", output)
	}
}

func TestTerminalEventHandler_JobFinishedFailed(t *testing.T) {
	out := &bytes.Buffer{}
	h := &terminalEventHandler{out: out, errOut: out, isTTY: false}
	err := h.HandleEvent(core.Event{
		Type:    core.EventJobFinished,
		JobName: "build",
		Status:  core.StatusFailed,
		Error:   "compilation error",
	})
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "✗") {
		t.Errorf("output missing failure icon, got: %s", output)
	}
	if !strings.Contains(output, "compilation error") {
		t.Errorf("output missing error, got: %s", output)
	}
}

func TestTerminalEventHandler_JobSkipped(t *testing.T) {
	out := &bytes.Buffer{}
	h := &terminalEventHandler{out: out, errOut: out, isTTY: false}
	err := h.HandleEvent(core.Event{
		Type:    core.EventJobSkipped,
		JobName: "deploy",
	})
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "deploy") {
		t.Errorf("output missing job name, got: %s", output)
	}
	if !strings.Contains(output, "skipped") {
		t.Errorf("output missing 'skipped', got: %s", output)
	}
	if !strings.Contains(output, "⊘") {
		t.Errorf("output missing skip icon, got: %s", output)
	}
}

func TestTerminalEventHandler_StepFinished(t *testing.T) {
	out := &bytes.Buffer{}
	h := &terminalEventHandler{out: out, errOut: out, isTTY: false}
	err := h.HandleEvent(core.Event{
		Type:     core.EventStepFinished,
		JobName:  "build",
		StepName: "compile",
		Status:   core.StatusFailed,
		ExitCode: 1,
		Duration: 3 * time.Second,
		Error:    "compilation error",
	})
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "build/compile") {
		t.Errorf("output missing step info, got: %s", output)
	}
	if !strings.Contains(output, "exit code 1") {
		t.Errorf("output missing exit code, got: %s", output)
	}
	if !strings.Contains(output, "compilation error") {
		t.Errorf("output missing error, got: %s", output)
	}
	if !strings.Contains(output, "✗") {
		t.Errorf("output missing failure icon, got: %s", output)
	}
}

func TestTerminalEventHandler_StepFinishedSuccess(t *testing.T) {
	out := &bytes.Buffer{}
	h := &terminalEventHandler{out: out, errOut: out, isTTY: false}
	err := h.HandleEvent(core.Event{
		Type:     core.EventStepFinished,
		JobName:  "build",
		StepName: "compile",
		Status:   core.StatusSuccess,
		Duration: 1200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "✓") {
		t.Errorf("output missing success icon, got: %s", output)
	}
	if !strings.Contains(output, "1.2s") {
		t.Errorf("output missing duration, got: %s", output)
	}
}

func TestTerminalEventHandler_StepLogStdout(t *testing.T) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	h := &terminalEventHandler{out: out, errOut: errOut, isTTY: false}
	err := h.HandleEvent(core.Event{
		Type:     core.EventStepLog,
		JobName:  "build",
		StepName: "compile",
		LogData:  []byte("building...\n"),
		IsStderr: false,
	})
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "building...") {
		t.Errorf("stdout missing log data, got: %s", output)
	}
	if !strings.Contains(output, "[build/compile]") {
		t.Errorf("output missing job/step prefix, got: %s", output)
	}
	if errOut.Len() != 0 {
		t.Errorf("stderr should be empty, got: %s", errOut.String())
	}
}

func TestTerminalEventHandler_StepLogStderr(t *testing.T) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	h := &terminalEventHandler{out: out, errOut: errOut, isTTY: false}
	err := h.HandleEvent(core.Event{
		Type:     core.EventStepLog,
		JobName:  "build",
		StepName: "compile",
		LogData:  []byte("warning: something\n"),
		IsStderr: true,
	})
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("stdout should be empty, got: %s", out.String())
	}
	if !strings.Contains(errOut.String(), "warning: something") {
		t.Errorf("stderr missing log data, got: %s", errOut.String())
	}
	if !strings.Contains(errOut.String(), "[build/compile]") {
		t.Errorf("stderr missing prefix, got: %s", errOut.String())
	}
}

func TestTerminalEventHandler_StepLogMultipleLines(t *testing.T) {
	out := &bytes.Buffer{}
	h := &terminalEventHandler{out: out, errOut: &bytes.Buffer{}, isTTY: false}
	err := h.HandleEvent(core.Event{
		Type:     core.EventStepLog,
		JobName:  "build",
		StepName: "compile",
		LogData:  []byte("line1\nline2\nline3\n"),
		IsStderr: false,
	})
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	output := out.String()
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d: %q", len(lines), output)
	}
	for _, line := range lines {
		if !strings.Contains(line, "[build/compile]") {
			t.Errorf("line missing prefix: %s", line)
		}
	}
}

func TestTerminalEventHandler_StepLogEmptyData(t *testing.T) {
	out := &bytes.Buffer{}
	h := &terminalEventHandler{out: out, errOut: &bytes.Buffer{}, isTTY: false}
	err := h.HandleEvent(core.Event{
		Type:     core.EventStepLog,
		JobName:  "build",
		StepName: "compile",
		LogData:  []byte(""),
		IsStderr: false,
	})
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output for empty log, got: %s", out.String())
	}
}

func TestTerminalEventHandler_ThreadSafe(t *testing.T) {
	out := &bytes.Buffer{}
	h := &terminalEventHandler{out: out, errOut: out, isTTY: false}

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = h.HandleEvent(core.Event{
					Type:     core.EventStepLog,
					JobName:  "job",
					StepName: "step",
					LogData:  []byte("data\n"),
				})
			}
			done <- struct{}{}
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	if out.Len() == 0 {
		t.Error("expected some output")
	}
}

func TestJobColorAssignment(t *testing.T) {
	h := &terminalEventHandler{isTTY: true}

	// Same job always gets same color
	c1 := h.colorForJob("build")
	c2 := h.colorForJob("build")
	if c1 != c2 {
		t.Errorf("same job got different colors: %s vs %s", c1, c2)
	}

	// Different jobs get different colors
	c3 := h.colorForJob("test")
	if c1 == c3 {
		t.Errorf("different jobs got same color: %s", c1)
	}

	// 11th job wraps around to first color
	allJobs := []string{"j0", "j1", "j2", "j3", "j4", "j5", "j6", "j7", "j8", "j9"}
	// Reset handler
	h2 := &terminalEventHandler{isTTY: true}
	colors := make([]string, len(allJobs))
	for i, j := range allJobs {
		colors[i] = h2.colorForJob(j)
	}
	c11 := h2.colorForJob("j10")
	if c11 != colors[0] {
		t.Errorf("11th job color %s should wrap to first job color %s", c11, colors[0])
	}
}

func TestColorize_TTY(t *testing.T) {
	result := colorize(true, "32", "hello")
	if !strings.Contains(result, "\033[32m") {
		t.Errorf("expected ANSI code in TTY mode, got: %s", result)
	}
	if !strings.Contains(result, "hello") {
		t.Errorf("expected text in result, got: %s", result)
	}
	if !strings.Contains(result, "\033[0m") {
		t.Errorf("expected ANSI reset, got: %s", result)
	}
}

func TestColorize_Pipe(t *testing.T) {
	result := colorize(false, "32", "hello")
	if strings.Contains(result, "\033[") {
		t.Errorf("expected no ANSI codes when not TTY, got: %s", result)
	}
	if result != "hello" {
		t.Errorf("expected plain text 'hello', got: %s", result)
	}
}

func TestPrintSummary_Success(t *testing.T) {
	out := &bytes.Buffer{}
	h := &terminalEventHandler{out: out, errOut: out, isTTY: false}

	result := &core.PipelineResult{
		PipelineID: "test-1",
		Status:     core.StatusSuccess,
		Duration:   7300 * time.Millisecond,
		Jobs: []core.JobResult{
			{JobName: "build", Status: core.StatusSuccess, Duration: 4600 * time.Millisecond},
			{JobName: "lint", Status: core.StatusSuccess, Duration: 2100 * time.Millisecond},
			{JobName: "test", Status: core.StatusSuccess, Duration: 5200 * time.Millisecond},
		},
	}

	h.PrintSummary(result)

	output := out.String()
	if !strings.Contains(output, "test-1") {
		t.Errorf("expected pipeline ID in summary, got: %s", output)
	}
	if !strings.Contains(output, "✓ success") {
		t.Errorf("expected '✓ success' in summary, got: %s", output)
	}
	if !strings.Contains(output, "7.3s") {
		t.Errorf("expected '7.3s' duration in summary, got: %s", output)
	}
	if !strings.Contains(output, "build") {
		t.Errorf("expected 'build' job in summary, got: %s", output)
	}
	if !strings.Contains(output, "lint") {
		t.Errorf("expected 'lint' job in summary, got: %s", output)
	}
	if !strings.Contains(output, "test") {
		t.Errorf("expected 'test' job in summary, got: %s", output)
	}
	if !strings.Contains(output, "4.6s") {
		t.Errorf("expected '4.6s' build duration, got: %s", output)
	}
	if !strings.Contains(output, "━") {
		t.Errorf("expected separator in summary, got: %s", output)
	}
}

func TestPrintSummary_Failure(t *testing.T) {
	out := &bytes.Buffer{}
	h := &terminalEventHandler{out: out, errOut: out, isTTY: false}

	result := &core.PipelineResult{
		PipelineID: "test-2",
		Status:     core.StatusFailed,
		Duration:   5 * time.Second,
		Jobs: []core.JobResult{
			{JobName: "build", Status: core.StatusSuccess, Duration: 3 * time.Second},
			{JobName: "test", Status: core.StatusFailed, Duration: 2 * time.Second},
		},
	}

	h.PrintSummary(result)

	output := out.String()
	if !strings.Contains(output, "✗ failed") {
		t.Errorf("expected '✗ failed' in summary, got: %s", output)
	}
	// Check we have both icons
	if !strings.Contains(output, "✓") {
		t.Errorf("expected success icon for build, got: %s", output)
	}
	if !strings.Contains(output, "✗") {
		t.Errorf("expected failure icon for test, got: %s", output)
	}
}

func TestPrintSummary_WithSkipped(t *testing.T) {
	out := &bytes.Buffer{}
	h := &terminalEventHandler{out: out, errOut: out, isTTY: false}

	result := &core.PipelineResult{
		PipelineID: "test-3",
		Status:     core.StatusFailed,
		Duration:   1 * time.Second,
		Jobs: []core.JobResult{
			{JobName: "build", Status: core.StatusFailed, Duration: 1 * time.Second},
			{JobName: "test", Status: core.StatusSkipped, Duration: 0},
		},
	}

	h.PrintSummary(result)

	output := out.String()
	if !strings.Contains(output, "⊘") {
		t.Errorf("expected skipped icon '⊘' in summary, got: %s", output)
	}
}

func TestPrintSummary_Canceled(t *testing.T) {
	out := &bytes.Buffer{}
	h := &terminalEventHandler{out: out, errOut: out, isTTY: false}

	result := &core.PipelineResult{
		PipelineID: "test-4",
		Status:     core.StatusCanceled,
		Duration:   2 * time.Second,
		Jobs: []core.JobResult{
			{JobName: "build", Status: core.StatusCanceled, Duration: 2 * time.Second},
		},
	}

	h.PrintSummary(result)

	output := out.String()
	if !strings.Contains(output, "⊗ canceled") {
		t.Errorf("expected '⊗ canceled' in summary, got: %s", output)
	}
}

func TestPrintSummary_NoColors_WhenPiped(t *testing.T) {
	out := &bytes.Buffer{}
	h := &terminalEventHandler{out: out, errOut: out, isTTY: false}

	result := &core.PipelineResult{
		PipelineID: "test-5",
		Status:     core.StatusSuccess,
		Duration:   2500 * time.Millisecond,
		Jobs: []core.JobResult{
			{JobName: "build", Status: core.StatusSuccess, Duration: 2500 * time.Millisecond},
		},
	}

	h.PrintSummary(result)

	output := out.String()
	if strings.Contains(output, "\033[") {
		t.Errorf("expected no ANSI codes when piped, got: %s", output)
	}
	if !strings.Contains(output, "✓ success") {
		t.Errorf("expected plain '✓ success', got: %s", output)
	}
}

func TestPrintSummary_WithColors_WhenTTY(t *testing.T) {
	out := &bytes.Buffer{}
	h := &terminalEventHandler{out: out, errOut: out, isTTY: true}

	result := &core.PipelineResult{
		PipelineID: "test-6",
		Status:     core.StatusSuccess,
		Duration:   2 * time.Second,
		Jobs: []core.JobResult{
			{JobName: "build", Status: core.StatusSuccess, Duration: 2 * time.Second},
		},
	}

	h.PrintSummary(result)

	output := out.String()
	if !strings.Contains(output, "\033[") {
		t.Errorf("expected ANSI codes in TTY mode, got: %s", output)
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
