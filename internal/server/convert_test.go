package server

import (
	"testing"
	"time"

	seedeev1 "github.com/rumpl/seedee/gen/seedee/v1"
	"github.com/rumpl/seedee/internal/core"
)

func TestPipelineDefFromProto(t *testing.T) {
	pb := &seedeev1.PipelineDefinition{
		Name: "my-pipeline",
		Env:  map[string]string{"GLOBAL": "val"},
		Jobs: map[string]*seedeev1.JobDefinition{
			"build": {
				Image:     "golang:1.24",
				DependsOn: []string{"lint"},
				Env:       map[string]string{"JOB_VAR": "jval"},
				Steps: []*seedeev1.StepDefinition{
					{
						Name: "compile",
						Run:  "go build ./...",
						Env:  map[string]string{"STEP_VAR": "sval"},
					},
					{
						Name: "test",
						Run:  "go test ./...",
					},
				},
			},
			"lint": {
				Image: "golangci/golangci-lint:v1.61",
				Steps: []*seedeev1.StepDefinition{
					{
						Name: "run-lint",
						Run:  "golangci-lint run",
					},
				},
			},
		},
	}

	cfg := PipelineDefFromProto(pb)
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}

	if cfg.Pipeline.Name != "my-pipeline" {
		t.Errorf("name = %q, want %q", cfg.Pipeline.Name, "my-pipeline")
	}
	if cfg.Pipeline.Env["GLOBAL"] != "val" {
		t.Errorf("env[GLOBAL] = %q, want %q", cfg.Pipeline.Env["GLOBAL"], "val")
	}
	if len(cfg.Pipeline.Jobs) != 2 {
		t.Fatalf("jobs count = %d, want 2", len(cfg.Pipeline.Jobs))
	}

	build := cfg.Pipeline.Jobs["build"]
	if build.Image != "golang:1.24" {
		t.Errorf("build.Image = %q, want %q", build.Image, "golang:1.24")
	}
	if len(build.DependsOn) != 1 || build.DependsOn[0] != "lint" {
		t.Errorf("build.DependsOn = %v, want [lint]", build.DependsOn)
	}
	if build.Env["JOB_VAR"] != "jval" {
		t.Errorf("build.Env[JOB_VAR] = %q, want %q", build.Env["JOB_VAR"], "jval")
	}
	if len(build.Steps) != 2 {
		t.Fatalf("build.Steps count = %d, want 2", len(build.Steps))
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

	lint := cfg.Pipeline.Jobs["lint"]
	if lint.Image != "golangci/golangci-lint:v1.61" {
		t.Errorf("lint.Image = %q, want %q", lint.Image, "golangci/golangci-lint:v1.61")
	}
	if len(lint.DependsOn) != 0 {
		t.Errorf("lint.DependsOn = %v, want empty", lint.DependsOn)
	}
	if len(lint.Steps) != 1 {
		t.Fatalf("lint.Steps count = %d, want 1", len(lint.Steps))
	}
}

func TestPipelineDefFromProto_Nil(t *testing.T) {
	if cfg := PipelineDefFromProto(nil); cfg != nil {
		t.Errorf("expected nil for nil input, got %v", cfg)
	}
}

func TestPipelineDefFromProto_Empty(t *testing.T) {
	cfg := PipelineDefFromProto(&seedeev1.PipelineDefinition{})
	if cfg == nil {
		t.Fatal("expected non-nil config for empty proto")
	}
	if cfg.Pipeline.Name != "" {
		t.Errorf("name = %q, want empty", cfg.Pipeline.Name)
	}
	if len(cfg.Pipeline.Jobs) != 0 {
		t.Errorf("jobs count = %d, want 0", len(cfg.Pipeline.Jobs))
	}
}

func TestStatusToProto(t *testing.T) {
	tests := []struct {
		core  core.Status
		proto seedeev1.Status
	}{
		{core.StatusPending, seedeev1.Status_STATUS_PENDING},
		{core.StatusRunning, seedeev1.Status_STATUS_RUNNING},
		{core.StatusSuccess, seedeev1.Status_STATUS_SUCCESS},
		{core.StatusFailed, seedeev1.Status_STATUS_FAILED},
		{core.StatusSkipped, seedeev1.Status_STATUS_SKIPPED},
		{core.StatusCanceled, seedeev1.Status_STATUS_CANCELED},
	}
	for _, tt := range tests {
		t.Run(string(tt.core), func(t *testing.T) {
			got := StatusToProto(tt.core)
			if got != tt.proto {
				t.Errorf("StatusToProto(%q) = %v, want %v", tt.core, got, tt.proto)
			}
		})
	}
}

func TestStatusToProto_Unknown(t *testing.T) {
	got := StatusToProto(core.Status("unknown"))
	if got != seedeev1.Status_STATUS_UNSPECIFIED {
		t.Errorf("StatusToProto(unknown) = %v, want STATUS_UNSPECIFIED", got)
	}
}

func TestStatusFromProto(t *testing.T) {
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
			got := StatusFromProto(tt.proto)
			if got != tt.core {
				t.Errorf("StatusFromProto(%v) = %q, want %q", tt.proto, got, tt.core)
			}
		})
	}
}

func TestStatusFromProto_Unspecified(t *testing.T) {
	got := StatusFromProto(seedeev1.Status_STATUS_UNSPECIFIED)
	if got != core.StatusPending {
		t.Errorf("StatusFromProto(UNSPECIFIED) = %q, want %q", got, core.StatusPending)
	}
}

func TestStatusRoundTrip(t *testing.T) {
	statuses := []core.Status{
		core.StatusPending,
		core.StatusRunning,
		core.StatusSuccess,
		core.StatusFailed,
		core.StatusSkipped,
		core.StatusCanceled,
	}
	for _, s := range statuses {
		t.Run(string(s), func(t *testing.T) {
			got := StatusFromProto(StatusToProto(s))
			if got != s {
				t.Errorf("round-trip %q -> proto -> core = %q", s, got)
			}
		})
	}
}

func TestPipelineStatusToProto(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(30 * time.Second)

	p := &core.Pipeline{
		ID:        "pipe-123",
		Name:      "my-pipeline",
		Status:    core.StatusSuccess,
		StartedAt: start,
		EndedAt:   end,
		Jobs: []*core.Job{
			{
				Name:      "build",
				Status:    core.StatusSuccess,
				StartedAt: start,
				EndedAt:   start.Add(20 * time.Second),
				Steps: []*core.Step{
					{
						Name:      "compile",
						Status:    core.StatusSuccess,
						ExitCode:  0,
						StartedAt: start,
						EndedAt:   start.Add(10 * time.Second),
					},
					{
						Name:      "test",
						Status:    core.StatusSuccess,
						ExitCode:  0,
						StartedAt: start.Add(10 * time.Second),
						EndedAt:   start.Add(20 * time.Second),
					},
				},
			},
			{
				Name:      "lint",
				Status:    core.StatusFailed,
				StartedAt: start,
				EndedAt:   start.Add(5 * time.Second),
				Steps: []*core.Step{
					{
						Name:      "run-lint",
						Status:    core.StatusFailed,
						ExitCode:  1,
						StartedAt: start,
						EndedAt:   start.Add(5 * time.Second),
					},
				},
			},
		},
	}

	resp := PipelineStatusToProto(p)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}

	if resp.PipelineId != "pipe-123" {
		t.Errorf("PipelineId = %q, want %q", resp.PipelineId, "pipe-123")
	}
	if resp.PipelineName != "my-pipeline" {
		t.Errorf("PipelineName = %q, want %q", resp.PipelineName, "my-pipeline")
	}
	if resp.Status != seedeev1.Status_STATUS_SUCCESS {
		t.Errorf("Status = %v, want STATUS_SUCCESS", resp.Status)
	}
	if resp.StartedAt == nil {
		t.Fatal("StartedAt should not be nil")
	}
	if !resp.StartedAt.AsTime().Equal(start) {
		t.Errorf("StartedAt = %v, want %v", resp.StartedAt.AsTime(), start)
	}
	if resp.Duration == nil {
		t.Fatal("Duration should not be nil")
	}
	if resp.Duration.AsDuration() != 30*time.Second {
		t.Errorf("Duration = %v, want 30s", resp.Duration.AsDuration())
	}

	if len(resp.Jobs) != 2 {
		t.Fatalf("Jobs count = %d, want 2", len(resp.Jobs))
	}

	// Check build job
	buildJob := resp.Jobs[0]
	if buildJob.Name != "build" {
		t.Errorf("Jobs[0].Name = %q, want %q", buildJob.Name, "build")
	}
	if buildJob.Status != seedeev1.Status_STATUS_SUCCESS {
		t.Errorf("Jobs[0].Status = %v, want STATUS_SUCCESS", buildJob.Status)
	}
	if buildJob.Duration.AsDuration() != 20*time.Second {
		t.Errorf("Jobs[0].Duration = %v, want 20s", buildJob.Duration.AsDuration())
	}
	if len(buildJob.Steps) != 2 {
		t.Fatalf("Jobs[0].Steps count = %d, want 2", len(buildJob.Steps))
	}
	if buildJob.Steps[0].Name != "compile" {
		t.Errorf("Jobs[0].Steps[0].Name = %q, want %q", buildJob.Steps[0].Name, "compile")
	}
	if buildJob.Steps[0].ExitCode != 0 {
		t.Errorf("Jobs[0].Steps[0].ExitCode = %d, want 0", buildJob.Steps[0].ExitCode)
	}
	if buildJob.Steps[0].Duration.AsDuration() != 10*time.Second {
		t.Errorf("Jobs[0].Steps[0].Duration = %v, want 10s", buildJob.Steps[0].Duration.AsDuration())
	}

	// Check lint job
	lintJob := resp.Jobs[1]
	if lintJob.Name != "lint" {
		t.Errorf("Jobs[1].Name = %q, want %q", lintJob.Name, "lint")
	}
	if lintJob.Status != seedeev1.Status_STATUS_FAILED {
		t.Errorf("Jobs[1].Status = %v, want STATUS_FAILED", lintJob.Status)
	}
	if len(lintJob.Steps) != 1 {
		t.Fatalf("Jobs[1].Steps count = %d, want 1", len(lintJob.Steps))
	}
	if lintJob.Steps[0].ExitCode != 1 {
		t.Errorf("Jobs[1].Steps[0].ExitCode = %d, want 1", lintJob.Steps[0].ExitCode)
	}
}

func TestPipelineStatusToProto_Nil(t *testing.T) {
	if resp := PipelineStatusToProto(nil); resp != nil {
		t.Errorf("expected nil for nil input, got %v", resp)
	}
}

func TestPipelineStatusToProto_NoTimestamps(t *testing.T) {
	p := &core.Pipeline{
		ID:     "pipe-456",
		Name:   "empty",
		Status: core.StatusPending,
	}
	resp := PipelineStatusToProto(p)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.StartedAt != nil {
		t.Error("StartedAt should be nil for zero time")
	}
	if resp.Duration != nil {
		t.Error("Duration should be nil when StartedAt is zero")
	}
}

func TestPipelineStatusToProto_PendingJobs(t *testing.T) {
	p := &core.Pipeline{
		ID:     "pipe-789",
		Name:   "pending",
		Status: core.StatusPending,
		Jobs: []*core.Job{
			{
				Name:   "build",
				Status: core.StatusPending,
				Steps: []*core.Step{
					{
						Name:   "compile",
						Status: core.StatusPending,
					},
				},
			},
		},
	}
	resp := PipelineStatusToProto(p)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if len(resp.Jobs) != 1 {
		t.Fatalf("Jobs count = %d, want 1", len(resp.Jobs))
	}
	if resp.Jobs[0].Duration != nil {
		t.Error("Job Duration should be nil for pending job")
	}
	if resp.Jobs[0].Steps[0].Duration != nil {
		t.Error("Step Duration should be nil for pending step")
	}
}

func TestEventTypeToProto(t *testing.T) {
	tests := []struct {
		core  core.EventType
		proto seedeev1.EventType
	}{
		{core.EventPipelineStarted, seedeev1.EventType_EVENT_TYPE_PIPELINE_STARTED},
		{core.EventPipelineFinished, seedeev1.EventType_EVENT_TYPE_PIPELINE_FINISHED},
		{core.EventJobStarted, seedeev1.EventType_EVENT_TYPE_JOB_STARTED},
		{core.EventJobFinished, seedeev1.EventType_EVENT_TYPE_JOB_FINISHED},
		{core.EventJobSkipped, seedeev1.EventType_EVENT_TYPE_JOB_SKIPPED},
		{core.EventStepStarted, seedeev1.EventType_EVENT_TYPE_STEP_STARTED},
		{core.EventStepFinished, seedeev1.EventType_EVENT_TYPE_STEP_FINISHED},
		{core.EventStepLog, seedeev1.EventType_EVENT_TYPE_STEP_LOG},
	}
	for _, tt := range tests {
		t.Run(string(tt.core), func(t *testing.T) {
			got := EventTypeToProto(tt.core)
			if got != tt.proto {
				t.Errorf("EventTypeToProto(%q) = %v, want %v", tt.core, got, tt.proto)
			}
		})
	}
}

func TestEventTypeToProto_Unknown(t *testing.T) {
	got := EventTypeToProto(core.EventType("bogus"))
	if got != seedeev1.EventType_EVENT_TYPE_UNSPECIFIED {
		t.Errorf("EventTypeToProto(bogus) = %v, want UNSPECIFIED", got)
	}
}

func TestEventToProto(t *testing.T) {
	ts := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	event := core.Event{
		Type:         core.EventStepLog,
		Timestamp:    ts,
		PipelineID:   "pipe-1",
		PipelineName: "my-pipeline",
		JobName:      "build",
		StepName:     "compile",
		LogData:      []byte("hello world"),
		IsStderr:     true,
		Status:       core.StatusRunning,
		ExitCode:     42,
		Error:        "some error",
		Duration:     5 * time.Second,
	}

	pe := EventToProto(&event)
	if pe.PipelineId != "pipe-1" {
		t.Errorf("PipelineId = %q, want %q", pe.PipelineId, "pipe-1")
	}
	if pe.Type != seedeev1.EventType_EVENT_TYPE_STEP_LOG {
		t.Errorf("Type = %v, want STEP_LOG", pe.Type)
	}
	if !pe.Timestamp.AsTime().Equal(ts) {
		t.Errorf("Timestamp = %v, want %v", pe.Timestamp.AsTime(), ts)
	}
	if pe.JobName != "build" {
		t.Errorf("JobName = %q, want %q", pe.JobName, "build")
	}
	if pe.StepName != "compile" {
		t.Errorf("StepName = %q, want %q", pe.StepName, "compile")
	}
	if string(pe.LogData) != "hello world" {
		t.Errorf("LogData = %q, want %q", string(pe.LogData), "hello world")
	}
	if !pe.IsStderr {
		t.Error("IsStderr should be true")
	}
	if pe.Status != seedeev1.Status_STATUS_RUNNING {
		t.Errorf("Status = %v, want STATUS_RUNNING", pe.Status)
	}
	if pe.ExitCode != 42 {
		t.Errorf("ExitCode = %d, want 42", pe.ExitCode)
	}
	if pe.Error != "some error" {
		t.Errorf("Error = %q, want %q", pe.Error, "some error")
	}
	if pe.Duration == nil || pe.Duration.AsDuration() != 5*time.Second {
		t.Errorf("Duration = %v, want 5s", pe.Duration)
	}
}

func TestEventToProto_ZeroTimestamp(t *testing.T) {
	event := core.Event{
		Type: core.EventPipelineStarted,
	}
	pe := EventToProto(&event)
	if pe.Timestamp != nil {
		t.Error("Timestamp should be nil for zero time")
	}
	if pe.Duration != nil {
		t.Error("Duration should be nil for zero duration")
	}
}
