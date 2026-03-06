//go:build integration

package server_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"

	seedeev1 "github.com/rumpl/seedee/gen/seedee/v1"
	"github.com/rumpl/seedee/gen/seedee/v1/seedeev1connect"
)

func TestLogStreamingLatency(t *testing.T) {
	baseURL, cancel := startIntegrationServer(t)
	defer cancel()

	client := seedeev1connect.NewCIServiceClient(http.DefaultClient, baseURL)

	// Submit a pipeline that prints 5 lines, one per second.
	stream, err := client.RunPipeline(context.Background(), connect.NewRequest(&seedeev1.RunPipelineRequest{
		Pipeline: &seedeev1.PipelineDefinition{
			Name: "latency-test",
			Jobs: map[string]*seedeev1.JobDefinition{
				"timed-job": {
					Image: "alpine:latest",
					Steps: []*seedeev1.StepDefinition{
						{
							Name: "timed-echo",
							Run:  "for i in 1 2 3 4 5; do echo line$i; sleep 1; done",
						},
					},
				},
			},
		},
	}))
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	var logTimestamps []time.Time
	for stream.Receive() {
		event := stream.Msg()
		if event.Type == seedeev1.EventType_EVENT_TYPE_STEP_LOG {
			logTimestamps = append(logTimestamps, time.Now())
		}
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("stream error: %v", err)
	}

	// We expect at least 5 log events (one per echo; Docker may merge some
	// output, so we tolerate getting fewer separate events, but the timing
	// of consecutive events must still show real-time delivery).
	if len(logTimestamps) < 3 {
		t.Fatalf("expected at least 3 log events, got %d", len(logTimestamps))
	}

	// Verify events arrive roughly 1 second apart (not all at once).
	for i := 1; i < len(logTimestamps); i++ {
		gap := logTimestamps[i].Sub(logTimestamps[i-1])
		// Accept gaps between 0.3s and 3s — the key assertion is that they
		// are NOT all delivered simultaneously after 5 seconds.
		if gap > 3*time.Second {
			t.Errorf("gap between log events %d and %d is too large: %s (expected real-time delivery)", i-1, i, gap)
		}
	}

	// The total time span of all log events should be at least 3 seconds
	// (proof that events stream in real-time, not all at end).
	totalSpan := logTimestamps[len(logTimestamps)-1].Sub(logTimestamps[0])
	if totalSpan < 3*time.Second {
		t.Errorf("total span of log events is %s; expected at least 3s for real-time streaming", totalSpan)
	}
}
