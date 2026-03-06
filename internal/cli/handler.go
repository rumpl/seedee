package cli

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/rumpl/seedee/internal/core"
)

// terminalEventHandler displays pipeline events in the terminal with
// color-coded job prefixes, aligned output, and a polished summary.
type terminalEventHandler struct {
	out    io.Writer
	errOut io.Writer
	isTTY  bool

	mu          sync.Mutex
	jobColorMap map[string]string // jobName -> ANSI color code
	nextColor   int
}

// colorForJob assigns a persistent color to each job name.
func (h *terminalEventHandler) colorForJob(name string) string {
	if h.jobColorMap == nil {
		h.jobColorMap = make(map[string]string)
	}
	if c, ok := h.jobColorMap[name]; ok {
		return c
	}
	c := jobColors[h.nextColor%len(jobColors)]
	h.nextColor++
	h.jobColorMap[name] = c
	return c
}

// jobPrefix returns a colorized [job/step] prefix.
func (h *terminalEventHandler) jobPrefix(jobName, stepName string) string {
	label := fmt.Sprintf("[%s/%s]", jobName, stepName)
	c := h.colorForJob(jobName)
	return colorize(h.isTTY, c, label)
}

// HandleEvent processes a single pipeline event and displays it.
func (h *terminalEventHandler) HandleEvent(e core.Event) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	switch e.Type {
	case core.EventPipelineStarted:
		name := e.PipelineName
		if name == "" {
			name = e.PipelineID
		}
		fmt.Fprintf(h.out, "%s Pipeline %s started\n\n",
			bold(h.isTTY, "▶"), bold(h.isTTY, fmt.Sprintf("%q", name)))

	case core.EventPipelineFinished:
		// Handled by PrintSummary

	case core.EventJobStarted:
		fmt.Fprintf(h.out, "  %s %s\n",
			bold(h.isTTY, "▶"),
			colorize(h.isTTY, h.colorForJob(e.JobName), e.JobName))

	case core.EventJobFinished:
		icon, status := h.jobStatusDecoration(e.Status)
		msg := fmt.Sprintf("  %s %s", icon, colorize(h.isTTY, h.colorForJob(e.JobName), e.JobName))
		if e.Duration > 0 {
			msg += " " + dim(h.isTTY, fmt.Sprintf("(%s)", e.Duration.Round(time.Millisecond)))
		}
		if e.Error != "" {
			msg += " — " + red(h.isTTY, e.Error)
		}
		_ = status
		fmt.Fprintln(h.out, msg)
		fmt.Fprintln(h.out) // blank line after job block

	case core.EventJobSkipped:
		fmt.Fprintf(h.out, "  %s %s skipped\n\n",
			yellow(h.isTTY, "⊘"),
			e.JobName)

	case core.EventStepStarted:
		// no output for step start — the log prefix is enough

	case core.EventStepFinished:
		icon, _ := h.stepStatusDecoration(e.Status)
		msg := fmt.Sprintf("    %s %s/%s", icon, e.JobName, e.StepName)
		if e.Duration > 0 {
			msg += " " + dim(h.isTTY, fmt.Sprintf("(%s)", e.Duration.Round(time.Millisecond)))
		}
		if e.ExitCode != 0 {
			msg += dim(h.isTTY, fmt.Sprintf(" (exit code %d)", e.ExitCode))
		}
		if e.Error != "" {
			msg += " — " + red(h.isTTY, e.Error)
		}
		fmt.Fprintln(h.out, msg)

	case core.EventStepLog:
		h.writeLog(e)
	}

	return nil
}

// writeLog writes log lines with a colorized [job/step] prefix.
func (h *terminalEventHandler) writeLog(e core.Event) {
	w := h.out
	if e.IsStderr {
		w = h.errOut
	}

	prefix := h.jobPrefix(e.JobName, e.StepName)
	text := strings.TrimRight(string(e.LogData), "\n")
	if text == "" {
		return
	}
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		fmt.Fprintf(w, "    %s %s\n", prefix, line)
	}
}

// jobStatusDecoration returns the icon and label for a job status.
func (h *terminalEventHandler) jobStatusDecoration(s core.Status) (icon, label string) {
	switch s {
	case core.StatusSuccess:
		return green(h.isTTY, "✓"), "success"
	case core.StatusFailed:
		return red(h.isTTY, "✗"), "failed"
	case core.StatusCanceled:
		return yellow(h.isTTY, "⊗"), "canceled"
	case core.StatusSkipped:
		return yellow(h.isTTY, "⊘"), "skipped"
	default:
		return "?", string(s)
	}
}

// stepStatusDecoration returns the icon and label for a step status.
func (h *terminalEventHandler) stepStatusDecoration(s core.Status) (icon, label string) {
	switch s {
	case core.StatusSuccess:
		return green(h.isTTY, "✓"), "success"
	case core.StatusFailed:
		return red(h.isTTY, "✗"), "failed"
	case core.StatusCanceled:
		return yellow(h.isTTY, "⊗"), "canceled"
	default:
		return "?", string(s)
	}
}

// PrintSummary prints a final summary of the pipeline run.
func (h *terminalEventHandler) PrintSummary(result *core.PipelineResult) {
	h.mu.Lock()
	defer h.mu.Unlock()

	separator := dim(h.isTTY, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Fprintf(h.out, "\n%s\n", separator)
	fmt.Fprintf(h.out, "Pipeline: %s\n", bold(h.isTTY, result.PipelineID))

	var statusStr string
	switch result.Status {
	case core.StatusSuccess:
		statusStr = green(h.isTTY, "✓ success")
	case core.StatusFailed:
		statusStr = red(h.isTTY, "✗ failed")
	case core.StatusCanceled:
		statusStr = yellow(h.isTTY, "⊗ canceled")
	default:
		statusStr = string(result.Status)
	}
	fmt.Fprintf(h.out, "Status:   %s\n", statusStr)
	fmt.Fprintf(h.out, "Duration: %s\n\n", result.Duration.Round(time.Millisecond))

	for _, jr := range result.Jobs {
		var icon string
		switch jr.Status {
		case core.StatusSuccess:
			icon = green(h.isTTY, "✓")
		case core.StatusFailed:
			icon = red(h.isTTY, "✗")
		case core.StatusSkipped:
			icon = yellow(h.isTTY, "⊘")
		case core.StatusCanceled:
			icon = yellow(h.isTTY, "⊗")
		default:
			icon = "?"
		}
		fmt.Fprintf(h.out, "  %s %-20s %s\n", icon, jr.JobName, dim(h.isTTY, jr.Duration.Round(time.Millisecond).String()))
	}

	fmt.Fprintf(h.out, "%s\n", separator)
}
