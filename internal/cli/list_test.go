package cli

import (
	"bytes"
	"testing"

	"time"

	seedeev1 "github.com/rumpl/seedee/gen/seedee/v1"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestListCmd_RequiresServer(t *testing.T) {
	root := NewRootCmd()
	root.SetArgs([]string{"list"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing --server flag")
	}
}

func TestPrintPipelineList_Empty(t *testing.T) {
	buf := &bytes.Buffer{}
	printPipelineList(buf, nil)
	if got := buf.String(); got != "No pipelines found.\n" {
		t.Errorf("expected 'No pipelines found.', got %q", got)
	}
}

func TestPrintPipelineList_Table(t *testing.T) {
	buf := &bytes.Buffer{}

	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	pipelines := []*seedeev1.PipelineSummary{
		{
			PipelineId: "pipe-1",
			Name:       "build",
			Status:     seedeev1.Status_STATUS_SUCCESS,
			StartedAt:  timestamppb.New(now),
			Duration:   durationpb.New(15 * time.Second),
		},
		{
			PipelineId: "pipe-2",
			Name:       "test",
			Status:     seedeev1.Status_STATUS_RUNNING,
			StartedAt:  timestamppb.New(now),
		},
		{
			PipelineId: "pipe-3",
			Name:       "deploy",
			Status:     seedeev1.Status_STATUS_FAILED,
			StartedAt:  timestamppb.New(now),
			Duration:   durationpb.New(5 * time.Second),
		},
	}

	printPipelineList(buf, pipelines)
	output := buf.String()

	// Check header
	if !bytes.Contains(buf.Bytes(), []byte("ID")) {
		t.Error("expected header to contain ID")
	}
	if !bytes.Contains(buf.Bytes(), []byte("NAME")) {
		t.Error("expected header to contain NAME")
	}
	if !bytes.Contains(buf.Bytes(), []byte("STATUS")) {
		t.Error("expected header to contain STATUS")
	}
	if !bytes.Contains(buf.Bytes(), []byte("DURATION")) {
		t.Error("expected header to contain DURATION")
	}

	// Check rows
	if !bytes.Contains([]byte(output), []byte("pipe-1")) {
		t.Error("expected output to contain pipe-1")
	}
	if !bytes.Contains([]byte(output), []byte("build")) {
		t.Error("expected output to contain build")
	}
	if !bytes.Contains([]byte(output), []byte("pipe-2")) {
		t.Error("expected output to contain pipe-2")
	}
	if !bytes.Contains([]byte(output), []byte("-")) {
		t.Error("expected output to contain '-' for missing duration")
	}
}

func TestParseStatusFilter(t *testing.T) {
	tests := []struct {
		input   string
		want    seedeev1.Status
		wantErr bool
	}{
		{"pending", seedeev1.Status_STATUS_PENDING, false},
		{"running", seedeev1.Status_STATUS_RUNNING, false},
		{"success", seedeev1.Status_STATUS_SUCCESS, false},
		{"failed", seedeev1.Status_STATUS_FAILED, false},
		{"skipped", seedeev1.Status_STATUS_SKIPPED, false},
		{"canceled", seedeev1.Status_STATUS_CANCELED, false},
		{"invalid", seedeev1.Status_STATUS_UNSPECIFIED, true},
		{"", seedeev1.Status_STATUS_UNSPECIFIED, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseStatusFilter(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseStatusFilter(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseStatusFilter(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
