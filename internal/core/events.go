package core

import (
	"fmt"
	"sync"
	"time"
)

// EventType identifies what kind of event occurred.
type EventType string

const (
	EventPipelineStarted  EventType = "pipeline_started"
	EventPipelineFinished EventType = "pipeline_finished"
	EventJobStarted       EventType = "job_started"
	EventJobFinished      EventType = "job_finished"
	EventJobSkipped       EventType = "job_skipped"
	EventStepStarted      EventType = "step_started"
	EventStepFinished     EventType = "step_finished"
	EventStepLog          EventType = "step_log"
)

// Event is emitted by the engine during execution.
type Event struct {
	Type         EventType
	Timestamp    time.Time
	PipelineID   string
	PipelineName string

	// Set for job-level and step-level events
	JobName string

	// Set for step-level events
	StepName string

	// Set for EventStepLog
	LogData  []byte
	IsStderr bool

	// Set for finished events
	Status   Status
	ExitCode int
	Error    string
	Duration time.Duration
}

// EventHandler receives events from the engine.
type EventHandler interface {
	HandleEvent(event Event) error
}

// MultiEventHandler fans out events to multiple handlers.
type MultiEventHandler struct {
	Handlers []EventHandler
}

// HandleEvent sends the event to all registered handlers.
func (m *MultiEventHandler) HandleEvent(event Event) error {
	for _, h := range m.Handlers {
		if err := h.HandleEvent(event); err != nil {
			return fmt.Errorf("dispatching %s event: %w", event.Type, err)
		}
	}
	return nil
}

// BufferedEventHandler collects events in a slice for assertions.
type BufferedEventHandler struct {
	mu     sync.Mutex
	Events []Event
}

// HandleEvent appends the event to the buffer in a thread-safe manner.
func (h *BufferedEventHandler) HandleEvent(event Event) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Events = append(h.Events, event)
	return nil
}
