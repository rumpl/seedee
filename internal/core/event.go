package core

import "time"

// EventType identifies what kind of event occurred during pipeline execution.
type EventType string

const (
	// EventPipelineStarted indicates the pipeline has started executing.
	EventPipelineStarted EventType = "pipeline_started"
	// EventPipelineFinished indicates the pipeline has finished executing.
	EventPipelineFinished EventType = "pipeline_finished"
	// EventJobStarted indicates a job has started executing.
	EventJobStarted EventType = "job_started"
	// EventJobFinished indicates a job has finished executing.
	EventJobFinished EventType = "job_finished"
	// EventJobSkipped indicates a job was skipped due to dependency failure.
	EventJobSkipped EventType = "job_skipped"
	// EventStepStarted indicates a step has started executing.
	EventStepStarted EventType = "step_started"
	// EventStepFinished indicates a step has finished executing.
	EventStepFinished EventType = "step_finished"
	// EventStepLog indicates log output from a step.
	EventStepLog EventType = "step_log"
)

// Event represents a pipeline execution event streamed to the client.
type Event struct {
	Type       EventType
	Timestamp  time.Time
	PipelineID string
	JobName    string
	StepName   string
	LogData    []byte
	IsStderr   bool
	Status     Status
	ExitCode   int
	Error      string
	Duration   time.Duration
}
