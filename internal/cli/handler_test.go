package cli

import (
	"bytes"
	"testing"
	"time"

	"github.com/rumpl/seedee/internal/core"
)

func TestTerminalEventHandler_PipelineStarted(t *testing.T) {
	out := &bytes.Buffer{}
	h := &terminalEventHandler{out: out, errOut: out}
	err := h.HandleEvent(core.Event{
		Type:       core.EventPipelineStarted,
		PipelineID: "pipe-123",
	})
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("pipe-123")) {
		t.Errorf("output missing pipeline ID, got: %s", out.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("started")) {
		t.Errorf("output missing 'started', got: %s", out.String())
	}
}

func TestTerminalEventHandler_PipelineFinished(t *testing.T) {
	out := &bytes.Buffer{}
	h := &terminalEventHandler{out: out, errOut: out}
	err := h.HandleEvent(core.Event{
		Type:       core.EventPipelineFinished,
		PipelineID: "pipe-123",
		Status:     core.StatusSuccess,
		Duration:   10 * time.Second,
	})
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	output := out.String()
	if !bytes.Contains(out.Bytes(), []byte("✓")) {
		t.Errorf("output missing success icon, got: %s", output)
	}
	if !bytes.Contains(out.Bytes(), []byte("10s")) {
		t.Errorf("output missing duration, got: %s", output)
	}
}

func TestTerminalEventHandler_PipelineFinishedWithError(t *testing.T) {
	out := &bytes.Buffer{}
	h := &terminalEventHandler{out: out, errOut: out}
	err := h.HandleEvent(core.Event{
		Type:       core.EventPipelineFinished,
		PipelineID: "pipe-123",
		Status:     core.StatusFailed,
		Error:      "something broke",
	})
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	output := out.String()
	if !bytes.Contains(out.Bytes(), []byte("✗")) {
		t.Errorf("output missing failure icon, got: %s", output)
	}
	if !bytes.Contains(out.Bytes(), []byte("something broke")) {
		t.Errorf("output missing error message, got: %s", output)
	}
}

func TestTerminalEventHandler_JobStarted(t *testing.T) {
	out := &bytes.Buffer{}
	h := &terminalEventHandler{out: out, errOut: out}
	err := h.HandleEvent(core.Event{
		Type:    core.EventJobStarted,
		JobName: "build",
	})
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("build")) {
		t.Errorf("output missing job name, got: %s", out.String())
	}
}

func TestTerminalEventHandler_JobFinished(t *testing.T) {
	out := &bytes.Buffer{}
	h := &terminalEventHandler{out: out, errOut: out}
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
	if !bytes.Contains(out.Bytes(), []byte("build")) {
		t.Errorf("output missing job name, got: %s", output)
	}
	if !bytes.Contains(out.Bytes(), []byte("5s")) {
		t.Errorf("output missing duration, got: %s", output)
	}
}

func TestTerminalEventHandler_JobSkipped(t *testing.T) {
	out := &bytes.Buffer{}
	h := &terminalEventHandler{out: out, errOut: out}
	err := h.HandleEvent(core.Event{
		Type:    core.EventJobSkipped,
		JobName: "deploy",
	})
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("deploy")) {
		t.Errorf("output missing job name, got: %s", out.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("skipped")) {
		t.Errorf("output missing 'skipped', got: %s", out.String())
	}
}

func TestTerminalEventHandler_StepStartedVerbose(t *testing.T) {
	out := &bytes.Buffer{}
	h := &terminalEventHandler{out: out, errOut: out, verbose: true}
	err := h.HandleEvent(core.Event{
		Type:     core.EventStepStarted,
		JobName:  "build",
		StepName: "compile",
	})
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("build/compile")) {
		t.Errorf("output missing step info, got: %s", out.String())
	}
}

func TestTerminalEventHandler_StepStartedNonVerbose(t *testing.T) {
	out := &bytes.Buffer{}
	h := &terminalEventHandler{out: out, errOut: out, verbose: false}
	err := h.HandleEvent(core.Event{
		Type:     core.EventStepStarted,
		JobName:  "build",
		StepName: "compile",
	})
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output in non-verbose mode, got: %s", out.String())
	}
}

func TestTerminalEventHandler_StepFinished(t *testing.T) {
	out := &bytes.Buffer{}
	h := &terminalEventHandler{out: out, errOut: out}
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
	if !bytes.Contains(out.Bytes(), []byte("build/compile")) {
		t.Errorf("output missing step info, got: %s", output)
	}
	if !bytes.Contains(out.Bytes(), []byte("exit code 1")) {
		t.Errorf("output missing exit code, got: %s", output)
	}
	if !bytes.Contains(out.Bytes(), []byte("compilation error")) {
		t.Errorf("output missing error, got: %s", output)
	}
}

func TestTerminalEventHandler_StepLogStdout(t *testing.T) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	h := &terminalEventHandler{out: out, errOut: errOut}
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
	if !bytes.Contains(out.Bytes(), []byte("building...\n")) {
		t.Errorf("stdout missing log data, got: %s", out.String())
	}
	if errOut.Len() != 0 {
		t.Errorf("stderr should be empty, got: %s", errOut.String())
	}
}

func TestTerminalEventHandler_StepLogStderr(t *testing.T) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	h := &terminalEventHandler{out: out, errOut: errOut}
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
	if !bytes.Contains(errOut.Bytes(), []byte("warning: something\n")) {
		t.Errorf("stderr missing log data, got: %s", errOut.String())
	}
}

func TestTerminalEventHandler_StepLogVerbosePrefix(t *testing.T) {
	out := &bytes.Buffer{}
	h := &terminalEventHandler{out: out, errOut: out, verbose: true}
	err := h.HandleEvent(core.Event{
		Type:     core.EventStepLog,
		JobName:  "build",
		StepName: "compile",
		LogData:  []byte("data\n"),
		IsStderr: false,
	})
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("[build/compile]")) {
		t.Errorf("verbose output missing prefix, got: %s", out.String())
	}
}

func TestStatusIconForCore(t *testing.T) {
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
			got := statusIconForCore(tt.status)
			if got != tt.want {
				t.Errorf("statusIconForCore(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}
