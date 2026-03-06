package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockRunner implements EngineRunner for testing.
type mockRunner struct {
	setupFn    func(ctx context.Context, job *Job) error
	runStepFn  func(ctx context.Context, job *Job, step *Step, stdout, stderr io.Writer) (*StepResult, error)
	teardownFn func(ctx context.Context, job *Job) error
}

func (m *mockRunner) Setup(ctx context.Context, job *Job) error {
	if m.setupFn != nil {
		return m.setupFn(ctx, job)
	}
	return nil
}

func (m *mockRunner) RunStep(ctx context.Context, job *Job, step *Step, stdout, stderr io.Writer) (*StepResult, error) {
	if m.runStepFn != nil {
		return m.runStepFn(ctx, job, step, stdout, stderr)
	}
	return &StepResult{ExitCode: 0}, nil
}

func (m *mockRunner) Teardown(ctx context.Context, job *Job) error {
	if m.teardownFn != nil {
		return m.teardownFn(ctx, job)
	}
	return nil
}

func TestEngine_SimpleSuccess(t *testing.T) {
	pipeline := &Pipeline{
		ID:   "test-1",
		Name: "simple",
		Jobs: []*Job{
			{
				Name: "build",
				Steps: []*Step{
					{Name: "compile", Command: "go build"},
					{Name: "test", Command: "go test"},
				},
				Status: StatusPending,
			},
		},
		Status: StatusPending,
	}

	var stepsRun []string
	var mu sync.Mutex

	runner := &mockRunner{
		runStepFn: func(ctx context.Context, job *Job, step *Step, stdout, stderr io.Writer) (*StepResult, error) {
			mu.Lock()
			stepsRun = append(stepsRun, step.Name)
			mu.Unlock()
			return &StepResult{ExitCode: 0}, nil
		},
	}

	engine := &Engine{Runner: runner}
	result, err := engine.Execute(context.Background(), pipeline)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusSuccess {
		t.Errorf("expected status %q, got %q", StatusSuccess, result.Status)
	}
	if len(stepsRun) != 2 {
		t.Fatalf("expected 2 steps run, got %d", len(stepsRun))
	}
	if stepsRun[0] != "compile" || stepsRun[1] != "test" {
		t.Errorf("expected steps [compile, test], got %v", stepsRun)
	}
	if result.Duration <= 0 {
		t.Error("expected positive duration")
	}
	if len(result.Jobs) != 1 {
		t.Fatalf("expected 1 job result, got %d", len(result.Jobs))
	}
	if result.Jobs[0].Status != StatusSuccess {
		t.Errorf("expected job status %q, got %q", StatusSuccess, result.Jobs[0].Status)
	}
}

func TestEngine_ParallelJobs(t *testing.T) {
	pipeline := &Pipeline{
		ID:   "test-2",
		Name: "parallel",
		Jobs: []*Job{
			{
				Name: "job-a",
				Steps: []*Step{
					{Name: "step-1", Command: "echo a"},
				},
				Status: StatusPending,
			},
			{
				Name: "job-b",
				Steps: []*Step{
					{Name: "step-1", Command: "echo b"},
				},
				Status: StatusPending,
			},
		},
		Status: StatusPending,
	}

	var running int64
	var maxConcurrent int64
	var mu sync.Mutex

	runner := &mockRunner{
		runStepFn: func(ctx context.Context, job *Job, step *Step, stdout, stderr io.Writer) (*StepResult, error) {
			cur := atomic.AddInt64(&running, 1)
			mu.Lock()
			if cur > maxConcurrent {
				maxConcurrent = cur
			}
			mu.Unlock()
			// Sleep to ensure overlap
			time.Sleep(50 * time.Millisecond)
			atomic.AddInt64(&running, -1)
			return &StepResult{ExitCode: 0}, nil
		},
	}

	engine := &Engine{Runner: runner}
	result, err := engine.Execute(context.Background(), pipeline)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusSuccess {
		t.Errorf("expected status %q, got %q", StatusSuccess, result.Status)
	}
	if maxConcurrent < 2 {
		t.Errorf("expected at least 2 concurrent jobs, got max %d", maxConcurrent)
	}
}

func TestEngine_StepFailureStopsJob(t *testing.T) {
	pipeline := &Pipeline{
		ID:   "test-3",
		Name: "step-failure",
		Jobs: []*Job{
			{
				Name: "build",
				Steps: []*Step{
					{Name: "step-1", Command: "ok"},
					{Name: "step-2", Command: "fail"},
					{Name: "step-3", Command: "should-not-run"},
				},
				Status: StatusPending,
			},
		},
		Status: StatusPending,
	}

	var stepsRun []string
	var mu sync.Mutex

	runner := &mockRunner{
		runStepFn: func(ctx context.Context, job *Job, step *Step, stdout, stderr io.Writer) (*StepResult, error) {
			mu.Lock()
			stepsRun = append(stepsRun, step.Name)
			mu.Unlock()
			if step.Name == "step-2" {
				return nil, fmt.Errorf("step-2 failed")
			}
			return &StepResult{ExitCode: 0}, nil
		},
	}

	engine := &Engine{Runner: runner}
	result, err := engine.Execute(context.Background(), pipeline)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusFailed {
		t.Errorf("expected status %q, got %q", StatusFailed, result.Status)
	}
	if len(stepsRun) != 2 {
		t.Fatalf("expected 2 steps run, got %d: %v", len(stepsRun), stepsRun)
	}
	if stepsRun[0] != "step-1" || stepsRun[1] != "step-2" {
		t.Errorf("expected steps [step-1, step-2], got %v", stepsRun)
	}
}

func TestEngine_FailedJobSkipsDependents(t *testing.T) {
	pipeline := &Pipeline{
		ID:   "test-4",
		Name: "skip-dependents",
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

	var jobsRun []string
	var mu sync.Mutex

	runner := &mockRunner{
		runStepFn: func(ctx context.Context, job *Job, step *Step, stdout, stderr io.Writer) (*StepResult, error) {
			mu.Lock()
			jobsRun = append(jobsRun, job.Name)
			mu.Unlock()
			if job.Name == "job-a" {
				return nil, fmt.Errorf("job-a failed")
			}
			return &StepResult{ExitCode: 0}, nil
		},
	}

	engine := &Engine{Runner: runner}
	result, err := engine.Execute(context.Background(), pipeline)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusFailed {
		t.Errorf("expected status %q, got %q", StatusFailed, result.Status)
	}
	// Only job-a should have run
	if len(jobsRun) != 1 {
		t.Fatalf("expected 1 job run, got %d: %v", len(jobsRun), jobsRun)
	}
	if jobsRun[0] != "job-a" {
		t.Errorf("expected job-a to run, got %v", jobsRun)
	}

	// Verify job-b was skipped
	for _, jr := range result.Jobs {
		if jr.JobName == "job-b" && jr.Status != StatusSkipped {
			t.Errorf("expected job-b to be skipped, got %q", jr.Status)
		}
	}
}

func TestEngine_ContextCancellation(t *testing.T) {
	pipeline := &Pipeline{
		ID:   "test-5",
		Name: "cancel",
		Jobs: []*Job{
			{
				Name: "slow-job",
				Steps: []*Step{
					{Name: "slow-step", Command: "sleep 10"},
				},
				Status: StatusPending,
			},
		},
		Status: StatusPending,
	}

	ctx, cancel := context.WithCancel(context.Background())

	runner := &mockRunner{
		runStepFn: func(ctx context.Context, job *Job, step *Step, stdout, stderr io.Writer) (*StepResult, error) {
			// Cancel the context while the step is "running"
			cancel()
			// Simulate checking context
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(10 * time.Second):
				return &StepResult{ExitCode: 0}, nil
			}
		},
	}

	engine := &Engine{Runner: runner}
	result, err := engine.Execute(ctx, pipeline)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusFailed {
		// Could be Failed or Canceled depending on timing
		if result.Status != StatusCanceled {
			t.Errorf("expected status %q or %q, got %q", StatusFailed, StatusCanceled, result.Status)
		}
	}
}

func TestEngine_ContextCancellationBetweenGroups(t *testing.T) {
	pipeline := &Pipeline{
		ID:   "test-5b",
		Name: "cancel-between-groups",
		Jobs: []*Job{
			{
				Name: "job-a",
				Steps: []*Step{
					{Name: "step-1", Command: "echo"},
				},
				Status: StatusPending,
			},
			{
				Name:      "job-b",
				DependsOn: []string{"job-a"},
				Steps: []*Step{
					{Name: "step-1", Command: "echo"},
				},
				Status: StatusPending,
			},
		},
		Status: StatusPending,
	}

	ctx, cancel := context.WithCancel(context.Background())

	runner := &mockRunner{
		runStepFn: func(ctx context.Context, job *Job, step *Step, stdout, stderr io.Writer) (*StepResult, error) {
			if job.Name == "job-a" {
				cancel() // Cancel after first job
			}
			return &StepResult{ExitCode: 0}, nil
		},
	}

	engine := &Engine{Runner: runner}
	result, err := engine.Execute(ctx, pipeline)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusCanceled {
		t.Errorf("expected status %q, got %q", StatusCanceled, result.Status)
	}
}

func TestEngine_DiamondDependency(t *testing.T) {
	pipeline := &Pipeline{
		ID:   "test-6",
		Name: "diamond",
		Jobs: []*Job{
			{
				Name: "A",
				Steps: []*Step{
					{Name: "step-1", Command: "echo A"},
				},
				Status: StatusPending,
			},
			{
				Name:      "B",
				DependsOn: []string{"A"},
				Steps: []*Step{
					{Name: "step-1", Command: "echo B"},
				},
				Status: StatusPending,
			},
			{
				Name:      "C",
				DependsOn: []string{"A"},
				Steps: []*Step{
					{Name: "step-1", Command: "echo C"},
				},
				Status: StatusPending,
			},
			{
				Name:      "D",
				DependsOn: []string{"B", "C"},
				Steps: []*Step{
					{Name: "step-1", Command: "echo D"},
				},
				Status: StatusPending,
			},
		},
		Status: StatusPending,
	}

	var executionOrder []string
	var mu sync.Mutex

	runner := &mockRunner{
		runStepFn: func(ctx context.Context, job *Job, step *Step, stdout, stderr io.Writer) (*StepResult, error) {
			mu.Lock()
			executionOrder = append(executionOrder, job.Name)
			mu.Unlock()
			return &StepResult{ExitCode: 0}, nil
		},
	}

	engine := &Engine{Runner: runner}
	result, err := engine.Execute(context.Background(), pipeline)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusSuccess {
		t.Errorf("expected status %q, got %q", StatusSuccess, result.Status)
	}
	if len(executionOrder) != 4 {
		t.Fatalf("expected 4 jobs run, got %d: %v", len(executionOrder), executionOrder)
	}

	// A must come before B and C; B and C must come before D
	indexOf := func(name string) int {
		for i, n := range executionOrder {
			if n == name {
				return i
			}
		}
		return -1
	}

	if indexOf("A") >= indexOf("B") || indexOf("A") >= indexOf("C") {
		t.Errorf("A should run before B and C: %v", executionOrder)
	}
	if indexOf("B") >= indexOf("D") || indexOf("C") >= indexOf("D") {
		t.Errorf("B and C should run before D: %v", executionOrder)
	}
}

func TestEngine_LogsAreRouted(t *testing.T) {
	pipeline := &Pipeline{
		ID:   "test-7",
		Name: "logs",
		Jobs: []*Job{
			{
				Name: "log-job",
				Steps: []*Step{
					{Name: "log-step", Command: "echo hello"},
				},
				Status: StatusPending,
			},
		},
		Status: StatusPending,
	}

	handler := &BufferedEventHandler{}

	runner := &mockRunner{
		runStepFn: func(ctx context.Context, job *Job, step *Step, stdout, stderr io.Writer) (*StepResult, error) {
			_, _ = stdout.Write([]byte("stdout output"))
			_, _ = stderr.Write([]byte("stderr output"))
			return &StepResult{ExitCode: 0}, nil
		},
	}

	engine := &Engine{Runner: runner, EventHandler: handler}
	result, err := engine.Execute(context.Background(), pipeline)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusSuccess {
		t.Errorf("expected status %q, got %q", StatusSuccess, result.Status)
	}

	handler.mu.Lock()
	defer handler.mu.Unlock()

	// Check for log events
	foundStdout := false
	foundStderr := false
	for _, event := range handler.Events {
		if event.Type != EventStepLog {
			continue
		}
		if event.JobName != "log-job" || event.StepName != "log-step" {
			t.Errorf("unexpected log event job/step: %s/%s", event.JobName, event.StepName)
		}
		if string(event.LogData) == "stdout output" && !event.IsStderr {
			foundStdout = true
		}
		if string(event.LogData) == "stderr output" && event.IsStderr {
			foundStderr = true
		}
	}

	if !foundStdout {
		t.Error("stdout output not routed to EventHandler")
	}
	if !foundStderr {
		t.Error("stderr output not routed to EventHandler")
	}
}

func TestEngine_SetupFailure(t *testing.T) {
	pipeline := &Pipeline{
		ID:   "test-8",
		Name: "setup-fail",
		Jobs: []*Job{
			{
				Name: "setup-fail-job",
				Steps: []*Step{
					{Name: "should-not-run", Command: "echo"},
				},
				Status: StatusPending,
			},
		},
		Status: StatusPending,
	}

	var stepsRun []string
	var mu sync.Mutex

	runner := &mockRunner{
		setupFn: func(ctx context.Context, job *Job) error {
			return fmt.Errorf("setup failed")
		},
		runStepFn: func(ctx context.Context, job *Job, step *Step, stdout, stderr io.Writer) (*StepResult, error) {
			mu.Lock()
			stepsRun = append(stepsRun, step.Name)
			mu.Unlock()
			return &StepResult{ExitCode: 0}, nil
		},
	}

	engine := &Engine{Runner: runner}
	result, err := engine.Execute(context.Background(), pipeline)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusFailed {
		t.Errorf("expected status %q, got %q", StatusFailed, result.Status)
	}
	if len(stepsRun) != 0 {
		t.Errorf("expected no steps to run, got %v", stepsRun)
	}
}

func TestEngine_TeardownAlwaysCalled(t *testing.T) {
	pipeline := &Pipeline{
		ID:   "test-9",
		Name: "teardown",
		Jobs: []*Job{
			{
				Name: "teardown-job",
				Steps: []*Step{
					{Name: "fail-step", Command: "fail"},
				},
				Status: StatusPending,
			},
		},
		Status: StatusPending,
	}

	var teardownCalled int64

	runner := &mockRunner{
		runStepFn: func(ctx context.Context, job *Job, step *Step, stdout, stderr io.Writer) (*StepResult, error) {
			return nil, fmt.Errorf("step failed")
		},
		teardownFn: func(ctx context.Context, job *Job) error {
			atomic.AddInt64(&teardownCalled, 1)
			return nil
		},
	}

	engine := &Engine{Runner: runner}
	_, err := engine.Execute(context.Background(), pipeline)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if atomic.LoadInt64(&teardownCalled) != 1 {
		t.Errorf("expected teardown to be called once, got %d", atomic.LoadInt64(&teardownCalled))
	}
}

func TestEngine_TeardownCalledOnSetupFailure(t *testing.T) {
	pipeline := &Pipeline{
		ID:   "test-9b",
		Name: "teardown-setup-fail",
		Jobs: []*Job{
			{
				Name: "job",
				Steps: []*Step{
					{Name: "step", Command: "echo"},
				},
				Status: StatusPending,
			},
		},
		Status: StatusPending,
	}

	var teardownCalled int64

	runner := &mockRunner{
		setupFn: func(ctx context.Context, job *Job) error {
			return errors.New("setup error")
		},
		teardownFn: func(ctx context.Context, job *Job) error {
			atomic.AddInt64(&teardownCalled, 1)
			return nil
		},
	}

	engine := &Engine{Runner: runner}
	_, _ = engine.Execute(context.Background(), pipeline)

	if atomic.LoadInt64(&teardownCalled) != 1 {
		t.Errorf("expected teardown to be called on setup failure, got %d calls", atomic.LoadInt64(&teardownCalled))
	}
}

func TestEngine_PipelineResult(t *testing.T) {
	pipeline := &Pipeline{
		ID:   "test-10",
		Name: "result-check",
		Jobs: []*Job{
			{
				Name: "job-a",
				Steps: []*Step{
					{Name: "step-1", Command: "echo a"},
					{Name: "step-2", Command: "echo b"},
				},
				Status: StatusPending,
			},
			{
				Name: "job-b",
				Steps: []*Step{
					{Name: "step-1", Command: "echo c"},
				},
				Status: StatusPending,
			},
		},
		Status: StatusPending,
	}

	runner := &mockRunner{
		runStepFn: func(ctx context.Context, job *Job, step *Step, stdout, stderr io.Writer) (*StepResult, error) {
			time.Sleep(10 * time.Millisecond) // ensure measurable duration
			return &StepResult{ExitCode: 0}, nil
		},
	}

	engine := &Engine{Runner: runner}
	result, err := engine.Execute(context.Background(), pipeline)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check pipeline result
	if result.PipelineID != "test-10" {
		t.Errorf("expected pipeline ID %q, got %q", "test-10", result.PipelineID)
	}
	if result.Status != StatusSuccess {
		t.Errorf("expected status %q, got %q", StatusSuccess, result.Status)
	}
	if result.Duration <= 0 {
		t.Error("expected positive duration")
	}
	if result.Error != nil {
		t.Errorf("expected no error, got %v", result.Error)
	}

	// Check job results
	if len(result.Jobs) != 2 {
		t.Fatalf("expected 2 job results, got %d", len(result.Jobs))
	}

	for _, jr := range result.Jobs {
		if jr.Status != StatusSuccess {
			t.Errorf("job %s: expected status %q, got %q", jr.JobName, StatusSuccess, jr.Status)
		}
	}

	// Check pipeline timing
	if pipeline.StartedAt.IsZero() {
		t.Error("expected non-zero start time")
	}
	if pipeline.EndedAt.IsZero() {
		t.Error("expected non-zero end time")
	}
	if !pipeline.EndedAt.After(pipeline.StartedAt) {
		t.Error("expected end time after start time")
	}
}

func TestEngine_NonZeroExitCode(t *testing.T) {
	pipeline := &Pipeline{
		ID:   "test-11",
		Name: "exit-code",
		Jobs: []*Job{
			{
				Name: "job",
				Steps: []*Step{
					{Name: "step-1", Command: "exit 1"},
					{Name: "step-2", Command: "should not run"},
				},
				Status: StatusPending,
			},
		},
		Status: StatusPending,
	}

	var stepsRun []string
	var mu sync.Mutex

	runner := &mockRunner{
		runStepFn: func(ctx context.Context, job *Job, step *Step, stdout, stderr io.Writer) (*StepResult, error) {
			mu.Lock()
			stepsRun = append(stepsRun, step.Name)
			mu.Unlock()
			if step.Name == "step-1" {
				return &StepResult{ExitCode: 1}, nil
			}
			return &StepResult{ExitCode: 0}, nil
		},
	}

	engine := &Engine{Runner: runner}
	result, err := engine.Execute(context.Background(), pipeline)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusFailed {
		t.Errorf("expected status %q, got %q", StatusFailed, result.Status)
	}
	if len(stepsRun) != 1 {
		t.Fatalf("expected 1 step run, got %d: %v", len(stepsRun), stepsRun)
	}
}

func TestEngine_EventLogAdapterNilHandler(t *testing.T) {
	adapter := &eventLogAdapter{
		handler:  nil,
		job:      "test-job",
		step:     "test-step",
		isStderr: false,
	}

	n, err := adapter.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 5 {
		t.Errorf("expected n=5, got %d", n)
	}
}

func TestEngine_StdoutEventHandler(t *testing.T) {
	h := &StdoutEventHandler{}
	err := h.HandleEvent(Event{
		Type:         EventPipelineStarted,
		Timestamp:    time.Now(),
		PipelineName: "test-pipeline",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
