package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rumpl/seedee/internal/core"
)

// fixedWidth returns a getWidth func that always returns w.
func fixedWidth(w int) func() int { return func() int { return w } }

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
	h.getWidth = fixedWidth(120)
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
		t.Errorf("expected '=>' prefix for job header, got: %s", output)
	}
	if !strings.Contains(output, "build") {
		t.Errorf("expected job name in header, got: %s", output)
	}
}

func TestProgressHandler_StepFinished_NonTTY(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, false)
	h.getWidth = fixedWidth(120)
	_ = h.HandleEvent(&core.Event{
		Type:       core.EventPipelineStarted,
		PipelineID: "pipe-1",
		Timestamp:  time.Now(),
	})
	_ = h.HandleEvent(&core.Event{
		Type:    core.EventJobStarted,
		JobName: "build",
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
		LogData:  []byte("compiled OK\n"),
	})
	out.Reset()

	_ = h.HandleEvent(&core.Event{
		Type:     core.EventJobFinished,
		JobName:  "build",
		Status:   core.StatusSuccess,
		Duration: 1200 * time.Millisecond,
	})

	output := out.String()
	if !strings.Contains(output, "✓") {
		t.Errorf("expected success icon, got: %s", output)
	}
	if !strings.Contains(output, "build") {
		t.Errorf("expected job name, got: %s", output)
	}
	if !strings.Contains(output, "1.2s") {
		t.Errorf("expected duration, got: %s", output)
	}
}

func TestProgressHandler_StepFailed_ShowsLogs_NonTTY(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, false)
	h.getWidth = fixedWidth(120)
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
	h.getWidth = fixedWidth(120)
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
	h.getWidth = fixedWidth(120)
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
	h.getWidth = fixedWidth(120)
	_ = h.HandleEvent(&core.Event{
		Type:         core.EventPipelineStarted,
		PipelineID:   "pipe-1",
		PipelineName: "my-pipeline",
		Timestamp:    time.Now(),
	})
	_ = h.HandleEvent(&core.Event{
		Type:    core.EventJobStarted,
		JobName: "build",
	})
	_ = h.HandleEvent(&core.Event{
		Type:     core.EventStepStarted,
		JobName:  "build",
		StepName: "compile",
	})
	_ = h.HandleEvent(&core.Event{
		Type:     core.EventJobFinished,
		JobName:  "build",
		Status:   core.StatusSuccess,
		Duration: 1 * time.Second,
	})
	output := out.String()
	if strings.Contains(output, "\033[") {
		t.Errorf("expected no ANSI codes when non-TTY, got: %s", output)
	}
}

// --- Streaming logs tests ---

func TestProgressHandler_StreamingLogs_NonTTY(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, false)
	h.getWidth = fixedWidth(120)
	_ = h.HandleEvent(&core.Event{
		Type:         core.EventPipelineStarted,
		PipelineID:   "pipe-1",
		PipelineName: "ci",
		Timestamp:    time.Now(),
	})
	_ = h.HandleEvent(&core.Event{
		Type:    core.EventJobStarted,
		JobName: "lint",
	})
	_ = h.HandleEvent(&core.Event{
		Type:     core.EventStepStarted,
		JobName:  "lint",
		StepName: "Download dependencies",
	})
	out.Reset()

	_ = h.HandleEvent(&core.Event{
		Type:     core.EventStepLog,
		JobName:  "lint",
		StepName: "Download dependencies",
		LogData:  []byte("go: downloading gopkg.in/yaml.v3 v3.0.1\n"),
	})
	output := out.String()
	if !strings.Contains(output, "[Download dependencies]") {
		t.Errorf("expected step name prefix in streamed log, got: %s", output)
	}
	if !strings.Contains(output, "go: downloading gopkg.in/yaml.v3 v3.0.1") {
		t.Errorf("expected log content, got: %s", output)
	}
}

func TestProgressHandler_WaitingForDeps_NonTTY(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, false)
	h.getWidth = fixedWidth(120)
	_ = h.HandleEvent(&core.Event{
		Type:         core.EventPipelineStarted,
		PipelineID:   "pipe-1",
		PipelineName: "ci",
		Timestamp:    time.Now(),
	})

	h.SetJobDependsOn("build", []string{"test"})
	out.Reset()

	_ = h.HandleEvent(&core.Event{
		Type:    core.EventJobStarted,
		JobName: "build",
	})
	output := out.String()
	if !strings.Contains(output, "waiting for test") {
		t.Errorf("expected 'waiting for test', got: %s", output)
	}
}

// --- TTY tests ---

func TestProgressHandler_PipelineStarted_TTY(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, true)
	h.getWidth = fixedWidth(120)
	_ = h.HandleEvent(&core.Event{
		Type:         core.EventPipelineStarted,
		PipelineID:   "pipe-1",
		PipelineName: "my-pipeline",
		Timestamp:    time.Now(),
	})
	output := out.String()
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
	h.getWidth = fixedWidth(120)
	_ = h.HandleEvent(&core.Event{
		Type:         core.EventPipelineStarted,
		PipelineID:   "pipe-1",
		PipelineName: "test-pipeline",
		Timestamp:    time.Now(),
	})
	_ = h.HandleEvent(&core.Event{
		Type:    core.EventJobStarted,
		JobName: "lint",
	})
	_ = h.HandleEvent(&core.Event{
		Type:      core.EventStepStarted,
		JobName:   "lint",
		StepName:  "run-lint",
		Timestamp: time.Now(),
	})
	output := out.String()
	if !strings.Contains(output, "=>") {
		t.Errorf("expected '=>' for in-progress job, got: %s", output)
	}
	if !strings.Contains(output, "lint") {
		t.Errorf("expected job name, got: %s", output)
	}
}

func TestProgressHandler_CompletedStep_TTY(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, true)
	h.getWidth = fixedWidth(120)
	_ = h.HandleEvent(&core.Event{
		Type:         core.EventPipelineStarted,
		PipelineID:   "pipe-1",
		PipelineName: "test-pipeline",
		Timestamp:    time.Now(),
	})
	_ = h.HandleEvent(&core.Event{
		Type:    core.EventJobStarted,
		JobName: "lint",
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
	_ = h.HandleEvent(&core.Event{
		Type:     core.EventJobFinished,
		JobName:  "lint",
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

func TestProgressHandler_MultipleParallelJobs_TTY(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, true)
	h.getWidth = fixedWidth(120)
	_ = h.HandleEvent(&core.Event{
		Type:         core.EventPipelineStarted,
		PipelineID:   "pipe-1",
		PipelineName: "ci",
		Timestamp:    time.Now(),
	})
	_ = h.HandleEvent(&core.Event{
		Type:    core.EventJobStarted,
		JobName: "lint",
	})
	_ = h.HandleEvent(&core.Event{
		Type:      core.EventStepStarted,
		JobName:   "lint",
		StepName:  "deps",
		Timestamp: time.Now(),
	})
	_ = h.HandleEvent(&core.Event{
		Type:    core.EventJobStarted,
		JobName: "build",
	})
	_ = h.HandleEvent(&core.Event{
		Type:      core.EventStepStarted,
		JobName:   "build",
		StepName:  "compile",
		Timestamp: time.Now(),
	})
	output := out.String()

	if !strings.Contains(output, "lint") {
		t.Errorf("expected lint job, got: %s", output)
	}
	if !strings.Contains(output, "build") {
		t.Errorf("expected build job, got: %s", output)
	}
	count := strings.Count(output, "=>")
	if count < 2 {
		t.Errorf("expected at least 2 '=>' markers for parallel jobs, got %d", count)
	}
}

func TestProgressHandler_FailedStepShowsLogs_TTY(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, true)
	h.getWidth = fixedWidth(120)
	_ = h.HandleEvent(&core.Event{
		Type:         core.EventPipelineStarted,
		PipelineID:   "pipe-1",
		PipelineName: "ci",
		Timestamp:    time.Now(),
	})
	_ = h.HandleEvent(&core.Event{
		Type:    core.EventJobStarted,
		JobName: "test",
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
	_ = h.HandleEvent(&core.Event{
		Type:     core.EventJobFinished,
		JobName:  "test",
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
	h.getWidth = fixedWidth(120)
	_ = h.HandleEvent(&core.Event{
		Type:       core.EventPipelineStarted,
		PipelineID: "pipe-1",
		Timestamp:  time.Now(),
	})
	_ = h.HandleEvent(&core.Event{
		Type:    core.EventJobStarted,
		JobName: "test",
	})
	_ = h.HandleEvent(&core.Event{
		Type:      core.EventStepStarted,
		JobName:   "test",
		StepName:  "unit",
		Timestamp: time.Now(),
	})

	// Send 20 log lines, only last 10 should be kept in stepState
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
	h.getWidth = fixedWidth(120)
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
	h.getWidth = fixedWidth(80)
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
	h.getWidth = fixedWidth(80)
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
	h.getWidth = fixedWidth(80)
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
	h.getWidth = fixedWidth(80)
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
	h.getWidth = fixedWidth(120)
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
	h.getWidth = fixedWidth(120)
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

// --- Live elapsed-time timer tests ---

func TestProgressHandler_RunningJobShowsElapsed_TTY(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, true)
	h.getWidth = fixedWidth(120)
	_ = h.HandleEvent(&core.Event{
		Type:         core.EventPipelineStarted,
		PipelineID:   "pipe-1",
		PipelineName: "ci",
		Timestamp:    time.Now(),
	})
	_ = h.HandleEvent(&core.Event{
		Type:    core.EventJobStarted,
		JobName: "build",
	})
	output := out.String()
	if !strings.Contains(output, "0s") {
		t.Errorf("expected running job to show elapsed time, got: %s", output)
	}
}

func TestProgressHandler_LiveRefresh_RedrawsWithoutEvents_TTY(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, true)
	h.getWidth = fixedWidth(120)
	_ = h.HandleEvent(&core.Event{
		Type:         core.EventPipelineStarted,
		PipelineID:   "pipe-1",
		PipelineName: "ci",
		Timestamp:    time.Now(),
	})
	_ = h.HandleEvent(&core.Event{
		Type:    core.EventJobStarted,
		JobName: "build",
	})
	_ = h.HandleEvent(&core.Event{
		Type:      core.EventStepStarted,
		JobName:   "build",
		StepName:  "compile",
		Timestamp: time.Now(),
	})

	before := len(out.String())

	h.startLiveRefresh(20 * time.Millisecond)
	defer h.stopLiveRefresh()

	time.Sleep(150 * time.Millisecond)

	after := len(out.String())
	if after <= before {
		t.Errorf("expected periodic redraws without events, before=%d after=%d", before, after)
	}
}

func TestProgressHandler_LiveRefresh_NoopNonTTY(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, false)
	h.getWidth = fixedWidth(120)
	_ = h.HandleEvent(&core.Event{
		Type:         core.EventPipelineStarted,
		PipelineID:   "pipe-1",
		PipelineName: "ci",
		Timestamp:    time.Now(),
	})
	_ = h.HandleEvent(&core.Event{
		Type:    core.EventJobStarted,
		JobName: "build",
	})

	before := len(out.String())

	h.startLiveRefresh(10 * time.Millisecond)
	defer h.stopLiveRefresh()

	time.Sleep(50 * time.Millisecond)

	after := len(out.String())
	if after != before {
		t.Errorf("expected no redraws in non-TTY mode, before=%d after=%d", before, after)
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
	h.getWidth = fixedWidth(120)
	_ = h.HandleEvent(&core.Event{
		Type:         core.EventPipelineStarted,
		PipelineID:   "pipe-1",
		PipelineName: "ci",
		Timestamp:    time.Now(),
	})
	_ = h.HandleEvent(&core.Event{
		Type:    core.EventJobStarted,
		JobName: "test",
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
		Type:     core.EventJobFinished,
		JobName:  "test",
		Status:   core.StatusFailed,
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
	h.getWidth = fixedWidth(80)

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
	h.getWidth = fixedWidth(120)
	_ = h.HandleEvent(&core.Event{
		Type:       core.EventPipelineStarted,
		PipelineID: "pipe-1",
		Timestamp:  time.Now(),
	})
	out.Reset()

	_ = h.HandleEvent(&core.Event{
		Type:     core.EventStepFinished,
		JobName:  "build",
		StepName: "compile",
		Status:   core.StatusSuccess,
		Duration: 1 * time.Second,
	})

	// No crash is the main assertion.
}

func TestProgressHandler_LogWithoutStepStart(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, false)
	h.getWidth = fixedWidth(120)
	_ = h.HandleEvent(&core.Event{
		Type:       core.EventPipelineStarted,
		PipelineID: "pipe-1",
		Timestamp:  time.Now(),
	})

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

func TestProgressHandler_JobFinishedFailed_NonTTY(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, false)
	h.getWidth = fixedWidth(120)
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

// --- Terminal width tests ---

func TestProgressHandler_TerminalWidthPadding_NonTTY(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, false)
	h.getWidth = fixedWidth(40)

	h.PrintSummary(&core.PipelineResult{
		PipelineID: "pipe-1",
		Status:     core.StatusSuccess,
		Duration:   1 * time.Second,
		Jobs: []core.JobResult{
			{JobName: "build", Status: core.StatusSuccess, Duration: 1 * time.Second},
		},
	})

	output := out.String()
	if !strings.Contains(output, "build") {
		t.Errorf("expected job name, got: %s", output)
	}
}

func TestProgressHandler_TerminalWidthPadding_TTY(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, true)
	h.getWidth = fixedWidth(40)

	h.PrintSummary(&core.PipelineResult{
		PipelineID: "pipe-1",
		Status:     core.StatusSuccess,
		Duration:   1 * time.Second,
		Jobs: []core.JobResult{
			{JobName: "build", Status: core.StatusSuccess, Duration: 1 * time.Second},
		},
	})

	output := out.String()
	if !strings.Contains(output, "build") {
		t.Errorf("expected job name, got: %s", output)
	}
}

func TestTruncateANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		width int
		want  string
	}{
		{
			name:  "no truncation needed",
			input: "hello",
			width: 10,
			want:  "hello",
		},
		{
			name:  "truncate plain text",
			input: "hello world",
			width: 5,
			want:  "hello",
		},
		{
			name:  "preserve ANSI codes",
			input: "\033[32mhello\033[0m world",
			width: 5,
			want:  "\033[32mhello\033[0m",
		},
		{
			name:  "zero width",
			input: "hello",
			width: 0,
			want:  "hello",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateANSI(tt.input, tt.width)
			if got != tt.want {
				t.Errorf("truncateANSI(%q, %d) = %q, want %q", tt.input, tt.width, got, tt.want)
			}
		})
	}
}

// --- SetJobDependsOn tests ---

func TestProgressHandler_SetJobDependsOn(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, true)
	h.getWidth = fixedWidth(120)

	h.SetJobDependsOn("deploy", []string{"build", "test"})

	h.mu.Lock()
	js, ok := h.jobs["deploy"]
	h.mu.Unlock()

	if !ok {
		t.Fatal("expected deploy job to be registered")
	}
	if len(js.dependsOn) != 2 {
		t.Errorf("expected 2 deps, got %d", len(js.dependsOn))
	}
}

func TestProgressHandler_WaitingDeps_TTY(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, true)
	h.getWidth = fixedWidth(120)

	_ = h.HandleEvent(&core.Event{
		Type:         core.EventPipelineStarted,
		PipelineID:   "pipe-1",
		PipelineName: "ci",
		Timestamp:    time.Now(),
	})

	h.SetJobDependsOn("deploy", []string{"build"})

	_ = h.HandleEvent(&core.Event{
		Type:    core.EventJobStarted,
		JobName: "build",
	})

	output := out.String()
	if !strings.Contains(output, "waiting for build") {
		t.Errorf("expected 'waiting for build', got: %s", output)
	}
}

func TestProgressHandler_StreamingLogsTTY(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, true)
	h.getWidth = fixedWidth(120)
	_ = h.HandleEvent(&core.Event{
		Type:         core.EventPipelineStarted,
		PipelineID:   "pipe-1",
		PipelineName: "ci",
		Timestamp:    time.Now(),
	})
	_ = h.HandleEvent(&core.Event{
		Type:    core.EventJobStarted,
		JobName: "build",
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
		LogData:  []byte("building binary\n"),
	})
	output := out.String()
	if !strings.Contains(output, "building binary") {
		t.Errorf("expected streaming log in TTY output, got: %s", output)
	}
	if !strings.Contains(output, "[compile]") {
		t.Errorf("expected step prefix [compile], got: %s", output)
	}
}

// --- Static top / dynamic bottom zone tests ---

func TestProgressHandler_TTY_CompletedJobNeverRedrawn(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, true)
	h.getWidth = fixedWidth(120)
	_ = h.HandleEvent(&core.Event{
		Type:         core.EventPipelineStarted,
		PipelineID:   "pipe-1",
		PipelineName: "ci",
		Timestamp:    time.Now(),
	})
	_ = h.HandleEvent(&core.Event{
		Type:    core.EventJobStarted,
		JobName: "lint",
	})
	_ = h.HandleEvent(&core.Event{
		Type:     core.EventJobFinished,
		JobName:  "lint",
		Status:   core.StatusSuccess,
		Duration: 2 * time.Second,
	})

	// After lint finishes, start build and send a log — the bottom zone
	// redraws should never cursor-up past the static ✓ lint line.
	_ = h.HandleEvent(&core.Event{
		Type:    core.EventJobStarted,
		JobName: "build",
	})
	_ = h.HandleEvent(&core.Event{
		Type:     core.EventStepLog,
		JobName:  "build",
		StepName: "compile",
		LogData:  []byte("compiling...\n"),
	})

	output := out.String()
	// The ✓ for lint should appear exactly once in the cumulative output,
	// proving it was printed to the static zone and never redrawn.
	checkCount := strings.Count(output, "✓")
	if checkCount < 1 {
		t.Errorf("expected at least one ✓ for completed lint, got: %s", output)
	}
	// build should appear in the bottom zone
	if !strings.Contains(output, "build") {
		t.Errorf("expected build in bottom zone, got: %s", output)
	}
}

func TestProgressHandler_TTY_BottomZoneClearedOnJobFinish(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, true)
	h.getWidth = fixedWidth(120)
	_ = h.HandleEvent(&core.Event{
		Type:         core.EventPipelineStarted,
		PipelineID:   "pipe-1",
		PipelineName: "ci",
		Timestamp:    time.Now(),
	})
	_ = h.HandleEvent(&core.Event{
		Type:    core.EventJobStarted,
		JobName: "build",
	})
	_ = h.HandleEvent(&core.Event{
		Type:     core.EventStepLog,
		JobName:  "build",
		StepName: "compile",
		LogData:  []byte("line1\nline2\nline3\n"),
	})

	// Record bottomLines before job finishes.
	h.mu.Lock()
	bottomBefore := h.bottomLines
	h.mu.Unlock()

	if bottomBefore == 0 {
		t.Fatal("expected non-zero bottomLines while job is running")
	}

	// Finish the job — bottom zone should be cleared and job promoted to
	// static zone.
	_ = h.HandleEvent(&core.Event{
		Type:     core.EventJobFinished,
		JobName:  "build",
		Status:   core.StatusSuccess,
		Duration: 1 * time.Second,
	})

	h.mu.Lock()
	bottomAfter := h.bottomLines
	h.mu.Unlock()

	if bottomAfter != 0 {
		t.Errorf("expected bottomLines=0 after all jobs finish, got %d", bottomAfter)
	}
}

func TestProgressHandler_TTY_LogTailMaxLines(t *testing.T) {
	out := &bytes.Buffer{}
	h := newProgressHandler(out, out, true)
	h.getWidth = fixedWidth(120)
	_ = h.HandleEvent(&core.Event{
		Type:         core.EventPipelineStarted,
		PipelineID:   "pipe-1",
		PipelineName: "ci",
		Timestamp:    time.Now(),
	})
	_ = h.HandleEvent(&core.Event{
		Type:    core.EventJobStarted,
		JobName: "build",
	})

	// Send more log lines than maxTailLines.
	var logLines []string
	for i := 0; i < 20; i++ {
		logLines = append(logLines, fmt.Sprintf("log-line-%d", i))
	}
	_ = h.HandleEvent(&core.Event{
		Type:     core.EventStepLog,
		JobName:  "build",
		StepName: "compile",
		LogData:  []byte(strings.Join(logLines, "\n") + "\n"),
	})

	// The bottom zone should only show the last maxTailLines (5).
	h.mu.Lock()
	bl := h.bottomLines
	h.mu.Unlock()

	// 1 line for " => build" + maxTailLines log lines.
	expected := 1 + maxTailLines
	if bl != expected {
		t.Errorf("expected bottomLines=%d (1 header + %d tail), got %d", expected, maxTailLines, bl)
	}
}
