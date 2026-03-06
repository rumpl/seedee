package docker

import (
	"context"
	"testing"
)

func TestCleanupRegistry_RegisterVolume(t *testing.T) {
	r := NewCleanupRegistry()
	r.RegisterVolume("vol1")
	r.RegisterVolume("vol2")

	vols := r.Volumes()
	if len(vols) != 2 {
		t.Fatalf("expected 2 volumes, got %d", len(vols))
	}
	if vols[0] != "vol1" || vols[1] != "vol2" {
		t.Fatalf("unexpected volumes: %v", vols)
	}
}

func TestCleanupRegistry_VolumesReturnsCopy(t *testing.T) {
	r := NewCleanupRegistry()
	r.RegisterVolume("vol1")

	vols := r.Volumes()
	vols[0] = "modified"

	original := r.Volumes()
	if original[0] != "vol1" {
		t.Fatalf("Volumes() should return a copy, but original was modified")
	}
}

func TestCleanupRegistry_CleanupAllWithNilClient(t *testing.T) {
	r := NewCleanupRegistry()
	// Empty registry should not error even with nil context.
	err := r.CleanupAll(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected no error for empty registry, got %v", err)
	}
}

func TestCleanupRegistry_Empty(t *testing.T) {
	r := NewCleanupRegistry()
	vols := r.Volumes()
	if len(vols) != 0 {
		t.Fatalf("expected 0 volumes, got %d", len(vols))
	}
}
