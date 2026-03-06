package core

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestEvents_PipelineStartFinish(t *testing.T) {
	pipeline := &Pipeline{
		ID:   "evt-1",
		Name: "start-finish",
		Jobs: []*Job{
			{
				Name: "job-a",
				Steps: []*Step{
					{Name: "step-1", Command: "echo hello"},
				},
				Status: StatusPending,
			},
		},
		Status: StatusPending,
	}

	handler := &BufferedEventHandler{}
	runner := &mockRunner{}
	engine := &Engine{Runner: runner, EventHandler: handler}

	_, err := engine.Execute(context.Background(), pipeline)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handler.mu.Lock()
	defer handler.mu.Unlock()

	if len(handler.Events) == 0 {
		t.Fatal("expected events to be emitted")
	}

	// First event should be pipeline started
	if handler.Events[0].Type != EventPipelineStarted {
		t.Errorf("expected first event to be %q, got %q", EventPipelineStarted, handler.Events[0].Type)
	}
	if handler.Events[0].PipelineID != "evt-1" {
		t.Errorf("expected pipeline ID %q, got %q", "evt-1", handler.Events[0].PipelineID)
	}
	if handler.Events[0].PipelineName != "start-finish" {
		t.Errorf("expected pipeline name %q, got %q", "start-finish", handler.Events[0].PipelineName)
	}

	// Last event should be pipeline finished
	last := handler.Events[len(handler.Events)-1]
	if last.Type != EventPipelineFinished {
		t.Errorf("expected last event to be %q, got %q", EventPipelineFinished, last.Type)
	}
	if last.Status != StatusSuccess {
		t.Errorf("expected pipeline finished status %q, got %q", StatusSuccess, last.Status)
	}
	if last.Duration <= 0 {
		t.Error("expected positive duration on pipeline finished event")
	}
}

func TestEvents_JobStartedBeforeFinished(t *testing.T) {
	pipeline := &Pipeline{
		ID:   "evt-2",
		Name: "job-order",
		Jobs: []*Job{
			{
				Name: "job-a",
				Steps: []*Step{
					{Name: "step-1", Command: "echo"},
				},
				Status: StatusPending,
			},
		},
		Status: StatusPending,
	}

	handler := &BufferedEventHandler{}
	runner := &mockRunner{}
	engine := &Engine{Runner: runner, EventHandler: handler}

	_, err := engine.Execute(context.Background(), pipeline)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handler.mu.Lock()
	defer handler.mu.Unlock()

	jobStartedIdx := -1
	jobFinishedIdx := -1
	for i, evt := range handler.Events {
		if evt.Type == EventJobStarted && evt.JobName == "job-a" {
			jobStartedIdx = i
		}
		if evt.Type == EventJobFinished && evt.JobName == "job-a" {
			jobFinishedIdx = i
		}
	}

	if jobStartedIdx == -1 {
		t.Fatal("expected EventJobStarted for job-a")
	}
	if jobFinishedIdx == -1 {
		t.Fatal("expected EventJobFinished for job-a")
	}
	if jobStartedIdx >= jobFinishedIdx {
		t.Errorf("expected JobStarted (idx %d) before JobFinished (idx %d)", jobStartedIdx, jobFinishedIdx)
	}
}

func TestEvents_StepLogContainsData(t *testing.T) {
	pipeline := &Pipeline{
		ID:   "evt-3",
		Name: "step-log",
		Jobs: []*Job{
			{
				Name: "job-a",
				Steps: []*Step{
					{Name: "step-1", Command: "echo hello"},
				},
				Status: StatusPending,
			},
		},
		Status: StatusPending,
	}

	handler := &BufferedEventHandler{}
	runner := &mockRunner{
		runStepFn: func(ctx context.Context, job *Job, step *Step, stdout, stderr io.Writer) (*StepResult, error) {
			_, _ = stdout.Write([]byte("hello world"))
			return &StepResult{ExitCode: 0}, nil
		},
	}
	engine := &Engine{Runner: runner, EventHandler: handler}

	_, err := engine.Execute(context.Background(), pipeline)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handler.mu.Lock()
	defer handler.mu.Unlock()

	found := false
	for _, evt := range handler.Events {
		if evt.Type == EventStepLog && string(evt.LogData) == "hello world" {
			found = true
			if evt.JobName != "job-a" {
				t.Errorf("expected job name %q, got %q", "job-a", evt.JobName)
			}
			if evt.StepName != "step-1" {
				t.Errorf("expected step name %q, got %q", "step-1", evt.StepName)
			}
			break
		}
	}
	if !found {
		t.Error("expected EventStepLog with data 'hello world'")
	}
}

func TestEvents_SkippedJobEmitsEvent(t *testing.T) {
	pipeline := &Pipeline{
		ID:   "evt-4",
		Name: "skip",
		Jobs: []*Job{
			{
				Name: "job-a",
				Steps: []*Step{
					{Name: "fail-step", Command: "fail"},
				},
				Status: StatusPending,
			},
			{
				Name:      "job-b",
				DependsOn: []string{"job-a"},
				Steps: []*Step{
					{Name: "should-not-run", Command: "echo"},
				},
				Status: StatusPending,
			},
		},
		Status: StatusPending,
	}

	handler := &BufferedEventHandler{}
	runner := &mockRunner{
		runStepFn: func(ctx context.Context, job *Job, step *Step, stdout, stderr io.Writer) (*StepResult, error) {
			if job.Name == "job-a" {
				return nil, fmt.Errorf("job-a failed")
			}
			return &StepResult{ExitCode: 0}, nil
		},
	}
	engine := &Engine{Runner: runner, EventHandler: handler}

	_, err := engine.Execute(context.Background(), pipeline)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handler.mu.Lock()
	defer handler.mu.Unlock()

	found := false
	for _, evt := range handler.Events {
		if evt.Type == EventJobSkipped && evt.JobName == "job-b" {
			found = true
			if evt.Status != StatusSkipped {
				t.Errorf("expected status %q, got %q", StatusSkipped, evt.Status)
			}
			break
		}
	}
	if !found {
		t.Error("expected EventJobSkipped for job-b")
	}
}

func TestEvents_BufferedHandlerGoroutineSafe(t *testing.T) {
	handler := &BufferedEventHandler{}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_ = handler.HandleEvent(&Event{
				Type:      EventStepLog,
				Timestamp: time.Now(),
				JobName:   fmt.Sprintf("job-%d", n),
			})
		}(i)
	}
	wg.Wait()

	handler.mu.Lock()
	defer handler.mu.Unlock()

	if len(handler.Events) != 100 {
		t.Errorf("expected 100 events, got %d", len(handler.Events))
	}
}

func TestEvents_MultiEventHandlerFanout(t *testing.T) {
	h1 := &BufferedEventHandler{}
	h2 := &BufferedEventHandler{}
	multi := &MultiEventHandler{Handlers: []EventHandler{h1, h2}}

	event := Event{
		Type:         EventPipelineStarted,
		Timestamp:    time.Now(),
		PipelineID:   "test-multi",
		PipelineName: "multi-test",
	}

	err := multi.HandleEvent(&event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	h1.mu.Lock()
	if len(h1.Events) != 1 {
		t.Errorf("handler 1: expected 1 event, got %d", len(h1.Events))
	}
	h1.mu.Unlock()

	h2.mu.Lock()
	if len(h2.Events) != 1 {
		t.Errorf("handler 2: expected 1 event, got %d", len(h2.Events))
	}
	h2.mu.Unlock()
}

type errHandler struct {
	err error
}

func (h *errHandler) HandleEvent(_ *Event) error {
	return h.err
}

func TestEvents_MultiEventHandlerStopsOnError(t *testing.T) {
	h1 := &errHandler{err: fmt.Errorf("handler error")}
	h2 := &BufferedEventHandler{}
	multi := &MultiEventHandler{Handlers: []EventHandler{h1, h2}}

	err := multi.HandleEvent(&Event{Type: EventPipelineStarted})
	if err == nil {
		t.Fatal("expected error from MultiEventHandler")
	}
	if !strings.Contains(err.Error(), "handler error") {
		t.Errorf("expected error to contain 'handler error', got %q", err.Error())
	}

	h2.mu.Lock()
	if len(h2.Events) != 0 {
		t.Errorf("handler 2 should not have received events, got %d", len(h2.Events))
	}
	h2.mu.Unlock()
}

func TestEvents_TimestampsNonZeroAndOrdered(t *testing.T) {
	pipeline := &Pipeline{
		ID:   "evt-ts",
		Name: "timestamps",
		Jobs: []*Job{
			{
				Name: "job-a",
				Steps: []*Step{
					{Name: "step-1", Command: "echo"},
				},
				Status: StatusPending,
			},
		},
		Status: StatusPending,
	}

	handler := &BufferedEventHandler{}
	runner := &mockRunner{
		runStepFn: func(ctx context.Context, job *Job, step *Step, stdout, stderr io.Writer) (*StepResult, error) {
			time.Sleep(1 * time.Millisecond) // ensure measurable time passes
			return &StepResult{ExitCode: 0}, nil
		},
	}
	engine := &Engine{Runner: runner, EventHandler: handler}

	_, err := engine.Execute(context.Background(), pipeline)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handler.mu.Lock()
	defer handler.mu.Unlock()

	if len(handler.Events) == 0 {
		t.Fatal("expected events")
	}

	for i, evt := range handler.Events {
		if evt.Timestamp.IsZero() {
			t.Errorf("event %d (%s) has zero timestamp", i, evt.Type)
		}
		if i > 0 {
			prev := handler.Events[i-1]
			if evt.Timestamp.Before(prev.Timestamp) {
				t.Errorf("event %d (%s) timestamp %v is before event %d (%s) timestamp %v",
					i, evt.Type, evt.Timestamp, i-1, prev.Type, prev.Timestamp)
			}
		}
	}
}

func TestEvents_StepStartedAndFinished(t *testing.T) {
	pipeline := &Pipeline{
		ID:   "evt-step",
		Name: "step-lifecycle",
		Jobs: []*Job{
			{
				Name: "job-a",
				Steps: []*Step{
					{Name: "step-1", Command: "echo"},
					{Name: "step-2", Command: "echo"},
				},
				Status: StatusPending,
			},
		},
		Status: StatusPending,
	}

	handler := &BufferedEventHandler{}
	runner := &mockRunner{}
	engine := &Engine{Runner: runner, EventHandler: handler}

	_, err := engine.Execute(context.Background(), pipeline)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handler.mu.Lock()
	defer handler.mu.Unlock()

	// Collect step events in order
	type stepEvent struct {
		typ  EventType
		name string
		idx  int
	}
	var stepEvents []stepEvent
	for i, evt := range handler.Events {
		if evt.Type == EventStepStarted || evt.Type == EventStepFinished {
			stepEvents = append(stepEvents, stepEvent{typ: evt.Type, name: evt.StepName, idx: i})
		}
	}

	// We expect: step-1 started, step-1 finished, step-2 started, step-2 finished
	if len(stepEvents) != 4 {
		t.Fatalf("expected 4 step events, got %d", len(stepEvents))
	}

	expected := []struct {
		typ  EventType
		name string
	}{
		{EventStepStarted, "step-1"},
		{EventStepFinished, "step-1"},
		{EventStepStarted, "step-2"},
		{EventStepFinished, "step-2"},
	}

	for i, exp := range expected {
		if stepEvents[i].typ != exp.typ || stepEvents[i].name != exp.name {
			t.Errorf("step event %d: expected %s/%s, got %s/%s",
				i, exp.typ, exp.name, stepEvents[i].typ, stepEvents[i].name)
		}
	}
}

func TestEvents_FullLifecycleOrder(t *testing.T) {
	pipeline := &Pipeline{
		ID:   "evt-full",
		Name: "full-lifecycle",
		Jobs: []*Job{
			{
				Name: "job-a",
				Steps: []*Step{
					{Name: "step-1", Command: "echo"},
				},
				Status: StatusPending,
			},
		},
		Status: StatusPending,
	}

	handler := &BufferedEventHandler{}
	runner := &mockRunner{}
	engine := &Engine{Runner: runner, EventHandler: handler}

	_, err := engine.Execute(context.Background(), pipeline)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handler.mu.Lock()
	defer handler.mu.Unlock()

	// Expected order: PipelineStarted, JobStarted, StepStarted, StepFinished, JobFinished, PipelineFinished
	expectedTypes := []EventType{
		EventPipelineStarted,
		EventJobStarted,
		EventStepStarted,
		EventStepFinished,
		EventJobFinished,
		EventPipelineFinished,
	}

	if len(handler.Events) != len(expectedTypes) {
		var actual []EventType
		for _, evt := range handler.Events {
			actual = append(actual, evt.Type)
		}
		t.Fatalf("expected %d events, got %d: %v", len(expectedTypes), len(handler.Events), actual)
	}

	for i, exp := range expectedTypes {
		if handler.Events[i].Type != exp {
			t.Errorf("event %d: expected %q, got %q", i, exp, handler.Events[i].Type)
		}
	}
}

func TestEvents_StepFinishedIncludesExitCode(t *testing.T) {
	pipeline := &Pipeline{
		ID:   "evt-exit",
		Name: "exit-code-event",
		Jobs: []*Job{
			{
				Name: "job-a",
				Steps: []*Step{
					{Name: "step-1", Command: "fail"},
				},
				Status: StatusPending,
			},
		},
		Status: StatusPending,
	}

	handler := &BufferedEventHandler{}
	runner := &mockRunner{
		runStepFn: func(ctx context.Context, job *Job, step *Step, stdout, stderr io.Writer) (*StepResult, error) {
			return &StepResult{ExitCode: 42}, nil
		},
	}
	engine := &Engine{Runner: runner, EventHandler: handler}

	_, err := engine.Execute(context.Background(), pipeline)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handler.mu.Lock()
	defer handler.mu.Unlock()

	for _, evt := range handler.Events {
		if evt.Type == EventStepFinished && evt.StepName == "step-1" {
			if evt.ExitCode != 42 {
				t.Errorf("expected exit code 42, got %d", evt.ExitCode)
			}
			if evt.Status != StatusFailed {
				t.Errorf("expected status %q, got %q", StatusFailed, evt.Status)
			}
			return
		}
	}
	t.Error("expected EventStepFinished for step-1")
}

func TestEvents_NilHandlerDoesNotPanic(t *testing.T) {
	pipeline := &Pipeline{
		ID:   "evt-nil",
		Name: "nil-handler",
		Jobs: []*Job{
			{
				Name: "job-a",
				Steps: []*Step{
					{Name: "step-1", Command: "echo"},
				},
				Status: StatusPending,
			},
		},
		Status: StatusPending,
	}

	runner := &mockRunner{}
	engine := &Engine{Runner: runner} // no EventHandler set

	result, err := engine.Execute(context.Background(), pipeline)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusSuccess {
		t.Errorf("expected status %q, got %q", StatusSuccess, result.Status)
	}
}
