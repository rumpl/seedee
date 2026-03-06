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
		_ = w.client.cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
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

// skipDirs is the set of directory names that should never be included in the
// injected source tar. These are either too large, contain symlink structures
// that break the tar walker (e.g. pnpm node_modules), or are generated
// artifacts that don't belong in the build context.
var skipDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	"bin":          true,
	"gen":          true,
}

// createTar creates a tar archive from a directory, skipping common
// directories that should not be injected (node_modules, .git, bin, gen)
// and properly handling symlinks and directories.
func createTar(srcDir string) (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("accessing %s: %w", path, err)
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

		// Skip directories that should never be injected.
		if info.IsDir() && skipDirs[info.Name()] {
			return filepath.SkipDir
		}

		// Handle symlinks: resolve the link target to decide what to do.
		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("reading symlink %s: %w", relPath, err)
			}

			// Stat the target to see if it's a directory.
			targetInfo, err := os.Stat(path)
			if err != nil {
				// Dangling symlink — skip it.
				return nil
			}

			if targetInfo.IsDir() {
				// Skip directory symlinks to avoid cycles and pnpm-style
				// structures that break the walker. We return nil rather
				// than filepath.SkipDir because Walk sees symlinks as
				// non-directory entries and SkipDir would skip remaining
				// siblings in the parent directory.
				return nil
			}

			// For file symlinks, record them as symlinks in the tar.
			header, err := tar.FileInfoHeader(info, linkTarget)
			if err != nil {
				return fmt.Errorf("creating tar header for symlink %s: %w", relPath, err)
			}
			header.Name = filepath.ToSlash(relPath)

			if err := tw.WriteHeader(header); err != nil {
				return fmt.Errorf("writing tar header for symlink %s: %w", relPath, err)
			}

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

		// Directories only need a header, no content.
		if info.IsDir() {
			return nil
		}

		// Only open and copy regular files.
		if !info.Mode().IsRegular() {
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
