package docker

import (
	"sort"
	"testing"
)

func TestMapToEnvSlice_Empty(t *testing.T) {
	result := mapToEnvSlice(map[string]string{})
	if len(result) != 0 {
		t.Fatalf("expected empty slice, got %v", result)
	}
}

func TestMapToEnvSlice_Nil(t *testing.T) {
	result := mapToEnvSlice(nil)
	if len(result) != 0 {
		t.Fatalf("expected empty slice, got %v", result)
	}
}

func TestMapToEnvSlice_SingleEntry(t *testing.T) {
	result := mapToEnvSlice(map[string]string{"FOO": "bar"})
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	if result[0] != "FOO=bar" {
		t.Fatalf("expected FOO=bar, got %s", result[0])
	}
}

func TestMapToEnvSlice_MultipleEntries(t *testing.T) {
	env := map[string]string{
		"A": "1",
		"B": "2",
		"C": "3",
	}
	result := mapToEnvSlice(env)
	if len(result) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(result))
	}

	// Sort for deterministic comparison since map iteration is unordered.
	sort.Strings(result)

	expected := []string{"A=1", "B=2", "C=3"}
	for i, v := range expected {
		if result[i] != v {
			t.Errorf("entry %d: expected %s, got %s", i, v, result[i])
		}
	}
}

func TestMapToEnvSlice_EmptyValue(t *testing.T) {
	result := mapToEnvSlice(map[string]string{"KEY": ""})
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	if result[0] != "KEY=" {
		t.Fatalf("expected KEY=, got %s", result[0])
	}
}

func TestMapToEnvSlice_SpecialCharacters(t *testing.T) {
	env := map[string]string{
		"PATH":    "/usr/bin:/usr/local/bin",
		"MESSAGE": "hello world",
	}
	result := mapToEnvSlice(env)
	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}

	sort.Strings(result)

	expected := []string{"MESSAGE=hello world", "PATH=/usr/bin:/usr/local/bin"}
	for i, v := range expected {
		if result[i] != v {
			t.Errorf("entry %d: expected %s, got %s", i, v, result[i])
		}
	}
}
