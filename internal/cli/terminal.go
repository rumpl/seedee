package cli

import (
	"fmt"
	"io"
	"time"

	"github.com/rumpl/seedee/internal/core"
)

// printPipelineSummary prints a final summary of the pipeline run.
func printPipelineSummary(w io.Writer, result *core.PipelineResult) {
	fmt.Fprintf(w, "\n━━━ Pipeline Summary ━━━\n")
	fmt.Fprintf(w, "Status:   %s\n", result.Status)
	fmt.Fprintf(w, "Duration: %s\n", result.Duration.Round(time.Millisecond))
	fmt.Fprintf(w, "\nJobs:\n")

	for _, jr := range result.Jobs {
		icon := statusIconCore(jr.Status)
		fmt.Fprintf(w, "  %s %s\n", icon, jr.JobName)
	}
	fmt.Fprintf(w, "━━━━━━━━━━━━━━━━━━━━━━━\n")
}

// statusIconCore maps core.Status values to terminal icons.
func statusIconCore(s core.Status) string {
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
