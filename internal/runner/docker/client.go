// Package docker provides a thin wrapper around the Docker SDK for
// managing the container lifecycle: pulling images, creating and running
// containers, and streaming their output.
package docker

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// Client wraps the Docker SDK with higher-level operations.
type Client struct {
	cli *client.Client
}

// NewClient creates a Client connected to the local Docker daemon.
func NewClient() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("creating docker client: %w", err)
	}
	return &Client{cli: cli}, nil
}

// Close closes the underlying Docker client.
func (c *Client) Close() error {
	if err := c.cli.Close(); err != nil {
		return fmt.Errorf("closing docker client: %w", err)
	}
	return nil
}

// Ping checks if the Docker daemon is reachable.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.cli.Ping(ctx)
	if err != nil {
		return fmt.Errorf("pinging docker daemon: %w", err)
	}
	return nil
}

// PullImage pulls a Docker image, writing progress to the given writer.
func (c *Client) PullImage(ctx context.Context, ref string, output io.Writer) error {
	reader, err := c.cli.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pulling image %s: %w", ref, err)
	}
	defer func() { _ = reader.Close() }()

	if _, err := io.Copy(output, reader); err != nil {
		return fmt.Errorf("reading pull output for %s: %w", ref, err)
	}
	return nil
}

// RunOptions configures a container run.
type RunOptions struct {
	Image   string
	Command string
	Env     map[string]string
	Binds   []string // Docker volume binds
	Stdout  io.Writer
	Stderr  io.Writer
}

// RunContainer creates a container, starts it, waits for it to exit,
// and returns the exit code. stdout/stderr are streamed to the given writers.
// The container is always removed after this call returns.
func (c *Client) RunContainer(ctx context.Context, opts *RunOptions) (int, error) {
	cfg := &container.Config{
		Image: opts.Image,
		Cmd:   []string{"sh", "-c", opts.Command},
		Env:   mapToEnvSlice(opts.Env),
	}

	hostCfg := &container.HostConfig{
		Binds: opts.Binds,
	}

	resp, err := c.cli.ContainerCreate(ctx, cfg, hostCfg, nil, nil, "")
	if err != nil {
		return -1, fmt.Errorf("creating container: %w", err)
	}

	containerID := resp.ID

	// Always remove the container when we're done.
	defer func() {
		_ = c.cli.ContainerRemove(context.WithoutCancel(ctx), containerID, container.RemoveOptions{Force: true})
	}()

	// Attach to the container to stream stdout/stderr.
	attachResp, err := c.cli.ContainerAttach(ctx, containerID, container.AttachOptions{
		Stream: true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return -1, fmt.Errorf("attaching to container: %w", err)
	}
	defer attachResp.Close()

	// Start the container.
	if err := c.cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return -1, fmt.Errorf("starting container: %w", err)
	}

	// Stream logs using stdcopy to demux stdout/stderr.
	stdout := opts.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = io.Discard
	}

	streamDone := make(chan error, 1)
	go func() {
		_, err := stdcopy.StdCopy(stdout, stderr, attachResp.Reader)
		streamDone <- err
	}()

	// Wait for the container to exit.
	waitCh, errCh := c.cli.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)

	select {
	case result := <-waitCh:
		// Wait for streaming to finish.
		<-streamDone
		if result.Error != nil {
			return int(result.StatusCode), fmt.Errorf("container exited with error: %s", result.Error.Message)
		}
		return int(result.StatusCode), nil

	case err := <-errCh:
		// On context cancellation, kill the container.
		if ctx.Err() != nil {
			_ = c.cli.ContainerKill(context.WithoutCancel(ctx), containerID, "KILL")
		}
		return -1, fmt.Errorf("waiting for container: %w", err)
	}
}

// mapToEnvSlice converts map[string]string to []string{"KEY=VALUE"}.
func mapToEnvSlice(env map[string]string) []string {
	result := make([]string, 0, len(env))
	for k, v := range env {
		result = append(result, k+"="+v)
	}
	return result
}
