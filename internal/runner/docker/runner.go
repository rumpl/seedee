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

// DockerRunnerConfig configures the Docker runner.
type DockerRunnerConfig struct {
	// SourceDir is the directory to inject into the workspace volume.
	// If empty, no source injection happens (empty workspace).
	SourceDir string

	// PipelineID is used for generating unique volume names.
	// If empty, a random suffix is used instead.
	PipelineID string
}

// DockerRunner implements runner.Runner using Docker containers.
// Each job gets a shared workspace volume that persists across steps.
type DockerRunner struct {
	client    *Client
	config    DockerRunnerConfig
	workspace *WorkspaceManager
	cleanup   *CleanupRegistry
	volumes   map[string]string // jobName -> volumeName
}

// NewDockerRunner creates a new DockerRunner backed by the given Docker client.
func NewDockerRunner(client *Client) *DockerRunner {
	return NewDockerRunnerWithConfig(client, DockerRunnerConfig{})
}

// NewDockerRunnerWithConfig creates a new DockerRunner with the given configuration.
func NewDockerRunnerWithConfig(client *Client, config DockerRunnerConfig) *DockerRunner {
	return &DockerRunner{
		client:    client,
		config:    config,
		workspace: NewWorkspaceManager(client),
		cleanup:   NewCleanupRegistry(),
		volumes:   make(map[string]string),
	}
}

// Setup pulls the job's image and creates a workspace volume for the job.
// If SourceDir is configured, it injects source code into the volume.
func (r *DockerRunner) Setup(ctx context.Context, job *core.Job) error {
	if err := r.client.PullImage(ctx, job.Image, io.Discard); err != nil {
		return fmt.Errorf("pulling image %s: %w", job.Image, err)
	}

	volName := volumeNameForJob(r.config.PipelineID, job.Name)
	if err := r.client.CreateVolume(ctx, volName); err != nil {
		return fmt.Errorf("creating volume %s: %w", volName, err)
	}

	r.cleanup.RegisterVolume(volName)
	r.volumes[job.Name] = volName

	if r.config.SourceDir != "" {
		if err := r.workspace.InjectSource(ctx, volName, r.config.SourceDir); err != nil {
			return fmt.Errorf("injecting source into %s: %w", volName, err)
		}
	}

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

// CleanupAll removes all registered resources tracked by this runner.
// This is useful for cleaning up on fatal error.
func (r *DockerRunner) CleanupAll(ctx context.Context) error {
	return r.cleanup.CleanupAll(ctx, r.client)
}

// volumeNameForJob generates a unique volume name for a job in a pipeline run.
func volumeNameForJob(pipelineID, jobName string) string {
	if pipelineID != "" {
		return fmt.Sprintf("seedee-%s-%s", pipelineID, jobName)
	}
	return fmt.Sprintf("seedee-%s-%s", jobName, randomSuffix())
}

// randomSuffix returns a short random hex string for unique naming.
func randomSuffix() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}
