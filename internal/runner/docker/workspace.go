package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types/container"
)

// WorkspaceManager handles workspace volume lifecycle.
type WorkspaceManager struct {
	client *Client
}

// NewWorkspaceManager creates a new WorkspaceManager.
func NewWorkspaceManager(client *Client) *WorkspaceManager {
	return &WorkspaceManager{client: client}
}

// InjectSource copies the contents of srcDir into the workspace volume
// by running a helper container that tars the content in.
func (w *WorkspaceManager) InjectSource(ctx context.Context, volumeName, srcDir string) error {
	// Create helper container with the volume mounted.
	resp, err := w.client.cli.ContainerCreate(ctx, &container.Config{
		Image: "alpine:latest",
		Cmd:   []string{"true"},
	}, &container.HostConfig{
		Binds: []string{volumeName + ":/workspace"},
	}, nil, nil, "")
	if err != nil {
		return fmt.Errorf("creating helper container: %w", err)
	}
	defer func() {
		_ = w.client.cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true}) //nolint:errcheck // best-effort cleanup of helper container
	}()

	// Create tar of srcDir.
	tarReader, err := createTar(srcDir)
	if err != nil {
		return fmt.Errorf("creating tar of %s: %w", srcDir, err)
	}

	// Copy into container at /workspace.
	err = w.client.cli.CopyToContainer(ctx, resp.ID, "/workspace", tarReader, container.CopyToContainerOptions{})
	if err != nil {
		return fmt.Errorf("copying source to workspace: %w", err)
	}

	return nil
}

// createTar creates a tar archive from a directory.
func createTar(srcDir string) (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get path relative to srcDir.
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return fmt.Errorf("computing relative path: %w", err)
		}

		// Skip the root directory itself.
		if relPath == "." {
			return nil
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("creating tar header for %s: %w", relPath, err)
		}
		header.Name = filepath.ToSlash(relPath)

		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("writing tar header for %s: %w", relPath, err)
		}

		if info.IsDir() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("opening %s: %w", path, err)
		}

		_, copyErr := io.Copy(tw, f)

		if err := f.Close(); err != nil {
			return fmt.Errorf("closing %s: %w", path, err)
		}

		if copyErr != nil {
			return fmt.Errorf("writing %s to tar: %w", relPath, copyErr)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking %s: %w", srcDir, err)
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("closing tar writer: %w", err)
	}

	return &buf, nil
}
