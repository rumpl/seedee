package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/volume"
)

// CreateVolume creates a named Docker volume.
func (c *Client) CreateVolume(ctx context.Context, name string) error {
	_, err := c.cli.VolumeCreate(ctx, volume.CreateOptions{
		Name: name,
	})
	if err != nil {
		return fmt.Errorf("creating volume %s: %w", name, err)
	}
	return nil
}

// RemoveVolume removes a Docker volume.
func (c *Client) RemoveVolume(ctx context.Context, name string) error {
	return c.cli.VolumeRemove(ctx, name, true)
}
