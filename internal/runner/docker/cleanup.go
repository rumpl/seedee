package docker

import (
	"context"
	"errors"
	"sync"
)

// CleanupRegistry tracks resources that need cleanup, ensuring nothing leaks.
type CleanupRegistry struct {
	mu      sync.Mutex
	volumes []string
}

// NewCleanupRegistry creates a new CleanupRegistry.
func NewCleanupRegistry() *CleanupRegistry {
	return &CleanupRegistry{}
}

// RegisterVolume adds a volume name to the cleanup registry.
func (r *CleanupRegistry) RegisterVolume(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.volumes = append(r.volumes, name)
}

// Volumes returns the currently registered volume names.
func (r *CleanupRegistry) Volumes() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]string, len(r.volumes))
	copy(out, r.volumes)

	return out
}

// CleanupAll removes all registered resources.
// Called at the end of a pipeline run or on fatal error.
func (r *CleanupRegistry) CleanupAll(ctx context.Context, client *Client) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errs []error
	for _, v := range r.volumes {
		if err := client.RemoveVolume(ctx, v); err != nil {
			errs = append(errs, err)
		}
	}
	r.volumes = nil

	return errors.Join(errs...)
}
