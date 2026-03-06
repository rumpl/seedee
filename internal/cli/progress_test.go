package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rumpl/seedee/internal/core"
)

// --- Non-TTY (plain) tests ---

func TestProgressHandler_PipelineStarted_NonTTY(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, false)
	err := h.HandleEvent(&core.Event{
		Type:         core.EventPipelineStarted,
		PipelineID:   "pipe-1",
		PipelineName: "my-pipeline",
		Timestamp:    time.Now(),
	})
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "[+] Running pipeline") {
		t.Errorf("expected header line, got: %s", output)
	}
	if !strings.Contains(output, "my-pipeline") {
		t.Errorf("expected pipeline name, got: %s", output)
	}
}

func TestProgressHandler_PipelineStarted_FallbackToID(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, false)
	_ = h.HandleEvent(&core.Event{
		Type:       core.EventPipelineStarted,
		PipelineID: "pipe-456",
		Timestamp:  time.Now(),
	})
	if !strings.Contains(out.String(), "pipe-456") {
		t.Errorf("expected pipeline ID, got: %s", out.String())
	}
}

func TestProgressHandler_StepStarted_NonTTY(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, false)
	_ = h.HandleEvent(&core.Event{
		Type:       core.EventPipelineStarted,
		PipelineID: "pipe-1",
		Timestamp:  time.Now(),
	})
	out.Reset()

	_ = h.HandleEvent(&core.Event{
		Type:     core.EventStepStarted,
		JobName:  "build",
		StepName: "compile",
	})
	output := out.String()
	if !strings.Contains(output, "=>") {
		t.Errorf("expected '=>' prefix for in-progress step, got: %s", output)
	}
	if !strings.Contains(output, "build/compile") {
		t.Errorf("expected job/step name, got: %s", output)
	}
}

func TestProgressHandler_StepFinished_NonTTY(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, false)
	_ = h.HandleEvent(&core.Event{
		Type:       core.EventPipelineStarted,
		PipelineID: "pipe-1",
		Timestamp:  time.Now(),
	})
	_ = h.HandleEvent(&core.Event{
		Type:     core.EventStepStarted,
		JobName:  "build",
		StepName: "compile",
	})
	out.Reset()

	_ = h.HandleEvent(&core.Event{
		Type:     core.EventStepFinished,
		JobName:  "build",
		StepName: "compile",
		Status:   core.StatusSuccess,
		Duration: 1200 * time.Millisecond,
	})
	output := out.String()
	if !strings.Contains(output, "✓") {
		t.Errorf("expected success icon, got: %s", output)
	}
	if !strings.Contains(output, "build/compile") {
		t.Errorf("expected step name, got: %s", output)
	}
	if !strings.Contains(output, "1.2s") {
		t.Errorf("expected duration, got: %s", output)
	}
}

func TestProgressHandler_StepFailed_ShowsLogs_NonTTY(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, false)
	_ = h.HandleEvent(&core.Event{
		Type:       core.EventPipelineStarted,
		PipelineID: "pipe-1",
		Timestamp:  time.Now(),
	})
	_ = h.HandleEvent(&core.Event{
		Type:     core.EventStepStarted,
		JobName:  "build",
		StepName: "compile",
	})
	_ = h.HandleEvent(&core.Event{
		Type:     core.EventStepLog,
		JobName:  "build",
		StepName: "compile",
		LogData:  []byte("error: something went wrong\n"),
	})
	out.Reset()

	_ = h.HandleEvent(&core.Event{
		Type:     core.EventStepFinished,
		JobName:  "build",
		StepName: "compile",
		Status:   core.StatusFailed,
		Duration: 2 * time.Second,
	})
	output := out.String()
	if !strings.Contains(output, "✗") {
		t.Errorf("expected failure icon, got: %s", output)
	}
	if !strings.Contains(output, "error: something went wrong") {
		t.Errorf("expected log output after failure, got: %s", output)
	}
	if !strings.Contains(output, "│") {
		t.Errorf("expected log prefix '│', got: %s", output)
	}
}

func TestProgressHandler_JobFinished_NonTTY(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, false)
	_ = h.HandleEvent(&core.Event{
		Type:       core.EventPipelineStarted,
		PipelineID: "pipe-1",
		Timestamp:  time.Now(),
	})
	_ = h.HandleEvent(&core.Event{
		Type:    core.EventJobStarted,
		JobName: "build",
	})
	out.Reset()

	_ = h.HandleEvent(&core.Event{
		Type:     core.EventJobFinished,
		JobName:  "build",
		Status:   core.StatusSuccess,
		Duration: 5 * time.Second,
	})
	output := out.String()
	if !strings.Contains(output, "✓") {
		t.Errorf("expected success icon, got: %s", output)
	}
	if !strings.Contains(output, "build") {
		t.Errorf("expected job name, got: %s", output)
	}
	if !strings.Contains(output, "5s") {
		t.Errorf("expected duration, got: %s", output)
	}
}

func TestProgressHandler_JobSkipped_NonTTY(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, false)
	_ = h.HandleEvent(&core.Event{
		Type:       core.EventPipelineStarted,
		PipelineID: "pipe-1",
		Timestamp:  time.Now(),
	})
	out.Reset()

	_ = h.HandleEvent(&core.Event{
		Type:    core.EventJobSkipped,
		JobName: "deploy",
	})
	output := out.String()
	if !strings.Contains(output, "⊘") {
		t.Errorf("expected skip icon, got: %s", output)
	}
	if !strings.Contains(output, "deploy") {
		t.Errorf("expected job name, got: %s", output)
	}
	if !strings.Contains(output, "skipped") {
		t.Errorf("expected 'skipped', got: %s", output)
	}
}

func TestProgressHandler_NoANSI_NonTTY(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, false)
	_ = h.HandleEvent(&core.Event{
		Type:         core.EventPipelineStarted,
		PipelineID:   "pipe-1",
		PipelineName: "my-pipeline",
		Timestamp:    time.Now(),
	})
	_ = h.HandleEvent(&core.Event{
		Type:     core.EventStepStarted,
		JobName:  "build",
		StepName: "compile",
	})
	_ = h.HandleEvent(&core.Event{
		Type:     core.EventStepFinished,
		JobName:  "build",
		StepName: "compile",
		Status:   core.StatusSuccess,
		Duration: 1 * time.Second,
	})
	output := out.String()
	if strings.Contains(output, "\033[") {
		t.Errorf("expected no ANSI codes when non-TTY, got: %s", output)
	}
}

// --- TTY tests ---

func TestProgressHandler_PipelineStarted_TTY(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, true)
	_ = h.HandleEvent(&core.Event{
		Type:         core.EventPipelineStarted,
		PipelineID:   "pipe-1",
		PipelineName: "my-pipeline",
		Timestamp:    time.Now(),
	})
	output := out.String()
	// TTY output should contain ANSI codes
	if !strings.Contains(output, "\033[") {
		t.Errorf("expected ANSI codes in TTY mode, got: %s", output)
	}
	if !strings.Contains(output, "my-pipeline") {
		t.Errorf("expected pipeline name, got: %s", output)
	}
	if !strings.Contains(output, "[+]") {
		t.Errorf("expected [+] prefix, got: %s", output)
	}
}

func TestProgressHandler_InProgressStep_TTY(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, true)
	_ = h.HandleEvent(&core.Event{
		Type:         core.EventPipelineStarted,
		PipelineID:   "pipe-1",
		PipelineName: "test-pipeline",
		Timestamp:    time.Now(),
	})
	_ = h.HandleEvent(&core.Event{
		Type:     core.EventStepStarted,
		JobName:  "lint",
		StepName: "run-lint",
		Timestamp: time.Now(),
	})
	output := out.String()
	if !strings.Contains(output, "=>") {
		t.Errorf("expected '=>' for in-progress step, got: %s", output)
	}
	if !strings.Contains(output, "lint/run-lint") {
		t.Errorf("expected step name, got: %s", output)
	}
}

func TestProgressHandler_CompletedStep_TTY(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, true)
	_ = h.HandleEvent(&core.Event{
		Type:         core.EventPipelineStarted,
		PipelineID:   "pipe-1",
		PipelineName: "test-pipeline",
		Timestamp:    time.Now(),
	})
	_ = h.HandleEvent(&core.Event{
		Type:      core.EventStepStarted,
		JobName:   "lint",
		StepName:  "run-lint",
		Timestamp: time.Now(),
	})
	_ = h.HandleEvent(&core.Event{
		Type:     core.EventStepFinished,
		JobName:  "lint",
		StepName: "run-lint",
		Status:   core.StatusSuccess,
		Duration: 1900 * time.Millisecond,
	})
	output := out.String()
	if !strings.Contains(output, "✓") {
		t.Errorf("expected success icon, got: %s", output)
	}
	if !strings.Contains(output, "1.9s") {
		t.Errorf("expected duration, got: %s", output)
	}
}

func TestProgressHandler_MultipleParallelSteps_TTY(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, true)
	_ = h.HandleEvent(&core.Event{
		Type:         core.EventPipelineStarted,
		PipelineID:   "pipe-1",
		PipelineName: "ci",
		Timestamp:    time.Now(),
	})
	_ = h.HandleEvent(&core.Event{
		Type:      core.EventStepStarted,
		JobName:   "lint",
		StepName:  "deps",
		Timestamp: time.Now(),
	})
	_ = h.HandleEvent(&core.Event{
		Type:      core.EventStepStarted,
		JobName:   "build",
		StepName:  "compile",
		Timestamp: time.Now(),
	})
	output := out.String()

	// Both steps should appear
	if !strings.Contains(output, "lint/deps") {
		t.Errorf("expected lint/deps step, got: %s", output)
	}
	if !strings.Contains(output, "build/compile") {
		t.Errorf("expected build/compile step, got: %s", output)
	}
	// Both should show => since they're running
	count := strings.Count(output, "=>")
	if count < 2 {
		t.Errorf("expected at least 2 '=>' markers for parallel steps, got %d", count)
	}
}

func TestProgressHandler_FailedStepShowsLogs_TTY(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, true)
	_ = h.HandleEvent(&core.Event{
		Type:         core.EventPipelineStarted,
		PipelineID:   "pipe-1",
		PipelineName: "ci",
		Timestamp:    time.Now(),
	})
	_ = h.HandleEvent(&core.Event{
		Type:      core.EventStepStarted,
		JobName:   "test",
		StepName:  "unit",
		Timestamp: time.Now(),
	})
	_ = h.HandleEvent(&core.Event{
		Type:     core.EventStepLog,
		JobName:  "test",
		StepName: "unit",
		LogData:  []byte("FAIL: TestFoo\nexpected 1 got 2\n"),
	})
	_ = h.HandleEvent(&core.Event{
		Type:     core.EventStepFinished,
		JobName:  "test",
		StepName: "unit",
		Status:   core.StatusFailed,
		Duration: 3 * time.Second,
	})
	output := out.String()
	if !strings.Contains(output, "✗") {
		t.Errorf("expected failure icon, got: %s", output)
	}
	if !strings.Contains(output, "FAIL: TestFoo") {
		t.Errorf("expected log output after failure, got: %s", output)
	}
	if !strings.Contains(output, "│") {
		t.Errorf("expected log prefix, got: %s", output)
	}
}

func TestProgressHandler_LogTruncation(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, false)
	_ = h.HandleEvent(&core.Event{
		Type:       core.EventPipelineStarted,
		PipelineID: "pipe-1",
		Timestamp:  time.Now(),
	})
	_ = h.HandleEvent(&core.Event{
		Type:      core.EventStepStarted,
		JobName:   "test",
		StepName:  "unit",
		Timestamp: time.Now(),
	})

	// Send 20 log lines, only last 10 should be kept
	var lines []string
	for i := 0; i < 20; i++ {
		lines = append(lines, fmt.Sprintf("line-%d", i))
	}
	_ = h.HandleEvent(&core.Event{
		Type:     core.EventStepLog,
		JobName:  "test",
		StepName: "unit",
		LogData:  []byte(strings.Join(lines, "\n") + "\n"),
	})
	out.Reset()

	_ = h.HandleEvent(&core.Event{
		Type:     core.EventStepFinished,
		JobName:  "test",
		StepName: "unit",
		Status:   core.StatusFailed,
		Duration: 1 * time.Second,
	})

	output := out.String()
	// Should have the last 10 lines (line-10 through line-19)
	if strings.Contains(output, "line-0") {
		t.Errorf("expected old log lines to be truncated, got: %s", output)
	}
	if !strings.Contains(output, "line-19") {
		t.Errorf("expected last log line, got: %s", output)
	}
	if !strings.Contains(output, "line-10") {
		t.Errorf("expected 10th line from end, got: %s", output)
	}
}

func TestProgressHandler_EmptyLogIgnored(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, false)
	_ = h.HandleEvent(&core.Event{
		Type:       core.EventPipelineStarted,
		PipelineID: "pipe-1",
		Timestamp:  time.Now(),
	})
	_ = h.HandleEvent(&core.Event{
		Type:      core.EventStepStarted,
		JobName:   "build",
		StepName:  "compile",
		Timestamp: time.Now(),
	})
	_ = h.HandleEvent(&core.Event{
		Type:     core.EventStepLog,
		JobName:  "build",
		StepName: "compile",
		LogData:  []byte(""),
	})

	h.mu.Lock()
	ss := h.steps["build/compile"]
	logLen := len(ss.logLines)
	h.mu.Unlock()

	if logLen != 0 {
		t.Errorf("expected no log lines for empty data, got %d", logLen)
	}
}

// --- PrintSummary tests ---

func TestProgressHandler_PrintSummary_Success_NonTTY(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, false)
	_ = h.HandleEvent(&core.Event{
		Type:         core.EventPipelineStarted,
		PipelineID:   "pipe-1",
		PipelineName: "ci",
		Timestamp:    time.Now(),
	})
	out.Reset()

	h.PrintSummary(&core.PipelineResult{
		PipelineID: "pipe-1",
		Status:     core.StatusSuccess,
		Duration:   8300 * time.Millisecond,
		Jobs: []core.JobResult{
			{JobName: "lint", Status: core.StatusSuccess, Duration: 5100 * time.Millisecond},
			{JobName: "test", Status: core.StatusSuccess, Duration: 5100 * time.Millisecond},
			{JobName: "build", Status: core.StatusSuccess, Duration: 2300 * time.Millisecond},
		},
	})

	output := out.String()
	if !strings.Contains(output, "[+] Pipeline") {
		t.Errorf("expected [+] header, got: %s", output)
	}
	if !strings.Contains(output, "completed") {
		t.Errorf("expected 'completed', got: %s", output)
	}
	if !strings.Contains(output, "8.3s") {
		t.Errorf("expected total duration, got: %s", output)
	}
	if !strings.Contains(output, "lint") {
		t.Errorf("expected lint job, got: %s", output)
	}
	if !strings.Contains(output, "test") {
		t.Errorf("expected test job, got: %s", output)
	}
	if !strings.Contains(output, "build") {
		t.Errorf("expected build job, got: %s", output)
	}
	if strings.Contains(output, "\033[") {
		t.Errorf("expected no ANSI in non-TTY summary, got: %s", output)
	}
}

func TestProgressHandler_PrintSummary_Failed_NonTTY(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, false)
	_ = h.HandleEvent(&core.Event{
		Type:         core.EventPipelineStarted,
		PipelineID:   "pipe-1",
		PipelineName: "ci",
		Timestamp:    time.Now(),
	})
	out.Reset()

	h.PrintSummary(&core.PipelineResult{
		PipelineID: "pipe-1",
		Status:     core.StatusFailed,
		Duration:   5 * time.Second,
		Jobs: []core.JobResult{
			{JobName: "build", Status: core.StatusSuccess, Duration: 3 * time.Second},
			{JobName: "test", Status: core.StatusFailed, Duration: 2 * time.Second},
		},
	})

	output := out.String()
	if !strings.Contains(output, "failed") {
		t.Errorf("expected 'failed', got: %s", output)
	}
	if !strings.Contains(output, "✓") {
		t.Errorf("expected success icon for build, got: %s", output)
	}
	if !strings.Contains(output, "✗") {
		t.Errorf("expected failure icon for test, got: %s", output)
	}
}

func TestProgressHandler_PrintSummary_Canceled_NonTTY(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, false)
	_ = h.HandleEvent(&core.Event{
		Type:         core.EventPipelineStarted,
		PipelineID:   "pipe-1",
		PipelineName: "ci",
		Timestamp:    time.Now(),
	})
	out.Reset()

	h.PrintSummary(&core.PipelineResult{
		PipelineID: "pipe-1",
		Status:     core.StatusCanceled,
		Duration:   2 * time.Second,
		Jobs: []core.JobResult{
			{JobName: "build", Status: core.StatusCanceled, Duration: 2 * time.Second},
		},
	})

	output := out.String()
	if !strings.Contains(output, "canceled") {
		t.Errorf("expected 'canceled', got: %s", output)
	}
	if !strings.Contains(output, "⊗") {
		t.Errorf("expected canceled icon, got: %s", output)
	}
}

func TestProgressHandler_PrintSummary_Skipped_NonTTY(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, false)
	_ = h.HandleEvent(&core.Event{
		Type:         core.EventPipelineStarted,
		PipelineID:   "pipe-1",
		PipelineName: "ci",
		Timestamp:    time.Now(),
	})
	out.Reset()

	h.PrintSummary(&core.PipelineResult{
		PipelineID: "pipe-1",
		Status:     core.StatusFailed,
		Duration:   1 * time.Second,
		Jobs: []core.JobResult{
			{JobName: "build", Status: core.StatusFailed, Duration: 1 * time.Second},
			{JobName: "test", Status: core.StatusSkipped, Duration: 0},
		},
	})

	output := out.String()
	if !strings.Contains(output, "⊘") {
		t.Errorf("expected skip icon, got: %s", output)
	}
}

func TestProgressHandler_PrintSummary_TTY_HasANSI(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, true)
	_ = h.HandleEvent(&core.Event{
		Type:         core.EventPipelineStarted,
		PipelineID:   "pipe-1",
		PipelineName: "ci",
		Timestamp:    time.Now(),
	})
	out.Reset()

	h.PrintSummary(&core.PipelineResult{
		PipelineID: "pipe-1",
		Status:     core.StatusSuccess,
		Duration:   2 * time.Second,
		Jobs: []core.JobResult{
			{JobName: "build", Status: core.StatusSuccess, Duration: 2 * time.Second},
		},
	})

	output := out.String()
	if !strings.Contains(output, "\033[") {
		t.Errorf("expected ANSI codes in TTY summary, got: %s", output)
	}
}

// --- Thread safety test ---

func TestProgressHandler_ThreadSafe(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, false)
	_ = h.HandleEvent(&core.Event{
		Type:         core.EventPipelineStarted,
		PipelineID:   "pipe-1",
		PipelineName: "ci",
		Timestamp:    time.Now(),
	})

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = h.HandleEvent(&core.Event{
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
}

// --- stepKey tests ---

func TestStepKey(t *testing.T) {
	if got := stepKey("build", "compile"); got != "build/compile" {
		t.Errorf("stepKey = %q, want %q", got, "build/compile")
	}
}

// --- Summary with failed step log tail in TTY ---

func TestProgressHandler_PrintSummary_FailedLogTail_TTY(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, true)
	_ = h.HandleEvent(&core.Event{
		Type:         core.EventPipelineStarted,
		PipelineID:   "pipe-1",
		PipelineName: "ci",
		Timestamp:    time.Now(),
	})
	_ = h.HandleEvent(&core.Event{
		Type:      core.EventStepStarted,
		JobName:   "test",
		StepName:  "unit",
		Timestamp: time.Now(),
	})
	_ = h.HandleEvent(&core.Event{
		Type:     core.EventStepLog,
		JobName:  "test",
		StepName: "unit",
		LogData:  []byte("FAIL: TestFoo\n"),
	})
	_ = h.HandleEvent(&core.Event{
		Type:     core.EventStepFinished,
		JobName:  "test",
		StepName: "unit",
		Status:   core.StatusFailed,
		Duration: 3 * time.Second,
	})
	_ = h.HandleEvent(&core.Event{
		Type:    core.EventJobFinished,
		JobName: "test",
		Status:  core.StatusFailed,
		Duration: 3 * time.Second,
	})
	_ = h.HandleEvent(&core.Event{
		Type:       core.EventPipelineFinished,
		PipelineID: "pipe-1",
		Status:     core.StatusFailed,
		Duration:   3 * time.Second,
	})
	out.Reset()

	h.PrintSummary(&core.PipelineResult{
		PipelineID: "pipe-1",
		Status:     core.StatusFailed,
		Duration:   3 * time.Second,
		Jobs: []core.JobResult{
			{JobName: "test", Status: core.StatusFailed, Duration: 3 * time.Second},
		},
	})

	output := out.String()
	if !strings.Contains(output, "FAIL: TestFoo") {
		t.Errorf("expected failed step log in summary, got: %s", output)
	}
}

func TestProgressHandler_PrintSummary_FallbackToPipelineID(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, false)
	// Don't send PipelineStarted event so pipelineName stays empty

	h.PrintSummary(&core.PipelineResult{
		PipelineID: "pipe-xyz",
		Status:     core.StatusSuccess,
		Duration:   1 * time.Second,
		Jobs: []core.JobResult{
			{JobName: "build", Status: core.StatusSuccess, Duration: 1 * time.Second},
		},
	})

	output := out.String()
	if !strings.Contains(output, "pipe-xyz") {
		t.Errorf("expected pipeline ID in summary, got: %s", output)
	}
}

func TestProgressHandler_StepFinishedWithoutStart(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, false)
	_ = h.HandleEvent(&core.Event{
		Type:       core.EventPipelineStarted,
		PipelineID: "pipe-1",
		Timestamp:  time.Now(),
	})
	out.Reset()

	// Step finished without a prior start event
	_ = h.HandleEvent(&core.Event{
		Type:     core.EventStepFinished,
		JobName:  "build",
		StepName: "compile",
		Status:   core.StatusSuccess,
		Duration: 1 * time.Second,
	})

	output := out.String()
	if !strings.Contains(output, "✓") {
		t.Errorf("expected success icon even without start event, got: %s", output)
	}
}

func TestProgressHandler_LogWithoutStepStart(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, false)
	_ = h.HandleEvent(&core.Event{
		Type:       core.EventPipelineStarted,
		PipelineID: "pipe-1",
		Timestamp:  time.Now(),
	})

	// Log without a prior step start
	err := h.HandleEvent(&core.Event{
		Type:     core.EventStepLog,
		JobName:  "build",
		StepName: "compile",
		LogData:  []byte("some output\n"),
	})
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}

	h.mu.Lock()
	ss, ok := h.steps["build/compile"]
	h.mu.Unlock()

	if !ok {
		t.Fatal("expected step to be auto-created")
	}
	if len(ss.logLines) != 1 || ss.logLines[0] != "some output" {
		t.Errorf("expected log line, got: %v", ss.logLines)
	}
}

// Verify that the formatStepLine function handles canceled steps.
func TestProgressHandler_FormatStepLine_Canceled(t *testing.T) {
	h := newProgressHandler(&bytes.Buffer{}, &bytes.Buffer{}, true)
	ss := &stepState{
		jobName:  "build",
		stepName: "compile",
		status:   core.StatusCanceled,
	}
	line := h.formatStepLine(ss)
	if !strings.Contains(line, "⊗") {
		t.Errorf("expected canceled icon, got: %s", line)
	}
	if !strings.Contains(line, "build/compile") {
		t.Errorf("expected step name, got: %s", line)
	}
}

func TestProgressHandler_JobFinishedFailed_NonTTY(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, false)
	_ = h.HandleEvent(&core.Event{
		Type:       core.EventPipelineStarted,
		PipelineID: "pipe-1",
		Timestamp:  time.Now(),
	})
	_ = h.HandleEvent(&core.Event{
		Type:    core.EventJobStarted,
		JobName: "build",
	})
	out.Reset()

	_ = h.HandleEvent(&core.Event{
		Type:     core.EventJobFinished,
		JobName:  "build",
		Status:   core.StatusFailed,
		Duration: 2 * time.Second,
		Error:    "compilation error",
	})
	output := out.String()
	if !strings.Contains(output, "✗") {
		t.Errorf("expected failure icon, got: %s", output)
	}
	if !strings.Contains(output, "compilation error") {
		t.Errorf("expected error message, got: %s", output)
	}
}
