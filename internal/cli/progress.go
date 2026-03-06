package cli

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/rumpl/seedee/internal/core"
)

// maxFailedLogLines is the maximum number of log lines shown for a failed step.
const maxFailedLogLines = 10

// stepState tracks the current state of a single step.
type stepState struct {
	jobName   string
	stepName  string
	status    core.Status
	startedAt time.Time
	duration  time.Duration
	// logLines holds recent log output; capped at maxFailedLogLines.
	logLines []string
}

// jobState tracks the current state of a single job.
type jobState struct {
	name      string
	status    core.Status
	startedAt time.Time
	duration  time.Duration
	errMsg    string
}

// progressEventHandler implements a BuildKit-style compact progress display.
// On a TTY it redraws status lines in-place using ANSI escape codes.
// On a non-TTY it prints simple line-by-line status updates.
type progressEventHandler struct {
	out    io.Writer
	errOut io.Writer
	isTTY  bool

	mu           sync.Mutex
	pipelineName string
	pipelineID   string
	pipelineStart time.Time

	// Ordered lists so display order is deterministic.
	stepOrder []string            // "job/step" keys in arrival order
	steps     map[string]*stepState

	jobOrder []string
	jobs     map[string]*jobState

	// linesDrawn tracks how many progress lines were drawn last frame,
	// so the TTY renderer can move the cursor back and overwrite them.
	linesDrawn int

	// finished is set when the pipeline finishes.
	finished bool
}

func newProgressHandler(out, errOut io.Writer, isTTY bool) *progressEventHandler {
	return &progressEventHandler{
		out:    out,
		errOut: errOut,
		isTTY:  isTTY,
		steps:  make(map[string]*stepState),
		jobs:   make(map[string]*jobState),
	}
}

func stepKey(job, step string) string { return job + "/" + step }

// HandleEvent processes pipeline events and updates the progress display.
func (h *progressEventHandler) HandleEvent(e *core.Event) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	switch e.Type {
	case core.EventPipelineStarted:
		h.pipelineName = e.PipelineName
		if h.pipelineName == "" {
			h.pipelineName = e.PipelineID
		}
		h.pipelineID = e.PipelineID
		h.pipelineStart = e.Timestamp
		if h.pipelineStart.IsZero() {
			h.pipelineStart = time.Now()
		}
		if !h.isTTY {
			_, _ = fmt.Fprintf(h.out, "[+] Running pipeline %q\n", h.pipelineName)
		}

	case core.EventPipelineFinished:
		h.finished = true
		// Final render happens in PrintSummary.

	case core.EventJobStarted:
		js := &jobState{
			name:      e.JobName,
			status:    core.StatusRunning,
			startedAt: e.Timestamp,
		}
		if js.startedAt.IsZero() {
			js.startedAt = time.Now()
		}
		if _, exists := h.jobs[e.JobName]; !exists {
			h.jobOrder = append(h.jobOrder, e.JobName)
		}
		h.jobs[e.JobName] = js

	case core.EventJobFinished:
		if js, ok := h.jobs[e.JobName]; ok {
			js.status = e.Status
			js.duration = e.Duration
			js.errMsg = e.Error
		}
		if !h.isTTY {
			h.printNonTTYJobFinished(e)
		}

	case core.EventJobSkipped:
		js := &jobState{
			name:   e.JobName,
			status: core.StatusSkipped,
		}
		if _, exists := h.jobs[e.JobName]; !exists {
			h.jobOrder = append(h.jobOrder, e.JobName)
		}
		h.jobs[e.JobName] = js
		if !h.isTTY {
			_, _ = fmt.Fprintf(h.out, " ⊘ %s skipped\n", e.JobName)
		}

	case core.EventStepStarted:
		key := stepKey(e.JobName, e.StepName)
		ss := &stepState{
			jobName:   e.JobName,
			stepName:  e.StepName,
			status:    core.StatusRunning,
			startedAt: e.Timestamp,
		}
		if ss.startedAt.IsZero() {
			ss.startedAt = time.Now()
		}
		if _, exists := h.steps[key]; !exists {
			h.stepOrder = append(h.stepOrder, key)
		}
		h.steps[key] = ss
		if !h.isTTY {
			_, _ = fmt.Fprintf(h.out, " => %s/%s\n", e.JobName, e.StepName)
		}

	case core.EventStepFinished:
		key := stepKey(e.JobName, e.StepName)
		ss, ok := h.steps[key]
		if !ok {
			ss = &stepState{
				jobName:  e.JobName,
				stepName: e.StepName,
			}
			h.stepOrder = append(h.stepOrder, key)
			h.steps[key] = ss
		}
		ss.status = e.Status
		ss.duration = e.Duration
		if !h.isTTY {
			h.printNonTTYStepFinished(e, ss)
		}

	case core.EventStepLog:
		key := stepKey(e.JobName, e.StepName)
		ss, ok := h.steps[key]
		if !ok {
			ss = &stepState{
				jobName:   e.JobName,
				stepName:  e.StepName,
				status:    core.StatusRunning,
				startedAt: time.Now(),
			}
			h.stepOrder = append(h.stepOrder, key)
			h.steps[key] = ss
		}
		// Store last N lines for failure output.
		text := strings.TrimRight(string(e.LogData), "\n")
		if text != "" {
			lines := strings.Split(text, "\n")
			ss.logLines = append(ss.logLines, lines...)
			if len(ss.logLines) > maxFailedLogLines {
				ss.logLines = ss.logLines[len(ss.logLines)-maxFailedLogLines:]
			}
		}
	}

	// Redraw the TTY progress display on every event.
	if h.isTTY && !h.finished {
		h.redraw()
	}

	return nil
}

// redraw clears the previously drawn lines and redraws the current progress.
// Must be called with h.mu held.
func (h *progressEventHandler) redraw() {
	// Move cursor up to overwrite previous frame.
	if h.linesDrawn > 0 {
		_, _ = fmt.Fprintf(h.out, "\033[%dA", h.linesDrawn)
	}

	var lines []string

	// Header line
	elapsed := time.Since(h.pipelineStart).Round(100 * time.Millisecond)
	lines = append(lines, fmt.Sprintf("\033[2K%s Running pipeline %q %s",
		bold(true, "[+]"),
		h.pipelineName,
		dim(true, elapsed.String())))

	// Step lines
	for _, key := range h.stepOrder {
		ss := h.steps[key]
		line := h.formatStepLine(ss)
		lines = append(lines, "\033[2K"+line)

		// If step failed, show last log lines.
		if ss.status == core.StatusFailed && len(ss.logLines) > 0 {
			for _, logLine := range ss.logLines {
				lines = append(lines, "\033[2K"+dim(true, "   │ ")+logLine)
			}
		}
	}

	output := strings.Join(lines, "\n") + "\n"
	_, _ = fmt.Fprint(h.out, output)
	h.linesDrawn = len(lines)
}

// formatStepLine formats a single step status line.
func (h *progressEventHandler) formatStepLine(ss *stepState) string {
	label := ss.jobName + "/" + ss.stepName

	switch ss.status {
	case core.StatusSuccess:
		dur := ss.duration.Round(100 * time.Millisecond)
		return fmt.Sprintf(" %s %-60s %s",
			green(h.isTTY, "✓"),
			label,
			dim(h.isTTY, dur.String()))

	case core.StatusFailed:
		dur := ss.duration.Round(100 * time.Millisecond)
		return fmt.Sprintf(" %s %-60s %s",
			red(h.isTTY, "✗"),
			label,
			dim(h.isTTY, dur.String()))

	case core.StatusCanceled:
		return fmt.Sprintf(" %s %-60s",
			yellow(h.isTTY, "⊗"),
			label)

	default: // running / pending
		elapsed := time.Since(ss.startedAt).Round(100 * time.Millisecond)
		return fmt.Sprintf(" %s %-60s %s",
			bold(h.isTTY, "=>"),
			label,
			dim(h.isTTY, elapsed.String()))
	}
}

// printNonTTYStepFinished prints a step completion line for non-TTY output.
func (h *progressEventHandler) printNonTTYStepFinished(e *core.Event, ss *stepState) {
	icon := "✓"
	if e.Status == core.StatusFailed {
		icon = "✗"
	} else if e.Status == core.StatusCanceled {
		icon = "⊗"
	}

	dur := ""
	if e.Duration > 0 {
		dur = " " + e.Duration.Round(100*time.Millisecond).String()
	}

	_, _ = fmt.Fprintf(h.out, " %s %s/%s%s\n", icon, e.JobName, e.StepName, dur)

	// Show log tail on failure.
	if e.Status == core.StatusFailed && len(ss.logLines) > 0 {
		for _, line := range ss.logLines {
			_, _ = fmt.Fprintf(h.out, "   │ %s\n", line)
		}
	}
}

// printNonTTYJobFinished prints a job completion line for non-TTY output.
func (h *progressEventHandler) printNonTTYJobFinished(e *core.Event) {
	icon := "✓"
	if e.Status == core.StatusFailed {
		icon = "✗"
	} else if e.Status == core.StatusCanceled {
		icon = "⊗"
	}

	dur := ""
	if e.Duration > 0 {
		dur = " " + e.Duration.Round(100*time.Millisecond).String()
	}

	msg := fmt.Sprintf(" %s %s%s", icon, e.JobName, dur)
	if e.Error != "" {
		msg += " — " + e.Error
	}
	_, _ = fmt.Fprintln(h.out, msg)
}

// PrintSummary prints the final pipeline summary.
func (h *progressEventHandler) PrintSummary(result *core.PipelineResult) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.isTTY {
		h.printTTYSummary(result)
	} else {
		h.printPlainSummary(result)
	}
}

// printTTYSummary redraws with the final completed state.
func (h *progressEventHandler) printTTYSummary(result *core.PipelineResult) {
	// Clear previous progress lines.
	if h.linesDrawn > 0 {
		_, _ = fmt.Fprintf(h.out, "\033[%dA", h.linesDrawn)
	}

	pipelineName := h.pipelineName
	if pipelineName == "" {
		pipelineName = result.PipelineID
	}

	var lines []string

	// Header with final status.
	statusWord := "completed"
	if result.Status == core.StatusFailed {
		statusWord = "failed"
	} else if result.Status == core.StatusCanceled {
		statusWord = "canceled"
	}

	lines = append(lines, fmt.Sprintf("\033[2K%s Pipeline %q %s in %s",
		bold(true, "[+]"),
		pipelineName,
		statusWord,
		result.Duration.Round(100*time.Millisecond)))

	// Job summary lines.
	for _, jr := range result.Jobs {
		var icon string
		switch jr.Status {
		case core.StatusSuccess:
			icon = green(true, "✓")
		case core.StatusFailed:
			icon = red(true, "✗")
		case core.StatusSkipped:
			icon = yellow(true, "⊘")
		case core.StatusCanceled:
			icon = yellow(true, "⊗")
		default:
			icon = "?"
		}
		dur := jr.Duration.Round(100 * time.Millisecond)
		lines = append(lines, fmt.Sprintf("\033[2K %s %-60s %s",
			icon,
			jr.JobName,
			dim(true, dur.String())))

		// For failed jobs, show failed step log tails.
		if jr.Status == core.StatusFailed {
			for _, key := range h.stepOrder {
				ss := h.steps[key]
				if ss.jobName == jr.JobName && ss.status == core.StatusFailed && len(ss.logLines) > 0 {
					for _, logLine := range ss.logLines {
						lines = append(lines, "\033[2K"+dim(true, "   │ ")+logLine)
					}
				}
			}
		}
	}

	output := strings.Join(lines, "\n") + "\n"
	_, _ = fmt.Fprint(h.out, output)
	h.linesDrawn = 0 // no more redraws
}

// printPlainSummary prints the summary without ANSI codes.
func (h *progressEventHandler) printPlainSummary(result *core.PipelineResult) {
	pipelineName := h.pipelineName
	if pipelineName == "" {
		pipelineName = result.PipelineID
	}

	statusWord := "completed"
	if result.Status == core.StatusFailed {
		statusWord = "failed"
	} else if result.Status == core.StatusCanceled {
		statusWord = "canceled"
	}

	_, _ = fmt.Fprintf(h.out, "[+] Pipeline %q %s in %s\n",
		pipelineName,
		statusWord,
		result.Duration.Round(100*time.Millisecond))

	for _, jr := range result.Jobs {
		icon := "✓"
		switch jr.Status {
		case core.StatusFailed:
			icon = "✗"
		case core.StatusSkipped:
			icon = "⊘"
		case core.StatusCanceled:
			icon = "⊗"
		}
		_, _ = fmt.Fprintf(h.out, " %s %-60s %s\n",
			icon,
			jr.JobName,
			jr.Duration.Round(100*time.Millisecond))
	}
}
