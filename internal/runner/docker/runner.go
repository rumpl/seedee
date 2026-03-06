package docker

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"

	"github.com/rumpl/seedee/internal/core"
	"github.com/rumpl/seedee/internal/runner"
)

// Compile-time check that DockerRunner implements runner.Runner.
var _ runner.Runner = (*DockerRunner)(nil)

// DockerRunner implements runner.Runner using Docker containers.
// Each job gets a shared workspace volume that persists across steps.
type DockerRunner struct {
	client  *Client
	volumes map[string]string // jobName -> volumeName
}

// NewDockerRunner creates a new DockerRunner backed by the given Docker client.
func NewDockerRunner(client *Client) *DockerRunner {
	return &DockerRunner{
		client:  client,
		volumes: make(map[string]string),
	}
}

// Setup pulls the job's image and creates a workspace volume for the job.
func (r *DockerRunner) Setup(ctx context.Context, job *core.Job) error {
	if err := r.client.PullImage(ctx, job.Image, io.Discard); err != nil {
		return fmt.Errorf("pulling image %s: %w", job.Image, err)
	}

	volName := fmt.Sprintf("seedee-%s-%s", job.Name, randomSuffix())
	if err := r.client.CreateVolume(ctx, volName); err != nil {
		return fmt.Errorf("creating volume %s: %w", volName, err)
	}

	r.volumes[job.Name] = volName

	return nil
}

// RunStep runs a single step inside a Docker container with the job's workspace
// volume mounted at /workspace.
func (r *DockerRunner) RunStep(ctx context.Context, job *core.Job, step *core.Step, stdout, stderr io.Writer) (*core.StepResult, error) {
	volName, ok := r.volumes[job.Name]
	if !ok {
		return nil, fmt.Errorf("no volume found for job %s; was Setup called?", job.Name)
	}

	exitCode, err := r.client.RunContainer(ctx, RunOptions{
		Image:   job.Image,
		Command: step.Command,
		Env:     step.Env,
		Binds:   []string{volName + ":/workspace"},
		Stdout:  stdout,
		Stderr:  stderr,
	})
	if err != nil {
		return nil, fmt.Errorf("running step %s: %w", step.Name, err)
	}

	return &core.StepResult{
		ExitCode: exitCode,
	}, nil
}

// Teardown removes the workspace volume for the job.
func (r *DockerRunner) Teardown(ctx context.Context, job *core.Job) error {
	volName, ok := r.volumes[job.Name]
	if !ok {
		return nil
	}

	if err := r.client.RemoveVolume(ctx, volName); err != nil {
		return fmt.Errorf("removing volume %s: %w", volName, err)
	}

	delete(r.volumes, job.Name)

	return nil
}

// randomSuffix returns a short random hex string for unique naming.
func randomSuffix() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}
