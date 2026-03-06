package cli

import (
	"bytes"
	"testing"

	"time"

	seedeev1 "github.com/rumpl/seedee/gen/seedee/v1"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestStatusCmd_RequiresServer(t *testing.T) {
	root := NewRootCmd()
	root.SetArgs([]string{"status", "pipeline-123"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when --server is not set")
	}
	if err.Error() != "--server flag is required for status command" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCancelCmd_RequiresServer(t *testing.T) {
	root := NewRootCmd()
	root.SetArgs([]string{"cancel", "pipeline-123"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when --server is not set")
	}
	if err.Error() != "--server flag is required for cancel command" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewCIClient_AddsScheme(t *testing.T) {
	tests := []struct {
		name string
		addr string
	}{
		{"bare host:port", "localhost:8080"},
		{"with http", "http://localhost:8080"},
		{"with https", "https://localhost:8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newCIClient(tt.addr)
			if client == nil {
				t.Fatal("expected non-nil client")
			}
		})
	}
}

func TestStatusIcon(t *testing.T) {
	tests := []struct {
		status seedeev1.Status
		want   string
	}{
		{seedeev1.Status_STATUS_SUCCESS, "✓"},
		{seedeev1.Status_STATUS_FAILED, "✗"},
		{seedeev1.Status_STATUS_RUNNING, "●"},
		{seedeev1.Status_STATUS_PENDING, "○"},
		{seedeev1.Status_STATUS_SKIPPED, "⊘"},
		{seedeev1.Status_STATUS_CANCELED, "⊗"},
		{seedeev1.Status_STATUS_UNSPECIFIED, "?"},
	}

	for _, tt := range tests {
		t.Run(tt.status.String(), func(t *testing.T) {
			got := statusIcon(tt.status)
			if got != tt.want {
				t.Errorf("statusIcon(%v) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestPrintPipelineStatus(t *testing.T) {
	resp := &seedeev1.GetPipelineStatusResponse{
		PipelineId:   "pipeline-123",
		PipelineName: "my-pipeline",
		Status:       seedeev1.Status_STATUS_RUNNING,
		Duration:     durationpb.New(5 * time.Second),
		Jobs: []*seedeev1.JobStatus{
			{
				Name:     "build",
				Status:   seedeev1.Status_STATUS_SUCCESS,
				Duration: durationpb.New(3 * time.Second),
				Steps: []*seedeev1.StepStatus{
					{
						Name:     "compile",
						Status:   seedeev1.Status_STATUS_SUCCESS,
						Duration: durationpb.New(2 * time.Second),
					},
				},
			},
			{
				Name:   "test",
				Status: seedeev1.Status_STATUS_PENDING,
			},
		},
	}

	buf := &bytes.Buffer{}
	printPipelineStatus(buf, resp)
	output := buf.String()

	expected := []string{
		"● Pipeline: my-pipeline (pipeline-123)",
		"Status:   STATUS_RUNNING",
		"Duration: 5s",
		"✓ Job: build",
		"Duration: 3s",
		"✓ Step: compile",
		"Duration: 2s",
		"○ Job: test",
	}

	for _, exp := range expected {
		if !bytes.Contains(buf.Bytes(), []byte(exp)) {
			t.Errorf("output missing %q\ngot:\n%s", exp, output)
		}
	}
}
