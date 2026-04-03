package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSplitCSV(t *testing.T) {
	got := splitCSV(" a, b ,, c ")
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitCSV() = %#v, want %#v", got, want)
	}
}

func TestFindProjectRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, settingsDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath(root), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "nested", "deeper")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := findProjectRoot(nested)
	if err != nil {
		t.Fatal(err)
	}
	if got != root {
		t.Fatalf("findProjectRoot() = %q, want %q", got, root)
	}
}

func TestFindProjectRootReturnsStartWhenMissing(t *testing.T) {
	start := t.TempDir()
	got, err := findProjectRoot(start)
	if err != nil {
		t.Fatal(err)
	}
	if got != start {
		t.Fatalf("findProjectRoot() = %q, want %q", got, start)
	}
}
