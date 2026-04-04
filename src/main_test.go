package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestReadChunkContent(t *testing.T) {
	root := t.TempDir()
	content := "line1\nline2\nline3\nline4\nline5"
	if err := os.WriteFile(filepath.Join(root, "test.go"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		startLine int
		endLine   int
		want      string
	}{
		{"normal range", 1, 3, "line1\nline2\nline3"},
		{"single line", 2, 2, "line2"},
		{"full file", 1, 5, content},
		{"end beyond file", 1, 10, content},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := readChunkContent(root, "test.go", tt.startLine, tt.endLine)
			if got != tt.want {
				t.Fatalf("readChunkContent() = %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("missing file", func(t *testing.T) {
		got := readChunkContent(root, "missing.go", 1, 5)
		if !strings.Contains(got, "file unavailable") {
			t.Fatalf("expected error message, got %q", got)
		}
	})

	t.Run("invalid start line", func(t *testing.T) {
		got := readChunkContent(root, "test.go", 0, 5)
		if got != "[line range invalid]" {
			t.Fatalf("expected invalid range message, got %q", got)
		}
	})
}
