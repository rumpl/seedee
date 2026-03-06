package server

import (
	"time"

	seedeev1 "github.com/rumpl/seedee/gen/seedee/v1"
	"github.com/rumpl/seedee/internal/core"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// EventToProto converts a core.Event to a protobuf RunPipelineEvent.
func EventToProto(e *core.Event) *seedeev1.RunPipelineEvent {
	pe := &seedeev1.RunPipelineEvent{
		PipelineId: e.PipelineID,
		Type:       EventTypeToProto(e.Type),
		JobName:    e.JobName,
		StepName:   e.StepName,
		LogData:    e.LogData,
		IsStderr:   e.IsStderr,
		Status:     StatusToProto(e.Status),
		ExitCode:   int32(e.ExitCode),
		Error:      e.Error,
	}

	if !e.Timestamp.IsZero() {
		pe.Timestamp = timestamppb.New(e.Timestamp)
	}
	if e.Duration > 0 {
		pe.Duration = durationpb.New(e.Duration)
	}

	return pe
}

// EventTypeToProto converts a core EventType to protobuf EventType.
func EventTypeToProto(t core.EventType) seedeev1.EventType {
	switch t {
	case core.EventPipelineStarted:
		return seedeev1.EventType_EVENT_TYPE_PIPELINE_STARTED
	case core.EventPipelineFinished:
		return seedeev1.EventType_EVENT_TYPE_PIPELINE_FINISHED
	case core.EventJobStarted:
		return seedeev1.EventType_EVENT_TYPE_JOB_STARTED
	case core.EventJobFinished:
		return seedeev1.EventType_EVENT_TYPE_JOB_FINISHED
	case core.EventJobSkipped:
		return seedeev1.EventType_EVENT_TYPE_JOB_SKIPPED
	case core.EventStepStarted:
		return seedeev1.EventType_EVENT_TYPE_STEP_STARTED
	case core.EventStepFinished:
		return seedeev1.EventType_EVENT_TYPE_STEP_FINISHED
	case core.EventStepLog:
		return seedeev1.EventType_EVENT_TYPE_STEP_LOG
	default:
		return seedeev1.EventType_EVENT_TYPE_UNSPECIFIED
	}
}

// PipelineDefFromProto converts a protobuf PipelineDefinition to a core PipelineConfig.
func PipelineDefFromProto(pb *seedeev1.PipelineDefinition) *core.PipelineConfig {
	if pb == nil {
		return nil
	}
	jobs := make(map[string]core.JobDef, len(pb.GetJobs()))
	for name, jd := range pb.GetJobs() {
		steps := make([]core.StepDef, 0, len(jd.GetSteps()))
		for _, sd := range jd.GetSteps() {
			steps = append(steps, core.StepDef{
				Name: sd.GetName(),
				Run:  sd.GetRun(),
				Env:  sd.GetEnv(),
			})
		}
		jobs[name] = core.JobDef{
			Image:     jd.GetImage(),
			DependsOn: jd.GetDependsOn(),
			Env:       jd.GetEnv(),
			Steps:     steps,
		}
	}
	return &core.PipelineConfig{
		Pipeline: core.PipelineDef{
			Name: pb.GetName(),
			Env:  pb.GetEnv(),
			Jobs: jobs,
		},
	}
}

// StatusToProto converts a core Status to protobuf Status.
func StatusToProto(s core.Status) seedeev1.Status {
	switch s {
	case core.StatusPending:
		return seedeev1.Status_STATUS_PENDING
	case core.StatusRunning:
		return seedeev1.Status_STATUS_RUNNING
	case core.StatusSuccess:
		return seedeev1.Status_STATUS_SUCCESS
	case core.StatusFailed:
		return seedeev1.Status_STATUS_FAILED
	case core.StatusSkipped:
		return seedeev1.Status_STATUS_SKIPPED
	case core.StatusCanceled:
		return seedeev1.Status_STATUS_CANCELED
	default:
		return seedeev1.Status_STATUS_UNSPECIFIED
	}
}

// StatusFromProto converts a protobuf Status to core Status.
func StatusFromProto(s seedeev1.Status) core.Status {
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
		return core.StatusPending
	}
}

// PipelineStatusToProto converts a runtime Pipeline to a GetPipelineStatusResponse.
func PipelineStatusToProto(p *core.Pipeline) *seedeev1.GetPipelineStatusResponse {
	if p == nil {
		return nil
	}

	jobs := make([]*seedeev1.JobStatus, 0, len(p.Jobs))
	for _, j := range p.Jobs {
		jobs = append(jobs, jobStatusToProto(j))
	}

	resp := &seedeev1.GetPipelineStatusResponse{
		PipelineId:   p.ID,
		PipelineName: p.Name,
		Status:       StatusToProto(p.Status),
		Jobs:         jobs,
	}

	if !p.StartedAt.IsZero() {
		resp.StartedAt = timestamppb.New(p.StartedAt)
	}

	if !p.EndedAt.IsZero() && !p.StartedAt.IsZero() {
		resp.Duration = durationpb.New(p.EndedAt.Sub(p.StartedAt))
	} else if !p.StartedAt.IsZero() {
		resp.Duration = durationpb.New(time.Since(p.StartedAt))
	}

	return resp
}

// jobStatusToProto converts a runtime Job to a protobuf JobStatus.
func jobStatusToProto(j *core.Job) *seedeev1.JobStatus {
	if j == nil {
		return nil
	}

	steps := make([]*seedeev1.StepStatus, 0, len(j.Steps))
	for _, s := range j.Steps {
		steps = append(steps, stepStatusToProto(s))
	}

	js := &seedeev1.JobStatus{
		Name:   j.Name,
		Status: StatusToProto(j.Status),
		Steps:  steps,
	}

	if !j.EndedAt.IsZero() && !j.StartedAt.IsZero() {
		js.Duration = durationpb.New(j.EndedAt.Sub(j.StartedAt))
	} else if !j.StartedAt.IsZero() {
		js.Duration = durationpb.New(time.Since(j.StartedAt))
	}

	return js
}

// stepStatusToProto converts a runtime Step to a protobuf StepStatus.
func stepStatusToProto(s *core.Step) *seedeev1.StepStatus {
	if s == nil {
		return nil
	}

	ss := &seedeev1.StepStatus{
		Name:     s.Name,
		Status:   StatusToProto(s.Status),
		ExitCode: int32(s.ExitCode),
	}

	if !s.EndedAt.IsZero() && !s.StartedAt.IsZero() {
		ss.Duration = durationpb.New(s.EndedAt.Sub(s.StartedAt))
	} else if !s.StartedAt.IsZero() {
		ss.Duration = durationpb.New(time.Since(s.StartedAt))
	}

	return ss
}

// PipelineSummaryToProto converts a runtime Pipeline to a protobuf PipelineSummary.
func PipelineSummaryToProto(p *core.Pipeline) *seedeev1.PipelineSummary {
	if p == nil {
		return nil
	}

	s := &seedeev1.PipelineSummary{
		PipelineId: p.ID,
		Name:       p.Name,
		Status:     StatusToProto(p.Status),
	}

	if !p.StartedAt.IsZero() {
		s.StartedAt = timestamppb.New(p.StartedAt)
	}

	if !p.EndedAt.IsZero() && !p.StartedAt.IsZero() {
		s.Duration = durationpb.New(p.EndedAt.Sub(p.StartedAt))
	} else if !p.StartedAt.IsZero() {
		s.Duration = durationpb.New(time.Since(p.StartedAt))
	}

	return s
}
