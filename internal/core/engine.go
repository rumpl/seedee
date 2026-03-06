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

// EngineRunner defines the interface the engine uses to execute jobs and steps.
// It mirrors the runner.Runner interface to avoid circular imports.
type EngineRunner interface {
	Setup(ctx context.Context, job *Job) error
	RunStep(ctx context.Context, job *Job, step *Step, stdout, stderr io.Writer) (*StepResult, error)
	Teardown(ctx context.Context, job *Job) error
}

// Engine orchestrates pipeline execution using a Runner and the DAG scheduler.
type Engine struct {
	Runner       EngineRunner
	EventHandler EventHandler
}

// emit sends an event to the EventHandler if one is set.
func (e *Engine) emit(event Event) {
	if e.EventHandler != nil {
		_ = e.EventHandler.HandleEvent(event)
	}
}

// Execute runs the given pipeline to completion, returning a PipelineResult.
func (e *Engine) Execute(ctx context.Context, pipeline *Pipeline) (*PipelineResult, error) {
	pipeline.Status = StatusRunning
	pipeline.StartedAt = time.Now()

	e.emit(Event{
		Type:         EventPipelineStarted,
		Timestamp:    pipeline.StartedAt,
		PipelineID:   pipeline.ID,
		PipelineName: pipeline.Name,
	})

	// Schedule the pipeline into execution groups
	groups, err := Schedule(pipeline)
	if err != nil {
		pipeline.Status = StatusFailed
		pipeline.EndedAt = time.Now()
		pipeline.Error = err

		e.emit(Event{
			Type:         EventPipelineFinished,
			Timestamp:    pipeline.EndedAt,
			PipelineID:   pipeline.ID,
			PipelineName: pipeline.Name,
			Status:       StatusFailed,
			Error:        err.Error(),
			Duration:     pipeline.EndedAt.Sub(pipeline.StartedAt),
		})

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
					JobName:  job.Name,
					Status:   StatusCanceled,
					Duration: 0,
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

					e.emit(Event{
						Type:         EventJobSkipped,
						Timestamp:    job.EndedAt,
						PipelineID:   pipeline.ID,
						PipelineName: pipeline.Name,
						JobName:      job.Name,
						Status:       StatusSkipped,
					})

					failedMu.Lock()
					failedJobs[job.Name] = true
					failedMu.Unlock()
					jobResultsMu.Lock()
					jobResults[job.Name] = JobResult{
						JobName:  job.Name,
						Status:   StatusSkipped,
						Duration: 0,
					}
					jobResultsMu.Unlock()
					return nil
				}

				result := e.executeJob(gctx, pipeline, job)

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

	pipelineDuration := pipeline.EndedAt.Sub(pipeline.StartedAt)

	errStr := ""
	if pipelineErr != nil {
		errStr = pipelineErr.Error()
	}
	e.emit(Event{
		Type:         EventPipelineFinished,
		Timestamp:    pipeline.EndedAt,
		PipelineID:   pipeline.ID,
		PipelineName: pipeline.Name,
		Status:       pipelineStatus,
		Error:        errStr,
		Duration:     pipelineDuration,
	})

	return &PipelineResult{
		PipelineID: pipeline.ID,
		Status:     pipelineStatus,
		Jobs:       orderedResults,
		Duration:   pipelineDuration,
		Error:      pipelineErr,
	}, nil
}

// executeJob runs a single job: Setup, Steps (sequentially), Teardown.
func (e *Engine) executeJob(ctx context.Context, pipeline *Pipeline, job *Job) JobResult {
	job.Status = StatusRunning
	job.StartedAt = time.Now()

	e.emit(Event{
		Type:         EventJobStarted,
		Timestamp:    job.StartedAt,
		PipelineID:   pipeline.ID,
		PipelineName: pipeline.Name,
		JobName:      job.Name,
	})

	var stepResults []StepResult

	// Setup
	if err := e.Runner.Setup(ctx, job); err != nil {
		job.Status = StatusFailed
		job.EndedAt = time.Now()
		job.Error = err
		// Teardown always called
		_ = e.Runner.Teardown(ctx, job)

		jobDuration := job.EndedAt.Sub(job.StartedAt)
		e.emit(Event{
			Type:         EventJobFinished,
			Timestamp:    job.EndedAt,
			PipelineID:   pipeline.ID,
			PipelineName: pipeline.Name,
			JobName:      job.Name,
			Status:       StatusFailed,
			Error:        fmt.Sprintf("setup failed: %v", err),
			Duration:     jobDuration,
		})

		return JobResult{
			JobName:  job.Name,
			Status:   StatusFailed,
			Steps:    stepResults,
			Duration: jobDuration,
			Error:    fmt.Errorf("setup failed: %w", err),
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

		e.emit(Event{
			Type:         EventStepStarted,
			Timestamp:    step.StartedAt,
			PipelineID:   pipeline.ID,
			PipelineName: pipeline.Name,
			JobName:      job.Name,
			StepName:     step.Name,
		})

		stdout := &eventLogAdapter{
			handler:      e.EventHandler,
			pipelineID:   pipeline.ID,
			pipelineName: pipeline.Name,
			job:          job.Name,
			step:         step.Name,
			isStderr:     false,
		}
		stderr := &eventLogAdapter{
			handler:      e.EventHandler,
			pipelineID:   pipeline.ID,
			pipelineName: pipeline.Name,
			job:          job.Name,
			step:         step.Name,
			isStderr:     true,
		}

		result, err := e.Runner.RunStep(ctx, job, step, stdout, stderr)
		step.EndedAt = time.Now()
		stepDuration := step.EndedAt.Sub(step.StartedAt)

		if err != nil {
			step.Status = StatusFailed
			step.Error = err
			stepResults = append(stepResults, StepResult{
				ExitCode: -1,
				Error:    err,
			})

			e.emit(Event{
				Type:         EventStepFinished,
				Timestamp:    step.EndedAt,
				PipelineID:   pipeline.ID,
				PipelineName: pipeline.Name,
				JobName:      job.Name,
				StepName:     step.Name,
				Status:       StatusFailed,
				ExitCode:     -1,
				Error:        err.Error(),
				Duration:     stepDuration,
			})

			jobFailed = true
			break
		}

		step.ExitCode = result.ExitCode
		if result.ExitCode != 0 {
			step.Status = StatusFailed
			step.Error = fmt.Errorf("exit code %d", result.ExitCode)
			stepResults = append(stepResults, *result)

			e.emit(Event{
				Type:         EventStepFinished,
				Timestamp:    step.EndedAt,
				PipelineID:   pipeline.ID,
				PipelineName: pipeline.Name,
				JobName:      job.Name,
				StepName:     step.Name,
				Status:       StatusFailed,
				ExitCode:     result.ExitCode,
				Error:        fmt.Sprintf("exit code %d", result.ExitCode),
				Duration:     stepDuration,
			})

			jobFailed = true
			break
		}

		step.Status = StatusSuccess
		stepResults = append(stepResults, *result)

		e.emit(Event{
			Type:         EventStepFinished,
			Timestamp:    step.EndedAt,
			PipelineID:   pipeline.ID,
			PipelineName: pipeline.Name,
			JobName:      job.Name,
			StepName:     step.Name,
			Status:       StatusSuccess,
			ExitCode:     0,
			Duration:     stepDuration,
		})
	}

	job.EndedAt = time.Now()
	jobDuration := job.EndedAt.Sub(job.StartedAt)

	if jobFailed {
		job.Status = StatusFailed

		errStr := ""
		if job.Error != nil {
			errStr = job.Error.Error()
		}
		e.emit(Event{
			Type:         EventJobFinished,
			Timestamp:    job.EndedAt,
			PipelineID:   pipeline.ID,
			PipelineName: pipeline.Name,
			JobName:      job.Name,
			Status:       StatusFailed,
			Error:        errStr,
			Duration:     jobDuration,
		})

		return JobResult{
			JobName:  job.Name,
			Status:   StatusFailed,
			Steps:    stepResults,
			Duration: jobDuration,
			Error:    job.Error,
		}
	}

	job.Status = StatusSuccess

	e.emit(Event{
		Type:         EventJobFinished,
		Timestamp:    job.EndedAt,
		PipelineID:   pipeline.ID,
		PipelineName: pipeline.Name,
		JobName:      job.Name,
		Status:       StatusSuccess,
		Duration:     jobDuration,
	})

	return JobResult{
		JobName:  job.Name,
		Status:   StatusSuccess,
		Steps:    stepResults,
		Duration: jobDuration,
	}
}

// eventLogAdapter implements io.Writer and routes writes to an EventHandler as EventStepLog events.
type eventLogAdapter struct {
	handler      EventHandler
	pipelineID   string
	pipelineName string
	job          string
	step         string
	isStderr     bool
}

func (l *eventLogAdapter) Write(p []byte) (n int, err error) {
	if l.handler != nil {
		data := make([]byte, len(p))
		copy(data, p)
		if err := l.handler.HandleEvent(Event{
			Type:         EventStepLog,
			Timestamp:    time.Now(),
			PipelineID:   l.pipelineID,
			PipelineName: l.pipelineName,
			JobName:      l.job,
			StepName:     l.step,
			LogData:      data,
			IsStderr:     l.isStderr,
		}); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}
