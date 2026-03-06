package cli

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/rumpl/seedee/internal/core"
)

// terminalLogWriter implements core.LogWriter for terminal output.
// It prefixes each log line with [job/step] and supports thread-safe writes.
type terminalLogWriter struct {
	out     io.Writer
	errOut  io.Writer
	verbose bool
	mu      sync.Mutex
}

// WriteLog writes log data to the terminal with [job/step] prefixes.
func (w *terminalLogWriter) WriteLog(jobName, stepName string, data []byte, isStderr bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	prefix := fmt.Sprintf("    [%s/%s] ", jobName, stepName)
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	for _, line := range lines {
		if isStderr {
			fmt.Fprintf(w.errOut, "%s%s\n", prefix, line)
		} else {
			fmt.Fprintf(w.out, "%s%s\n", prefix, line)
		}
	}
	return nil
}

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
