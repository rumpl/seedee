package cli

import (
	"bytes"
	"testing"
	"time"

	seedeev1 "github.com/rumpl/seedee/gen/seedee/v1"
	"github.com/rumpl/seedee/internal/core"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestPipelineToProtoRequest(t *testing.T) {
	pipeline := &core.Pipeline{
		Name: "test",
		Env:  map[string]string{"GLOBAL": "val"},
		Jobs: []*core.Job{
			{
				Name:      "build",
				Image:     "golang:1.22",
				DependsOn: []string{"lint"},
				Env:       map[string]string{"JOB_VAR": "jval"},
				Steps: []*core.Step{
					{Name: "compile", Command: "go build ./...", Env: map[string]string{"STEP_VAR": "sval"}},
					{Name: "test", Command: "go test ./..."},
				},
			},
			{
				Name:  "lint",
				Image: "golangci/golangci-lint:v1.61",
				Steps: []*core.Step{
					{Name: "run-lint", Command: "golangci-lint run"},
				},
			},
		},
	}

	req := pipelineToProtoRequest(pipeline)
	if req.Pipeline == nil {
		t.Fatal("expected non-nil pipeline")
	}
	if req.Pipeline.Name != "test" {
		t.Errorf("pipeline name = %q, want %q", req.Pipeline.Name, "test")
	}
	if req.Pipeline.Env["GLOBAL"] != "val" {
		t.Errorf("pipeline env[GLOBAL] = %q, want %q", req.Pipeline.Env["GLOBAL"], "val")
	}
	if len(req.Pipeline.Jobs) != 2 {
		t.Fatalf("jobs count = %d, want 2", len(req.Pipeline.Jobs))
	}

	build, ok := req.Pipeline.Jobs["build"]
	if !ok {
		t.Fatal("missing 'build' job")
	}
	if build.Image != "golang:1.22" {
		t.Errorf("build.Image = %q, want %q", build.Image, "golang:1.22")
	}
	if len(build.DependsOn) != 1 || build.DependsOn[0] != "lint" {
		t.Errorf("build.DependsOn = %v, want [lint]", build.DependsOn)
	}
	if build.Env["JOB_VAR"] != "jval" {
		t.Errorf("build.Env[JOB_VAR] = %q, want %q", build.Env["JOB_VAR"], "jval")
	}
	if len(build.Steps) != 2 {
		t.Fatalf("build steps count = %d, want 2", len(build.Steps))
	}
	if build.Steps[0].Name != "compile" {
		t.Errorf("step[0].Name = %q, want %q", build.Steps[0].Name, "compile")
	}
	if build.Steps[0].Run != "go build ./..." {
		t.Errorf("step[0].Run = %q, want %q", build.Steps[0].Run, "go build ./...")
	}
	if build.Steps[0].Env["STEP_VAR"] != "sval" {
		t.Errorf("step[0].Env[STEP_VAR] = %q, want %q", build.Steps[0].Env["STEP_VAR"], "sval")
	}

	lint, ok := req.Pipeline.Jobs["lint"]
	if !ok {
		t.Fatal("missing 'lint' job")
	}
	if lint.Image != "golangci/golangci-lint:v1.61" {
		t.Errorf("lint.Image = %q, want %q", lint.Image, "golangci/golangci-lint:v1.61")
	}
	if len(lint.Steps) != 1 {
		t.Fatalf("lint steps count = %d, want 1", len(lint.Steps))
	}
}

func TestPipelineToProtoRequest_Empty(t *testing.T) {
	pipeline := &core.Pipeline{
		Name: "empty",
	}
	req := pipelineToProtoRequest(pipeline)
	if req.Pipeline.Name != "empty" {
		t.Errorf("name = %q, want %q", req.Pipeline.Name, "empty")
	}
	if len(req.Pipeline.Jobs) != 0 {
		t.Errorf("jobs count = %d, want 0", len(req.Pipeline.Jobs))
	}
}

func TestProtoEventToCore(t *testing.T) {
	ts := timestamppb.New(time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC))
	dur := durationpb.New(5 * time.Second)

	pe := &seedeev1.RunPipelineEvent{
		PipelineId: "pipe-123",
		Type:       seedeev1.EventType_EVENT_TYPE_STEP_LOG,
		Timestamp:  ts,
		JobName:    "build",
		StepName:   "compile",
		LogData:    []byte("building...\n"),
		IsStderr:   false,
		Status:     seedeev1.Status_STATUS_RUNNING,
		ExitCode:   0,
		Error:      "",
		Duration:   dur,
	}

	ce := protoEventToCore(pe)
	if ce.Type != core.EventStepLog {
		t.Errorf("Type = %q, want %q", ce.Type, core.EventStepLog)
	}
	if ce.PipelineID != "pipe-123" {
		t.Errorf("PipelineID = %q, want %q", ce.PipelineID, "pipe-123")
	}
	if ce.JobName != "build" {
		t.Errorf("JobName = %q, want %q", ce.JobName, "build")
	}
	if ce.StepName != "compile" {
		t.Errorf("StepName = %q, want %q", ce.StepName, "compile")
	}
	if !bytes.Equal(ce.LogData, []byte("building...\n")) {
		t.Errorf("LogData = %q, want %q", ce.LogData, "building...\n")
	}
	if ce.IsStderr {
		t.Error("IsStderr = true, want false")
	}
	if ce.Status != core.StatusRunning {
		t.Errorf("Status = %q, want %q", ce.Status, core.StatusRunning)
	}
	if ce.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", ce.ExitCode)
	}
	if ce.Error != "" {
		t.Errorf("Error = %q, want empty", ce.Error)
	}
	if ce.Duration != 5*time.Second {
		t.Errorf("Duration = %v, want 5s", ce.Duration)
	}
	if !ce.Timestamp.Equal(ts.AsTime()) {
		t.Errorf("Timestamp = %v, want %v", ce.Timestamp, ts.AsTime())
	}
}

func TestProtoEventToCore_NilTimestampAndDuration(t *testing.T) {
	pe := &seedeev1.RunPipelineEvent{
		PipelineId: "pipe-456",
		Type:       seedeev1.EventType_EVENT_TYPE_PIPELINE_STARTED,
	}
	ce := protoEventToCore(pe)
	if !ce.Timestamp.IsZero() {
		t.Errorf("Timestamp = %v, want zero", ce.Timestamp)
	}
	if ce.Duration != 0 {
		t.Errorf("Duration = %v, want 0", ce.Duration)
	}
}

func TestProtoEventTypeToCore_AllTypes(t *testing.T) {
	tests := []struct {
		proto seedeev1.EventType
		core  core.EventType
	}{
		{seedeev1.EventType_EVENT_TYPE_PIPELINE_STARTED, core.EventPipelineStarted},
		{seedeev1.EventType_EVENT_TYPE_PIPELINE_FINISHED, core.EventPipelineFinished},
		{seedeev1.EventType_EVENT_TYPE_JOB_STARTED, core.EventJobStarted},
		{seedeev1.EventType_EVENT_TYPE_JOB_FINISHED, core.EventJobFinished},
		{seedeev1.EventType_EVENT_TYPE_JOB_SKIPPED, core.EventJobSkipped},
		{seedeev1.EventType_EVENT_TYPE_STEP_STARTED, core.EventStepStarted},
		{seedeev1.EventType_EVENT_TYPE_STEP_FINISHED, core.EventStepFinished},
		{seedeev1.EventType_EVENT_TYPE_STEP_LOG, core.EventStepLog},
	}
	for _, tt := range tests {
		t.Run(tt.proto.String(), func(t *testing.T) {
			got := protoEventTypeToCore(tt.proto)
			if got != tt.core {
				t.Errorf("protoEventTypeToCore(%v) = %q, want %q", tt.proto, got, tt.core)
			}
		})
	}
}

func TestProtoEventTypeToCore_Unspecified(t *testing.T) {
	got := protoEventTypeToCore(seedeev1.EventType_EVENT_TYPE_UNSPECIFIED)
	if got != "" {
		t.Errorf("protoEventTypeToCore(UNSPECIFIED) = %q, want empty", got)
	}
}

func TestProtoStatusToCore_AllStatuses(t *testing.T) {
	tests := []struct {
		proto seedeev1.Status
		core  core.Status
	}{
		{seedeev1.Status_STATUS_PENDING, core.StatusPending},
		{seedeev1.Status_STATUS_RUNNING, core.StatusRunning},
		{seedeev1.Status_STATUS_SUCCESS, core.StatusSuccess},
		{seedeev1.Status_STATUS_FAILED, core.StatusFailed},
		{seedeev1.Status_STATUS_SKIPPED, core.StatusSkipped},
		{seedeev1.Status_STATUS_CANCELED, core.StatusCanceled},
	}
	for _, tt := range tests {
		t.Run(tt.proto.String(), func(t *testing.T) {
			got := protoStatusToCore(tt.proto)
			if got != tt.core {
				t.Errorf("protoStatusToCore(%v) = %q, want %q", tt.proto, got, tt.core)
			}
		})
	}
}

func TestProtoStatusToCore_Unspecified(t *testing.T) {
	got := protoStatusToCore(seedeev1.Status_STATUS_UNSPECIFIED)
	if got != "" {
		t.Errorf("protoStatusToCore(UNSPECIFIED) = %q, want empty", got)
	}
}

func TestFormatProtoStatus(t *testing.T) {
	tests := []struct {
		status seedeev1.Status
		want   string
	}{
		{seedeev1.Status_STATUS_SUCCESS, "success"},
		{seedeev1.Status_STATUS_FAILED, "failed"},
		{seedeev1.Status_STATUS_RUNNING, "running"},
		{seedeev1.Status_STATUS_PENDING, "pending"},
		{seedeev1.Status_STATUS_SKIPPED, "skipped"},
		{seedeev1.Status_STATUS_CANCELED, "canceled"},
		{seedeev1.Status_STATUS_UNSPECIFIED, "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.status.String(), func(t *testing.T) {
			got := formatProtoStatus(tt.status)
			if got != tt.want {
				t.Errorf("formatProtoStatus(%v) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}
