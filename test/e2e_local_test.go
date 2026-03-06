//go:build e2e

package test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedeeBin returns the path to the compiled seedee binary.
func seedeeBin(t *testing.T) string {
	t.Helper()
	bin := filepath.Join("..", "bin", "seedee")
	if _, err := os.Stat(bin); err != nil {
		t.Skip("seedee binary not found — run `make build` first")
	}
	return bin
}

func runSeedee(t *testing.T, dir string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(seedeeBin(t), args...)
	cmd.Dir = dir
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	exitCode = 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("unexpected error running seedee: %v", err)
	}
	return outBuf.String(), errBuf.String(), exitCode
}

func TestE2E_Local_SimpleSuccess(t *testing.T) {
	stdout, _, exitCode := runSeedee(t, "../testdata/simple", "run")
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "Hello from seedee!")
	assert.Contains(t, stdout, "success")
}

func TestE2E_Local_ParallelJobs(t *testing.T) {
	start := time.Now()
	stdout, _, exitCode := runSeedee(t, "../testdata/parallel", "run")
	elapsed := time.Since(start)

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "Job A")
	assert.Contains(t, stdout, "Job B")
	// Both jobs sleep 2s. If parallel, total should be ~2-4s, not ~4+s
	assert.Less(t, elapsed, 6*time.Second, "parallel jobs should run concurrently")
}

func TestE2E_Local_Dependencies(t *testing.T) {
	stdout, _, exitCode := runSeedee(t, "../testdata/deps", "run")
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "built")
	assert.Contains(t, stdout, "testing")
	assert.Contains(t, stdout, "deploying")
}

func TestE2E_Local_Failure(t *testing.T) {
	stdout, _, exitCode := runSeedee(t, "../testdata/failure", "run")
	assert.NotEqual(t, 0, exitCode)
	assert.Contains(t, stdout, "this passes")           // pass job runs
	assert.Contains(t, stdout, "failed")                 // fail job fails
	assert.Contains(t, stdout, "skipped")                // skipped job is skipped
	assert.NotContains(t, stdout, "should not see this") // skipped step never ran
}

func TestE2E_Local_EnvironmentVariables(t *testing.T) {
	stdout, _, exitCode := runSeedee(t, "../testdata/env", "run")
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "GLOBAL=global")
	assert.Contains(t, stdout, "JOB=job-level")
	assert.Contains(t, stdout, "STEP=step-level")
}

func TestE2E_Local_ConfigNotFound(t *testing.T) {
	dir := t.TempDir() // empty dir, no .seedee.yml
	_, stderr, exitCode := runSeedee(t, dir, "run")
	assert.NotEqual(t, 0, exitCode)
	assert.Contains(t, stderr, "config file not found")
}

func TestE2E_Local_InvalidConfig(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".seedee.yml"), []byte("garbage: [[["), 0644))
	_, stderr, exitCode := runSeedee(t, dir, "run")
	assert.NotEqual(t, 0, exitCode)
	assert.True(t, strings.Contains(stderr, "error") || strings.Contains(stderr, "Error"),
		"expected error mention in stderr, got: %s", stderr)
}
