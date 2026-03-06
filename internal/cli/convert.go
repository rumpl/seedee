package cli

import (
	"time"

	seedeev1 "github.com/rumpl/seedee/gen/seedee/v1"
	"github.com/rumpl/seedee/internal/core"
)

// pipelineToProtoRequest converts a core Pipeline to a RunPipelineRequest.
func pipelineToProtoRequest(p *core.Pipeline) *seedeev1.RunPipelineRequest {
	jobs := make(map[string]*seedeev1.JobDefinition, len(p.Jobs))
	for _, job := range p.Jobs {
		steps := make([]*seedeev1.StepDefinition, len(job.Steps))
		for i, step := range job.Steps {
			steps[i] = &seedeev1.StepDefinition{
				Name: step.Name,
				Run:  step.Command,
				Env:  step.Env,
			}
		}
		jobs[job.Name] = &seedeev1.JobDefinition{
			Image:     job.Image,
			DependsOn: job.DependsOn,
			Env:       job.Env,
			Steps:     steps,
		}
	}

	return &seedeev1.RunPipelineRequest{
		Pipeline: &seedeev1.PipelineDefinition{
			Name: p.Name,
			Env:  p.Env,
			Jobs: jobs,
		},
	}
}

// protoEventToCore converts a protobuf RunPipelineEvent to a core Event.
func protoEventToCore(pe *seedeev1.RunPipelineEvent) *core.Event {
	var ts time.Time
	if pe.GetTimestamp() != nil {
		ts = pe.GetTimestamp().AsTime()
	}

	var dur time.Duration
	if pe.GetDuration() != nil {
		dur = pe.GetDuration().AsDuration()
	}

	return &core.Event{
		Type:       protoEventTypeToCore(pe.GetType()),
		Timestamp:  ts,
		PipelineID: pe.GetPipelineId(),
		JobName:    pe.GetJobName(),
		StepName:   pe.GetStepName(),
		LogData:    pe.GetLogData(),
		IsStderr:   pe.GetIsStderr(),
		Status:     protoStatusToCore(pe.GetStatus()),
		ExitCode:   int(pe.GetExitCode()),
		Error:      pe.GetError(),
		Duration:   dur,
	}
}

// protoEventTypeToCore converts a protobuf EventType to a core EventType.
func protoEventTypeToCore(t seedeev1.EventType) core.EventType {
	switch t {
	case seedeev1.EventType_EVENT_TYPE_PIPELINE_STARTED:
		return core.EventPipelineStarted
	case seedeev1.EventType_EVENT_TYPE_PIPELINE_FINISHED:
		return core.EventPipelineFinished
	case seedeev1.EventType_EVENT_TYPE_JOB_STARTED:
		return core.EventJobStarted
	case seedeev1.EventType_EVENT_TYPE_JOB_FINISHED:
		return core.EventJobFinished
	case seedeev1.EventType_EVENT_TYPE_JOB_SKIPPED:
		return core.EventJobSkipped
	case seedeev1.EventType_EVENT_TYPE_STEP_STARTED:
		return core.EventStepStarted
	case seedeev1.EventType_EVENT_TYPE_STEP_FINISHED:
		return core.EventStepFinished
	case seedeev1.EventType_EVENT_TYPE_STEP_LOG:
		return core.EventStepLog
	default:
		return ""
	}
}

// protoStatusToCore converts a protobuf Status to a core Status.
func protoStatusToCore(s seedeev1.Status) core.Status {
	switch s {
	case seedeev1.Status_STATUS_PENDING:
		return core.StatusPending
	case seedeev1.Status_STATUS_RUNNING:
		return core.StatusRunning
	case seedeev1.Status_STATUS_SUCCESS:
		return core.StatusSuccess
	case seedeev1.Status_STATUS_FAILED:
		return core.StatusFailed
	case seedeev1.Status_STATUS_SKIPPED:
		return core.StatusSkipped
	case seedeev1.Status_STATUS_CANCELED:
		return core.StatusCanceled
	default:
		return ""
	}
}

// formatProtoStatus returns a human-readable string for a proto Status.
func formatProtoStatus(s seedeev1.Status) string {
	switch s {
	case seedeev1.Status_STATUS_SUCCESS:
		return "success"
	case seedeev1.Status_STATUS_FAILED:
		return "failed"
	case seedeev1.Status_STATUS_RUNNING:
		return "running"
	case seedeev1.Status_STATUS_PENDING:
		return "pending"
	case seedeev1.Status_STATUS_SKIPPED:
		return "skipped"
	case seedeev1.Status_STATUS_CANCELED:
		return "canceled"
	default:
		return "unknown"
	}
}
