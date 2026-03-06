package core

import "fmt"

// StdoutEventHandler prints events to stdout for local CLI use.
type StdoutEventHandler struct{}

// HandleEvent prints a human-readable representation of the event to stdout.
func (h *StdoutEventHandler) HandleEvent(event Event) error {
	switch event.Type {
	case EventPipelineStarted:
		fmt.Printf("=== Pipeline %q started ===\n", event.PipelineName)
	case EventJobStarted:
		fmt.Printf("--- Job %q started ---\n", event.JobName)
	case EventStepLog:
		prefix := fmt.Sprintf("[%s/%s] ", event.JobName, event.StepName)
		fmt.Print(prefix + string(event.LogData))
	case EventStepFinished:
		status := "✓"
		if event.Status == StatusFailed {
			status = "✗"
		}
		fmt.Printf("[%s/%s] %s (%s)\n", event.JobName, event.StepName, status, event.Duration)
	case EventJobFinished:
		fmt.Printf("--- Job %q %s (%s) ---\n", event.JobName, event.Status, event.Duration)
	case EventPipelineFinished:
		fmt.Printf("=== Pipeline %s (%s) ===\n", event.Status, event.Duration)
	}
	return nil
}
