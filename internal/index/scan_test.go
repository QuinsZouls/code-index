package index

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/QuinsZouls/code-index/internal/config"
)

func TestWalkFilesHonorsIncludesAndIgnores(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".codeindex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "ignored"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("ignored/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"main.go":                               "package main",
		filepath.Join("sub", "readme.md"):       "# readme",
		filepath.Join("ignored", "skip.go"):     "package main",
		filepath.Join(".codeindex", "local.go"): "package main",
		"notes.txt":                             "ignore me",
	}
	for rel, content := range files {
		abs := filepath.Join(root, rel)
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.Config{}
	cfg.Normalize()
	cfg.IncludePatterns = []string{"**/*.go", "**/*.md"}
	got, err := WalkFiles(root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"main.go", filepath.Join("sub", "readme.md")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WalkFiles() = %#v, want %#v", got, want)
	}
}

func TestWalkFilesHonorsNestedGitignore(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "repo", "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "repo", ".gitignore"), []byte("nested/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "repo", "keep.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "repo", "nested", "skip.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{}
	cfg.Normalize()
	got, err := WalkFiles(filepath.Join(root, "repo"), cfg)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"keep.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WalkFiles() = %#v, want %#v", got, want)
	}
}

func TestMatchExcludeHelper(t *testing.T) {
	cfg := config.Config{}
	cfg.Normalize()
	if !shouldExclude(filepath.Join("a", ".codeindex"), true, cfg, nil) {
		t.Fatal("expected .codeindex directory to be excluded")
	}
	if !shouldInclude("src/main.go", cfg) {
		t.Fatal("expected go file to be included")
	}
	if shouldInclude("notes.txt", cfg) {
		t.Fatal("expected txt file to be excluded by default")
	}
}
