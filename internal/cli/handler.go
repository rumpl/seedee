package cli

import (
	"fmt"
	"io"

	"github.com/rumpl/seedee/internal/core"
)

// terminalEventHandler displays pipeline events in the terminal.
type terminalEventHandler struct {
	out     io.Writer
	errOut  io.Writer
	verbose bool
}

// HandleEvent processes a single pipeline event and displays it.
func (h *terminalEventHandler) HandleEvent(e core.Event) error {
	switch e.Type {
	case core.EventPipelineStarted:
		fmt.Fprintf(h.out, "● Pipeline %s started\n", e.PipelineID)

	case core.EventPipelineFinished:
		icon := statusIconForCore(e.Status)
		msg := fmt.Sprintf("%s Pipeline %s finished: %s", icon, e.PipelineID, e.Status)
		if e.Duration > 0 {
			msg += fmt.Sprintf(" (%s)", e.Duration)
		}
		if e.Error != "" {
			msg += fmt.Sprintf(" — %s", e.Error)
		}
		fmt.Fprintln(h.out, msg)

	case core.EventJobStarted:
		fmt.Fprintf(h.out, "  ● Job %s started\n", e.JobName)

	case core.EventJobFinished:
		icon := statusIconForCore(e.Status)
		msg := fmt.Sprintf("  %s Job %s: %s", icon, e.JobName, e.Status)
		if e.Duration > 0 {
			msg += fmt.Sprintf(" (%s)", e.Duration)
		}
		if e.Error != "" {
			msg += fmt.Sprintf(" — %s", e.Error)
		}
		fmt.Fprintln(h.out, msg)

	case core.EventJobSkipped:
		fmt.Fprintf(h.out, "  ⊘ Job %s skipped\n", e.JobName)

	case core.EventStepStarted:
		if h.verbose {
			fmt.Fprintf(h.out, "    ● Step %s/%s started\n", e.JobName, e.StepName)
		}

	case core.EventStepFinished:
		icon := statusIconForCore(e.Status)
		msg := fmt.Sprintf("    %s Step %s/%s: %s", icon, e.JobName, e.StepName, e.Status)
		if e.Duration > 0 {
			msg += fmt.Sprintf(" (%s)", e.Duration)
		}
		if e.ExitCode != 0 {
			msg += fmt.Sprintf(" (exit code %d)", e.ExitCode)
		}
		if e.Error != "" {
			msg += fmt.Sprintf(" — %s", e.Error)
		}
		fmt.Fprintln(h.out, msg)

	case core.EventStepLog:
		w := h.out
		if e.IsStderr {
			w = h.errOut
		}
		if h.verbose {
			fmt.Fprintf(w, "[%s/%s] %s", e.JobName, e.StepName, string(e.LogData))
		} else {
			_, _ = w.Write(e.LogData)
		}
	}

	return nil
}

// statusIconForCore returns a terminal icon for a core Status.
func statusIconForCore(s core.Status) string {
	switch s {
	case core.StatusSuccess:
		return "✓"
	case core.StatusFailed:
		return "✗"
	case core.StatusRunning:
		return "●"
	case core.StatusPending:
		return "○"
	case core.StatusSkipped:
		return "⊘"
	case core.StatusCanceled:
		return "⊗"
	default:
		return "?"
	}
}
