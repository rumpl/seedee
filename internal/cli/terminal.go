package cli

import (
	"github.com/rumpl/seedee/internal/core"
)

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
