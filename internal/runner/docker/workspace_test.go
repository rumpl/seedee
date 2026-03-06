package docker

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestDockerRunnerConfig_Defaults(t *testing.T) {
	cfg := DockerRunnerConfig{}
	if cfg.SourceDir != "" {
		t.Fatalf("expected empty SourceDir, got %q", cfg.SourceDir)
	}
	if cfg.PipelineID != "" {
		t.Fatalf("expected empty PipelineID, got %q", cfg.PipelineID)
	}
}

func TestVolumeNameForJob_WithPipelineID(t *testing.T) {
	name := volumeNameForJob("abc123", "build")
	expected := "seedee-abc123-build"
	if name != expected {
		t.Fatalf("expected %q, got %q", expected, name)
	}
}

func TestVolumeNameForJob_WithoutPipelineID(t *testing.T) {
	name := volumeNameForJob("", "build")
	// Should have format: seedee-build-<hex>
	prefix := "seedee-build-"
	if len(name) <= len(prefix) {
		t.Fatalf("expected name longer than prefix %q, got %q", prefix, name)
	}
	if name[:len(prefix)] != prefix {
		t.Fatalf("expected prefix %q, got %q", prefix, name)
	}
}

func TestVolumeNameForJob_Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for range 100 {
		name := volumeNameForJob("", "build")
		if seen[name] {
			t.Fatalf("duplicate volume name %q", name)
		}
		seen[name] = true
	}
}

func TestCreateTar_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	reader, err := createTar(tmpDir)
	if err != nil {
		t.Fatalf("createTar: %v", err)
	}

	entries := tarEntries(t, reader)
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d: %v", len(entries), entries)
	}
}

func TestCreateTar_SingleFile(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "hello.txt"), "hello world")

	reader, err := createTar(tmpDir)
	if err != nil {
		t.Fatalf("createTar: %v", err)
	}

	entries := tarEntries(t, reader)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d: %v", len(entries), entries)
	}
	if entries[0] != "hello.txt" {
		t.Fatalf("expected entry 'hello.txt', got %q", entries[0])
	}
}

func TestCreateTar_Subdirectories(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "sub", "nested")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(tmpDir, "root.txt"), "root")
	writeFile(t, filepath.Join(subDir, "deep.txt"), "deep")

	reader, err := createTar(tmpDir)
	if err != nil {
		t.Fatalf("createTar: %v", err)
	}

	entries := tarEntries(t, reader)
	sort.Strings(entries)

	expected := []string{"root.txt", "sub", "sub/nested", "sub/nested/deep.txt"}
	if len(entries) != len(expected) {
		t.Fatalf("expected %d entries, got %d: %v", len(expected), len(entries), entries)
	}
	for i, e := range expected {
		if entries[i] != e {
			t.Errorf("entry %d: expected %q, got %q", i, e, entries[i])
		}
	}
}

func TestCreateTar_FileContent(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "data.txt"), "expected-content")

	reader, err := createTar(tmpDir)
	if err != nil {
		t.Fatalf("createTar: %v", err)
	}

	tr := tar.NewReader(reader)
	hdr, err := tr.Next()
	if err != nil {
		t.Fatalf("reading tar: %v", err)
	}
	if hdr.Name != "data.txt" {
		t.Fatalf("expected 'data.txt', got %q", hdr.Name)
	}

	content, err := io.ReadAll(tr)
	if err != nil {
		t.Fatalf("reading file content: %v", err)
	}
	if string(content) != "expected-content" {
		t.Fatalf("expected 'expected-content', got %q", string(content))
	}
}

func TestNewWorkspaceManager(t *testing.T) {
	wm := NewWorkspaceManager(nil)
	if wm == nil {
		t.Fatal("expected non-nil WorkspaceManager")
	}
}

// tarEntries reads all entries from a tar reader and returns their names.
func tarEntries(t *testing.T, r io.Reader) []string {
	t.Helper()

	tr := tar.NewReader(r)
	var names []string

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("reading tar entry: %v", err)
		}
		names = append(names, hdr.Name)
	}

	return names
}

// writeFile creates a file with the given content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}
