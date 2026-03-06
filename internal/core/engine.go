// Package core contains the pipeline model, DAG scheduler, and execution engine.
package core

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

// LogWriter receives log output from step execution.
type LogWriter interface {
	WriteLog(jobName, stepName string, data []byte, isStderr bool) error
}

// EngineRunner defines the interface the engine uses to execute jobs and steps.
// It mirrors the runner.Runner interface to avoid circular imports.
type EngineRunner interface {
	Setup(ctx context.Context, job *Job) error
	RunStep(ctx context.Context, job *Job, step *Step, stdout, stderr io.Writer) (*StepResult, error)
	Teardown(ctx context.Context, job *Job) error
}

// Engine orchestrates pipeline execution using a Runner and the DAG scheduler.
type Engine struct {
	Runner    EngineRunner
	LogWriter LogWriter
}

// Execute runs the given pipeline to completion, returning a PipelineResult.
func (e *Engine) Execute(ctx context.Context, pipeline *Pipeline) (*PipelineResult, error) {
	pipeline.Status = StatusRunning
	pipeline.StartedAt = time.Now()

	// Schedule the pipeline into execution groups
	groups, err := Schedule(pipeline)
	if err != nil {
		pipeline.Status = StatusFailed
		pipeline.EndedAt = time.Now()
		pipeline.Error = err
		return &PipelineResult{
			PipelineID: pipeline.ID,
			Status:     StatusFailed,
			Duration:   pipeline.EndedAt.Sub(pipeline.StartedAt),
			Error:      err,
		}, err
	}

	// Track which jobs failed so we can skip dependents
	var failedMu sync.Mutex
	failedJobs := make(map[string]bool)

	// Collect job results
	var jobResultsMu sync.Mutex
	jobResults := make(map[string]JobResult)

	// Process each execution group sequentially; jobs within a group run in parallel
	for _, group := range groups {
		// Check for context cancellation before starting a new group
		if ctx.Err() != nil {
			// Mark remaining jobs as canceled
			for _, job := range group.Jobs {
				job.Status = StatusCanceled
				job.EndedAt = time.Now()
				jobResultsMu.Lock()
				jobResults[job.Name] = JobResult{
					JobName: job.Name,
					Status:  StatusCanceled,
				}
				jobResultsMu.Unlock()
			}
			break
		}

		g, gctx := errgroup.WithContext(ctx)

		for _, job := range group.Jobs {
			job := job // capture loop variable

			g.Go(func() error {
				// Check if any dependency failed
				failedMu.Lock()
				shouldSkip := false
				for _, dep := range job.DependsOn {
					if failedJobs[dep] {
						shouldSkip = true
						break
					}
				}
				failedMu.Unlock()

				if shouldSkip {
					job.Status = StatusSkipped
					job.EndedAt = time.Now()
					failedMu.Lock()
					failedJobs[job.Name] = true
					failedMu.Unlock()
					jobResultsMu.Lock()
					jobResults[job.Name] = JobResult{
						JobName: job.Name,
						Status:  StatusSkipped,
					}
					jobResultsMu.Unlock()
					return nil
				}

				result := e.executeJob(gctx, job)

				jobResultsMu.Lock()
				jobResults[job.Name] = result
				jobResultsMu.Unlock()

				if result.Status == StatusFailed || result.Status == StatusCanceled {
					failedMu.Lock()
					failedJobs[job.Name] = true
					failedMu.Unlock()
				}

				// Don't return error from errgroup — we handle failure via failedJobs tracking.
				// Returning an error would cancel sibling jobs in the same group.
				return nil
			})
		}

		// Wait for all jobs in this group to complete
		_ = g.Wait()
	}

	// Determine final pipeline status
	pipeline.EndedAt = time.Now()
	pipelineStatus := StatusSuccess
	var pipelineErr error

	if ctx.Err() != nil {
		pipelineStatus = StatusCanceled
		pipelineErr = ctx.Err()
	} else {
		for _, jr := range jobResults {
			if jr.Status == StatusFailed {
				pipelineStatus = StatusFailed
				pipelineErr = jr.Error
				break
			}
		}
	}

	pipeline.Status = pipelineStatus
	pipeline.Error = pipelineErr

	// Build ordered job results
	var orderedResults []JobResult
	for _, job := range pipeline.Jobs {
		if r, ok := jobResults[job.Name]; ok {
			orderedResults = append(orderedResults, r)
		}
	}

	return &PipelineResult{
		PipelineID: pipeline.ID,
		Status:     pipelineStatus,
		Jobs:       orderedResults,
		Duration:   pipeline.EndedAt.Sub(pipeline.StartedAt),
		Error:      pipelineErr,
	}, nil
}

// executeJob runs a single job: Setup, Steps (sequentially), Teardown.
func (e *Engine) executeJob(ctx context.Context, job *Job) JobResult {
	job.Status = StatusRunning
	job.StartedAt = time.Now()

	var stepResults []StepResult

	// Setup
	if err := e.Runner.Setup(ctx, job); err != nil {
		job.Status = StatusFailed
		job.EndedAt = time.Now()
		job.Error = err
		// Teardown always called
		_ = e.Runner.Teardown(ctx, job)
		return JobResult{
			JobName: job.Name,
			Status:  StatusFailed,
			Steps:   stepResults,
			Error:   fmt.Errorf("setup failed: %w", err),
		}
	}

	// Always call Teardown
	defer func() {
		_ = e.Runner.Teardown(ctx, job)
	}()

	// Run steps sequentially
	jobFailed := false
	for _, step := range job.Steps {
		// Check for context cancellation
		if ctx.Err() != nil {
			step.Status = StatusCanceled
			step.EndedAt = time.Now()
			stepResults = append(stepResults, StepResult{
				ExitCode: -1,
				Error:    ctx.Err(),
			})
			jobFailed = true
			break
		}

		step.Status = StatusRunning
		step.StartedAt = time.Now()

		stdout := &logAdapter{
			writer:   e.LogWriter,
			job:      job.Name,
			step:     step.Name,
			isStderr: false,
		}
		stderr := &logAdapter{
			writer:   e.LogWriter,
			job:      job.Name,
			step:     step.Name,
			isStderr: true,
		}

		result, err := e.Runner.RunStep(ctx, job, step, stdout, stderr)
		step.EndedAt = time.Now()

		if err != nil {
			step.Status = StatusFailed
			step.Error = err
			stepResults = append(stepResults, StepResult{
				ExitCode: -1,
				Error:    err,
			})
			jobFailed = true
			break
		}

		step.ExitCode = result.ExitCode
		if result.ExitCode != 0 {
			step.Status = StatusFailed
			step.Error = fmt.Errorf("exit code %d", result.ExitCode)
			stepResults = append(stepResults, *result)
			jobFailed = true
			break
		}

		step.Status = StatusSuccess
		stepResults = append(stepResults, *result)
	}

	job.EndedAt = time.Now()

	if jobFailed {
		job.Status = StatusFailed
		return JobResult{
			JobName: job.Name,
			Status:  StatusFailed,
			Steps:   stepResults,
			Error:   job.Error,
		}
	}

	job.Status = StatusSuccess
	return JobResult{
		JobName: job.Name,
		Status:  StatusSuccess,
		Steps:   stepResults,
	}
}

// logAdapter implements io.Writer and routes writes to a LogWriter.
type logAdapter struct {
	writer   LogWriter
	job      string
	step     string
	isStderr bool
}

func (l *logAdapter) Write(p []byte) (n int, err error) {
	if l.writer != nil {
		if err := l.writer.WriteLog(l.job, l.step, p, l.isStderr); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

// StdoutLogWriter is a LogWriter that prints logs to stdout with a prefix.
type StdoutLogWriter struct{}

// WriteLog prints log data to stdout with a [job/step] prefix.
func (w *StdoutLogWriter) WriteLog(jobName, stepName string, data []byte, isStderr bool) error {
	prefix := fmt.Sprintf("[%s/%s] ", jobName, stepName)
	fmt.Print(prefix + string(data))
	return nil
}
