package docker

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestRunnerConfig_Defaults(t *testing.T) {
	cfg := RunnerConfig{}
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

func TestCreateTar_SkipsNodeModules(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "index.js"), "console.log('hi')")
	nm := filepath.Join(tmpDir, "node_modules", "pkg")
	if err := os.MkdirAll(nm, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(nm, "index.js"), "module.exports = {}")

	reader, err := createTar(tmpDir)
	if err != nil {
		t.Fatalf("createTar: %v", err)
	}

	entries := tarEntries(t, reader)
	for _, e := range entries {
		if e == "node_modules" || strings.HasPrefix(e, "node_modules/") {
			t.Fatalf("tar should not contain node_modules, found %q", e)
		}
	}
	if len(entries) != 1 || entries[0] != "index.js" {
		t.Fatalf("expected [index.js], got %v", entries)
	}
}

func TestCreateTar_SkipsGitDir(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "main.go"), "package main")
	gitDir := filepath.Join(tmpDir, ".git", "objects")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(gitDir, "abc"), "blob")

	reader, err := createTar(tmpDir)
	if err != nil {
		t.Fatalf("createTar: %v", err)
	}

	entries := tarEntries(t, reader)
	for _, e := range entries {
		if e == ".git" || strings.HasPrefix(e, ".git/") {
			t.Fatalf("tar should not contain .git, found %q", e)
		}
	}
}

func TestCreateTar_SkipsBinAndGen(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "main.go"), "package main")

	for _, dir := range []string{"bin", "gen"} {
		d := filepath.Join(tmpDir, dir)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(d, "output"), "data")
	}

	reader, err := createTar(tmpDir)
	if err != nil {
		t.Fatalf("createTar: %v", err)
	}

	entries := tarEntries(t, reader)
	sort.Strings(entries)

	// bin/ is still skipped, but gen/ is now included.
	expected := []string{"gen", "gen/output", "main.go"}
	if len(entries) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, entries)
	}
	for i, e := range expected {
		if entries[i] != e {
			t.Errorf("entry %d: expected %q, got %q", i, e, entries[i])
		}
	}
}

func TestCreateTar_SkipsNestedNodeModules(t *testing.T) {
	tmpDir := t.TempDir()
	front := filepath.Join(tmpDir, "frontend")
	nm := filepath.Join(front, "node_modules", "pkg")
	if err := os.MkdirAll(nm, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(front, "index.js"), "app")
	writeFile(t, filepath.Join(nm, "lib.js"), "lib")

	reader, err := createTar(tmpDir)
	if err != nil {
		t.Fatalf("createTar: %v", err)
	}

	entries := tarEntries(t, reader)
	sort.Strings(entries)

	expected := []string{"frontend", "frontend/index.js"}
	if len(entries) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, entries)
	}
	for i, e := range expected {
		if entries[i] != e {
			t.Errorf("entry %d: expected %q, got %q", i, e, entries[i])
		}
	}
}

func TestCreateTar_HandlesSymlinks(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "real.txt"), "content")

	// Create a symlink to a file.
	if err := os.Symlink(
		filepath.Join(tmpDir, "real.txt"),
		filepath.Join(tmpDir, "link.txt"),
	); err != nil {
		t.Fatal(err)
	}

	reader, err := createTar(tmpDir)
	if err != nil {
		t.Fatalf("createTar: %v", err)
	}

	entries := tarEntries(t, reader)
	sort.Strings(entries)

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %v", entries)
	}
}

func TestCreateTar_SkipsDirSymlinks(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a real directory with a file.
	realDir := filepath.Join(tmpDir, "realdir")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(realDir, "file.txt"), "data")

	// Create a symlink pointing to that directory.
	if err := os.Symlink(realDir, filepath.Join(tmpDir, "linkdir")); err != nil {
		t.Fatal(err)
	}

	reader, err := createTar(tmpDir)
	if err != nil {
		t.Fatalf("createTar: %v", err)
	}

	entries := tarEntries(t, reader)
	sort.Strings(entries)

	// linkdir should be skipped (directory symlink), but realdir and its
	// contents should be present.
	for _, e := range entries {
		if e == "linkdir" || strings.HasPrefix(e, "linkdir/") {
			t.Fatalf("tar should not contain directory symlink 'linkdir', found %q", e)
		}
	}

	expected := []string{"realdir", "realdir/file.txt"}
	if len(entries) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, entries)
	}
}

// writeFile creates a file with the given content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}
