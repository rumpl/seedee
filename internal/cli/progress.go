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

// maxTailLines is the number of log tail lines shown under a running job in the
// dynamic bottom zone.
const maxTailLines = 5

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
	jobName  string
	stepName string
	status   core.Status
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

// progressEventHandler implements a BuildKit-style progress display with two
// output zones:
//
//   - Top zone (static): the pipeline header and completed/skipped job lines.
//     These are printed once and never redrawn.
//   - Bottom zone (dynamic): running/pending jobs with their last few log lines.
//     This zone is erased and redrawn on each event using ANSI cursor-up codes.
//
// On a non-TTY the handler falls back to a simple forward-printing mode that
// prints job headers, streamed log lines, and completion lines sequentially
// without any cursor movement.
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

	// printedJobHeaders tracks which jobs have had their " => job" header
	// printed (non-TTY only).
	printedJobHeaders map[string]bool

	// headerPrinted is true once the "[+] Running pipeline" line has been
	// printed to the static top zone (TTY mode).
	headerPrinted bool

	// bottomLines is the number of lines currently occupying the dynamic
	// bottom zone so we know how far to cursor-up before redrawing.
	bottomLines int

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
		if h.isTTY {
			// Print the header into the static top zone.
			_, _ = fmt.Fprintf(h.out, "%s Running pipeline %q\n",
				bold(true, "[+]"), h.pipelineName)
			h.headerPrinted = true
		} else {
			_, _ = fmt.Fprintf(h.out, "[+] Running pipeline %q\n", h.pipelineName)
		}

	case core.EventPipelineFinished:
		h.finished = true

	case core.EventJobStarted:
		js := &jobState{
			name:      e.JobName,
			status:    core.StatusRunning,
			startedAt: e.Timestamp,
		}
		if js.startedAt.IsZero() {
			js.startedAt = time.Now()
		}
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
			jobName:  e.JobName,
			stepName: e.StepName,
			status:   core.StatusRunning,
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
		if !h.isTTY {
			h.printNonTTYStepFinished(e, ss)
		}

	case core.EventStepLog:
		key := stepKey(e.JobName, e.StepName)
		ss, ok := h.steps[key]
		if !ok {
			ss = &stepState{
				jobName:  e.JobName,
				stepName: e.StepName,
				status:   core.StatusRunning,
			}
			h.stepOrder = append(h.stepOrder, key)
			h.steps[key] = ss
		}
		text := strings.TrimRight(string(e.LogData), "\n")
		if text != "" {
			lines := strings.Split(text, "\n")
			ss.logLines = append(ss.logLines, lines...)
			if len(ss.logLines) > maxFailedLogLines {
				ss.logLines = ss.logLines[len(ss.logLines)-maxFailedLogLines:]
			}
			if !h.isTTY {
				h.ensureJobHeader(e.JobName)
				h.printLogLines(e.StepName, lines)
			}
		}
	}

	// Redraw the TTY bottom zone on every event.
	if h.isTTY && !h.finished {
		h.redrawBottomZone()
	}

	return nil
}

// SetJobDependsOn records the dependency list for a job so we can show
// "(waiting for X, Y)" in the progress output.
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

// ---------------------------------------------------------------------------
// Non-TTY helpers — simple forward-printing, no cursor movement
// ---------------------------------------------------------------------------

// ensureJobHeader prints the " => jobName" header once per job (non-TTY).
func (h *progressEventHandler) ensureJobHeader(jobName string) {
	if h.printedJobHeaders[jobName] {
		return
	}
	h.printedJobHeaders[jobName] = true

	waitingFor := h.waitingDeps(jobName)
	if len(waitingFor) > 0 {
		_, _ = fmt.Fprintf(h.out, "\n => %s (waiting for %s)\n", jobName, strings.Join(waitingFor, ", "))
	} else {
		_, _ = fmt.Fprintf(h.out, "\n => %s\n", jobName)
	}
}

// printLogLines writes indented log lines for a step (non-TTY).
func (h *progressEventHandler) printLogLines(stepName string, lines []string) {
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

// printNonTTYStepFinished prints failure log tails for a failed step (non-TTY).
func (h *progressEventHandler) printNonTTYStepFinished(e *core.Event, ss *stepState) {
	if e.Status == core.StatusFailed && len(ss.logLines) > 0 {
		for _, line := range ss.logLines {
			_, _ = fmt.Fprintf(h.out, "   │ %s\n", line)
		}
	}
}

// printNonTTYJobFinished prints a job completion line (non-TTY).
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

// ---------------------------------------------------------------------------
// TTY rendering — static top zone + dynamic bottom zone
// ---------------------------------------------------------------------------

// clearBottomZone erases the current bottom zone by moving the cursor up and
// clearing each line, leaving the cursor at the start of where the bottom zone
// was. Must be called with h.mu held.
func (h *progressEventHandler) clearBottomZone() {
	if h.bottomLines <= 0 {
		return
	}
	// Move cursor up N lines, clearing each.
	_, _ = fmt.Fprintf(h.out, "\033[%dA", h.bottomLines)
	for i := 0; i < h.bottomLines; i++ {
		_, _ = fmt.Fprint(h.out, "\033[2K\n")
	}
	// Move back up to the start of the cleared region.
	_, _ = fmt.Fprintf(h.out, "\033[%dA", h.bottomLines)
	h.bottomLines = 0
}

// redrawBottomZone clears the dynamic bottom zone, promotes any newly-finished
// jobs to the static top zone (by printing their completion lines), then redraws
// the running/pending jobs with their log tails. Must be called with h.mu held.
func (h *progressEventHandler) redrawBottomZone() {
	width := h.getWidth()

	// 1. Clear the previous bottom zone.
	h.clearBottomZone()

	// 2. Promote completed/skipped/failed jobs to the static top zone.
	//    "Promoting" just means printing them now — they become part of
	//    the scrollback and are never touched again.
	for _, jobName := range h.jobOrder {
		js := h.jobs[jobName]
		// Skip jobs that are still running/pending — they go in the bottom zone.
		if js.status == core.StatusRunning || js.status == core.StatusPending {
			continue
		}
		// Skip jobs we already promoted.
		if h.printedJobHeaders[jobName] {
			continue
		}
		h.printedJobHeaders[jobName] = true
		h.printStaticJobLine(js, width)
	}

	// 3. Build the dynamic bottom zone lines.
	var bottomBuf strings.Builder
	lineCount := 0

	for _, jobName := range h.jobOrder {
		js := h.jobs[jobName]
		if js.status != core.StatusRunning && js.status != core.StatusPending {
			continue
		}

		// Job status line
		switch js.status {
		case core.StatusRunning:
			_, _ = fmt.Fprintf(&bottomBuf, " %s %s\n",
				bold(true, "=>"), jobName)
			lineCount++

			// Show log tails for running steps of this job.
			for _, key := range h.stepOrder {
				ss := h.steps[key]
				if ss.jobName != jobName {
					continue
				}
				tail := ss.logLines
				if len(tail) > maxTailLines {
					tail = tail[len(tail)-maxTailLines:]
				}
				for _, logLine := range tail {
					full := "    " + dim(true, fmt.Sprintf("[%s] ", ss.stepName)) + logLine
					full = truncateANSI(full, width)
					_, _ = fmt.Fprintln(&bottomBuf, full)
					lineCount++
				}
			}

		default: // pending
			waitFor := h.waitingDeps(jobName)
			if len(waitFor) > 0 {
				_, _ = fmt.Fprintf(&bottomBuf, " %s %s %s\n",
					bold(true, "=>"),
					jobName,
					dim(true, fmt.Sprintf("(waiting for %s)", strings.Join(waitFor, ", "))))
			} else {
				_, _ = fmt.Fprintf(&bottomBuf, " %s %s\n",
					bold(true, "=>"), jobName)
			}
			lineCount++
		}
	}

	// 4. Write the bottom zone.
	if lineCount > 0 {
		_, _ = fmt.Fprint(h.out, bottomBuf.String())
	}
	h.bottomLines = lineCount
}

// printStaticJobLine prints a single completed/skipped/canceled job line into
// the static top zone. Must be called with h.mu held.
func (h *progressEventHandler) printStaticJobLine(js *jobState, _ int) {
	switch js.status {
	case core.StatusSuccess:
		dur := js.duration.Round(100 * time.Millisecond)
		_, _ = fmt.Fprintf(h.out, " %s %s %s\n",
			green(true, "✓"),
			js.name,
			dim(true, dur.String()))

	case core.StatusFailed:
		dur := js.duration.Round(100 * time.Millisecond)
		_, _ = fmt.Fprintf(h.out, " %s %s %s",
			red(true, "✗"),
			js.name,
			dim(true, dur.String()))
		if js.errMsg != "" {
			_, _ = fmt.Fprintf(h.out, " — %s", red(true, js.errMsg))
		}
		_, _ = fmt.Fprint(h.out, "\n")
		// Show failed step log tails.
		for _, key := range h.stepOrder {
			ss := h.steps[key]
			if ss.jobName == js.name && ss.status == core.StatusFailed && len(ss.logLines) > 0 {
				for _, logLine := range ss.logLines {
					_, _ = fmt.Fprintf(h.out, "%s%s\n",
						dim(true, "   │ "), logLine)
				}
			}
		}

	case core.StatusSkipped:
		_, _ = fmt.Fprintf(h.out, " %s %s skipped\n",
			yellow(true, "⊘"), js.name)

	case core.StatusCanceled:
		_, _ = fmt.Fprintf(h.out, " %s %s\n",
			yellow(true, "⊗"), js.name)
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

// truncateANSI truncates a string that may contain ANSI escapes so that the
// visible (non-escape) character count does not exceed width.
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

// ---------------------------------------------------------------------------
// PrintSummary — final output after the pipeline finishes
// ---------------------------------------------------------------------------

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

// printTTYSummary clears the bottom zone and prints the final summary into the
// static zone.
func (h *progressEventHandler) printTTYSummary(result *core.PipelineResult) {
	// Clear any remaining bottom zone.
	h.clearBottomZone()

	pipelineName := h.pipelineName
	if pipelineName == "" {
		pipelineName = result.PipelineID
	}

	width := h.getWidth()

	// Promote any remaining jobs that haven't been printed yet.
	for _, jobName := range h.jobOrder {
		if h.printedJobHeaders[jobName] {
			continue
		}
		h.printedJobHeaders[jobName] = true
		if js, ok := h.jobs[jobName]; ok {
			h.printStaticJobLine(js, width)
		}
	}

	// Also print lines for jobs that only appear in the result (e.g. if
	// the event stream was sparse).
	for _, jr := range result.Jobs {
		if h.printedJobHeaders[jr.JobName] {
			continue
		}
		h.printedJobHeaders[jr.JobName] = true
		h.printSummaryJobLine(jr, width)
	}

	// For failed jobs, show the error log tail even if the completion line
	// was already printed to the static zone during event handling.
	for _, jr := range result.Jobs {
		if jr.Status != core.StatusFailed {
			continue
		}
		for _, key := range h.stepOrder {
			ss := h.steps[key]
			if ss.jobName == jr.JobName && ss.status == core.StatusFailed && len(ss.logLines) > 0 {
				for _, logLine := range ss.logLines {
					full := dim(true, "   │ ") + logLine
					full = truncateANSI(full, width)
					_, _ = fmt.Fprintln(h.out, full)
				}
			}
		}
	}

	statusWord := "completed"
	if result.Status == core.StatusFailed {
		statusWord = "failed"
	} else if result.Status == core.StatusCanceled {
		statusWord = "canceled"
	}

	_, _ = fmt.Fprintf(h.out, "\n%s Pipeline %q %s in %s\n",
		bold(true, "[+]"),
		pipelineName,
		statusWord,
		result.Duration.Round(100*time.Millisecond))
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

// printSummaryJobLine prints a job result line with ANSI colors (used in TTY
// summary when we only have a JobResult, not a jobState).
func (h *progressEventHandler) printSummaryJobLine(jr core.JobResult, width int) {
	padWidth := width - 10
	if padWidth < 20 {
		padWidth = 20
	}
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
	_, _ = fmt.Fprintf(h.out, " %s %-*s %s\n",
		icon, padWidth, jr.JobName, dim(true, dur.String()))

	// For failed jobs, show failed step log tails.
	if jr.Status == core.StatusFailed {
		for _, key := range h.stepOrder {
			ss := h.steps[key]
			if ss.jobName == jr.JobName && ss.status == core.StatusFailed && len(ss.logLines) > 0 {
				for _, logLine := range ss.logLines {
					full := dim(true, "   │ ") + logLine
					full = truncateANSI(full, width)
					_, _ = fmt.Fprintln(h.out, full)
				}
			}
		}
	}
}
