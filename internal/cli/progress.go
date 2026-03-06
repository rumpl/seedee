package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"

	"github.com/rumpl/seedee/internal/core"
)

// maxFailedLogLines is the maximum number of log lines shown for a failed step.
const maxFailedLogLines = 10

// termWidth returns the current terminal width, falling back to 80.
func termWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		return 80
	}
	return width
}

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
	dependsOn []string
}

// progressEventHandler implements a BuildKit-style progress display that
// streams log output indented under each job header.
//
// On a TTY it redraws status lines in-place using ANSI escape codes.
// On a non-TTY it prints simple line-by-line status updates.
type progressEventHandler struct {
	out    io.Writer
	errOut io.Writer
	isTTY  bool

	// getWidth returns the current terminal width. Pluggable for tests.
	getWidth func() int

	mu            sync.Mutex
	pipelineName  string
	pipelineID    string
	pipelineStart time.Time

	// Ordered lists so display order is deterministic.
	stepOrder []string // "job/step" keys in arrival order
	steps     map[string]*stepState

	jobOrder []string
	jobs     map[string]*jobState

	// printedJobHeaders tracks which jobs have had their " => job" header printed (non-TTY).
	printedJobHeaders map[string]bool

	// linesDrawn tracks how many progress lines were drawn last frame,
	// so the TTY renderer can move the cursor back and overwrite them.
	linesDrawn int

	// finished is set when the pipeline finishes.
	finished bool
}

func newProgressHandler(out, errOut io.Writer, isTTY bool) *progressEventHandler {
	return &progressEventHandler{
		out:               out,
		errOut:            errOut,
		isTTY:             isTTY,
		getWidth:          termWidth,
		steps:             make(map[string]*stepState),
		jobs:              make(map[string]*jobState),
		printedJobHeaders: make(map[string]bool),
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
		// Preserve dependsOn from a previous pending entry.
		if prev, exists := h.jobs[e.JobName]; exists {
			js.dependsOn = prev.dependsOn
		}
		if _, exists := h.jobs[e.JobName]; !exists {
			h.jobOrder = append(h.jobOrder, e.JobName)
		}
		h.jobs[e.JobName] = js
		if !h.isTTY {
			h.ensureJobHeader(e.JobName)
		}

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
			h.ensureJobHeader(e.JobName)
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
			// Stream log lines to output in non-TTY mode.
			if !h.isTTY {
				h.ensureJobHeader(e.JobName)
				h.printLogLines(e.JobName, e.StepName, lines)
			}
		}
	}

	// Redraw the TTY progress display on every event.
	if h.isTTY && !h.finished {
		h.redraw()
	}

	return nil
}

// SetJobDependsOn records the dependency list for a job so we can show
// "(waiting for X, Y)" in the progress output. It must be called before
// events are processed for the job.
func (h *progressEventHandler) SetJobDependsOn(jobName string, deps []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	js, ok := h.jobs[jobName]
	if !ok {
		js = &jobState{
			name:   jobName,
			status: core.StatusPending,
		}
		h.jobOrder = append(h.jobOrder, jobName)
		h.jobs[jobName] = js
	}
	js.dependsOn = deps
}

// ensureJobHeader prints the " => jobName" header once per job (non-TTY).
func (h *progressEventHandler) ensureJobHeader(jobName string) {
	if h.printedJobHeaders[jobName] {
		return
	}
	h.printedJobHeaders[jobName] = true

	// Check if this job is waiting on dependencies that haven't finished.
	waitingFor := h.waitingDeps(jobName)
	if len(waitingFor) > 0 {
		_, _ = fmt.Fprintf(h.out, "\n => %s (waiting for %s)\n", jobName, strings.Join(waitingFor, ", "))
	} else {
		_, _ = fmt.Fprintf(h.out, "\n => %s\n", jobName)
	}
}

// waitingDeps returns the subset of a job's dependencies that have not yet finished.
func (h *progressEventHandler) waitingDeps(jobName string) []string {
	js, ok := h.jobs[jobName]
	if !ok || len(js.dependsOn) == 0 {
		return nil
	}
	var waiting []string
	for _, dep := range js.dependsOn {
		depJob, exists := h.jobs[dep]
		if !exists || (depJob.status != core.StatusSuccess && depJob.status != core.StatusFailed && depJob.status != core.StatusSkipped && depJob.status != core.StatusCanceled) {
			waiting = append(waiting, dep)
		}
	}
	return waiting
}

// printLogLines writes indented log lines for a step (non-TTY).
func (h *progressEventHandler) printLogLines(jobName, stepName string, lines []string) {
	width := h.getWidth()
	prefix := fmt.Sprintf("    [%s] ", stepName)
	for _, line := range lines {
		full := prefix + line
		if len(full) > width {
			full = full[:width]
		}
		_, _ = fmt.Fprintln(h.out, full)
	}
}

// redraw clears the previously drawn lines and redraws the current progress.
// Must be called with h.mu held.
func (h *progressEventHandler) redraw() {
	width := h.getWidth()

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

	// Job-oriented display: each job gets a section.
	for _, jobName := range h.jobOrder {
		js := h.jobs[jobName]

		switch js.status {
		case core.StatusSuccess:
			dur := js.duration.Round(100 * time.Millisecond)
			lines = append(lines, fmt.Sprintf("\033[2K %s %s %s",
				green(true, "✓"),
				jobName,
				dim(true, dur.String())))

		case core.StatusFailed:
			dur := js.duration.Round(100 * time.Millisecond)
			jobLine := fmt.Sprintf("\033[2K %s %s %s",
				red(true, "✗"),
				jobName,
				dim(true, dur.String()))
			lines = append(lines, jobLine)
			// Show failed step log tails.
			for _, key := range h.stepOrder {
				ss := h.steps[key]
				if ss.jobName == jobName && ss.status == core.StatusFailed && len(ss.logLines) > 0 {
					for _, logLine := range ss.logLines {
						full := "\033[2K" + dim(true, "   │ ") + logLine
						full = truncateANSI(full, width)
						lines = append(lines, full)
					}
				}
			}

		case core.StatusSkipped:
			lines = append(lines, fmt.Sprintf("\033[2K %s %s skipped",
				yellow(true, "⊘"), jobName))

		case core.StatusCanceled:
			lines = append(lines, fmt.Sprintf("\033[2K %s %s",
				yellow(true, "⊗"), jobName))

		case core.StatusRunning:
			lines = append(lines, fmt.Sprintf("\033[2K %s %s",
				bold(true, "=>"), jobName))
			// Show log tails for running steps.
			for _, key := range h.stepOrder {
				ss := h.steps[key]
				if ss.jobName != jobName {
					continue
				}
				// Show last few log lines for running steps.
				for _, logLine := range ss.logLines {
					full := "\033[2K    " + dim(true, fmt.Sprintf("[%s] ", ss.stepName)) + logLine
					full = truncateANSI(full, width)
					lines = append(lines, full)
				}
			}

		default: // pending
			waitFor := h.waitingDeps(jobName)
			if len(waitFor) > 0 {
				lines = append(lines, fmt.Sprintf("\033[2K %s %s %s",
					bold(true, "=>"),
					jobName,
					dim(true, fmt.Sprintf("(waiting for %s)", strings.Join(waitFor, ", ")))))
			} else {
				lines = append(lines, fmt.Sprintf("\033[2K %s %s",
					bold(true, "=>"), jobName))
			}
		}
	}

	output := strings.Join(lines, "\n") + "\n"
	_, _ = fmt.Fprint(h.out, output)
	h.linesDrawn = len(lines)
}

// truncateANSI truncates a string that may contain ANSI escapes so that the
// visible (non-escape) character count does not exceed width. This is a
// best-effort helper; it counts visible runes naively.
func truncateANSI(s string, width int) string {
	if width <= 0 {
		return s
	}
	visible := 0
	inEsc := false
	for i, r := range s {
		if inEsc {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEsc = false
			}
			continue
		}
		if r == '\033' {
			inEsc = true
			continue
		}
		visible++
		if visible > width {
			return s[:i]
		}
	}
	return s
}

// printNonTTYStepFinished prints a step completion line for non-TTY output.
func (h *progressEventHandler) printNonTTYStepFinished(e *core.Event, ss *stepState) {
	// In the streaming-log style, step completion on failure shows the log tail.
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
	width := h.getWidth()

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

		// Pad job name dynamically based on terminal width.
		padWidth := width - 10 // leave room for icon + duration
		if padWidth < 20 {
			padWidth = 20
		}
		lines = append(lines, fmt.Sprintf("\033[2K %s %-*s %s",
			icon,
			padWidth,
			jr.JobName,
			dim(true, dur.String())))

		// For failed jobs, show failed step log tails.
		if jr.Status == core.StatusFailed {
			for _, key := range h.stepOrder {
				ss := h.steps[key]
				if ss.jobName == jr.JobName && ss.status == core.StatusFailed && len(ss.logLines) > 0 {
					for _, logLine := range ss.logLines {
						full := "\033[2K" + dim(true, "   │ ") + logLine
						full = truncateANSI(full, width)
						lines = append(lines, full)
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
	width := h.getWidth()

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

	_, _ = fmt.Fprintf(h.out, "\n[+] Pipeline %q %s in %s\n",
		pipelineName,
		statusWord,
		result.Duration.Round(100*time.Millisecond))

	padWidth := width - 10
	if padWidth < 20 {
		padWidth = 20
	}
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
		_, _ = fmt.Fprintf(h.out, " %s %-*s %s\n",
			icon,
			padWidth,
			jr.JobName,
			jr.Duration.Round(100*time.Millisecond))
	}
}
