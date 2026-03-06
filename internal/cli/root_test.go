package cli

import (
	"bytes"
	"testing"
)

func TestRootCmd_Help(t *testing.T) {
	root := NewRootCmd()
	root.SetArgs([]string{"--help"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRootCmd_Version(t *testing.T) {
	root := NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"version"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("seedee")) {
		t.Errorf("expected output to contain 'seedee', got: %s", buf.String())
	}
}

func TestRunCmd_NotImplemented(t *testing.T) {
	root := NewRootCmd()
	root.SetArgs([]string{"run"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestStatusCmd_RequiresArg(t *testing.T) {
	root := NewRootCmd()
	root.SetArgs([]string{"status"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing arg")
	}
}

func TestCancelCmd_RequiresArg(t *testing.T) {
	root := NewRootCmd()
	root.SetArgs([]string{"cancel"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing arg")
	}
}
