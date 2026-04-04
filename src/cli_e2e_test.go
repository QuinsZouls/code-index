package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCLIE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skip e2e in short mode")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if r.URL.Path != "/embeddings" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		input := body["input"]
		texts, ok := input.([]any)
		if !ok || len(texts) == 0 {
			t.Fatalf("input = %#v", input)
		}
		resp := map[string]any{"data": make([]map[string]any, len(texts))}
		for i, raw := range texts {
			text, _ := raw.(string)
			vec := []float32{0}
			if strings.Contains(text, "hello") {
				vec = []float32{1, 0}
			}
			resp["data"].([]map[string]any)[i] = map[string]any{"embedding": vec}
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "hello.go"), []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("ignore"), 0o644); err != nil {
		t.Fatal(err)
	}

	bin := filepath.Join(t.TempDir(), "codeindex-test-bin")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	build := exec.Command("go", "build", "-o", bin, "./src")
	build.Dir = projectRoot(t)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}

	run := func(args ...string) string {
		cmd := exec.Command(bin, args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "OPENAI_API_KEY=test", "OPENAI_BASE_URL="+server.URL)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s failed: %v\n%s", strings.Join(args, " "), err, out)
		}
		return string(out)
	}

	if out := run("init", "-path", root); !strings.Contains(out, "initialized:") {
		t.Fatalf("init output = %q", out)
	}
	settings := filepath.Join(root, settingsDirName, settingsFileName)
	data, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	cfg.Embedding = EmbeddingConfig{Provider: "openai-compatible", Model: "fake", BaseURL: server.URL, APIKey: "test"}
	if err := saveConfig(root, cfg); err != nil {
		t.Fatal(err)
	}

	if out := run("index", "-path", root); !strings.Contains(out, "indexed 1 files") {
		t.Fatalf("index output = %q", out)
	}
	if out := run("index", "-path", root, "--verbose"); !strings.Contains(out, "[=] hello.go (unchanged)") {
		t.Fatalf("verbose output = %q", out)
	}
	if out := run("search", "-path", root, "hello"); !strings.Contains(out, "hello.go") {
		t.Fatalf("search output = %q", out)
	}
	if out := run("search", "-path", root, "-files", "hello"); !strings.Contains(out, "hello.go") {
		t.Fatalf("search -files output = %q", out)
	}
	if out := run("search", "-path", root, "-files", "hello"); strings.Contains(out, "--- Result") {
		t.Fatalf("search -files should not show content blocks: %q", out)
	}
	if out := run("status", "-path", root); !strings.Contains(out, "Files: 1") || !strings.Contains(out, "Chunks: 1") {
		t.Fatalf("status output = %q", out)
	}

}

func projectRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Dir(wd)
}
