// Package runner defines the runner interface and its implementations.
package runner

import (
	"context"
	"io"

	"github.com/rumpl/seedee/internal/core"
)

// Runner executes a single step and returns the result.
type Runner interface {
	Setup(ctx context.Context, job *core.Job) error
	RunStep(ctx context.Context, job *core.Job, step *core.Step, stdout, stderr io.Writer) (*core.StepResult, error)
	Teardown(ctx context.Context, job *core.Job) error
}
