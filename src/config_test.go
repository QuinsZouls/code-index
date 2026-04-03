package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDefaultConfigNormalize(t *testing.T) {
	cfg := defaultConfig()
	cfg.normalize()
	if cfg.Embedding.Provider != "openai" {
		t.Fatalf("provider = %q, want openai", cfg.Embedding.Provider)
	}
	if cfg.Embedding.BaseURL == "" {
		t.Fatal("expected default base_url")
	}
	if len(cfg.IncludePatterns) == 0 || len(cfg.ExcludePatterns) == 0 {
		t.Fatal("expected default patterns")
	}
	if !containsString(cfg.ExcludePatterns, "**/.codeindex") {
		t.Fatal("expected .codeindex to be excluded")
	}
}

func TestSaveLoadConfigRoundTrip(t *testing.T) {
	root := t.TempDir()
	cfg := defaultConfig()
	cfg.IncludePatterns = []string{"**/*.go"}
	cfg.ExcludePatterns = []string{"**/tmp"}
	cfg.LanguageOverrides = map[string]string{"inc": "php"}
	cfg.Embedding = EmbeddingConfig{Provider: "openai-compatible", Model: "embed-1", BaseURL: "https://example.com/v1", APIKeyEnv: "MY_KEY"}
	if err := saveConfig(root, cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded.IncludePatterns, cfg.IncludePatterns) {
		t.Fatalf("IncludePatterns = %#v, want %#v", loaded.IncludePatterns, cfg.IncludePatterns)
	}
	if !reflect.DeepEqual(loaded.ExcludePatterns, cfg.ExcludePatterns) {
		t.Fatalf("ExcludePatterns = %#v, want %#v", loaded.ExcludePatterns, cfg.ExcludePatterns)
	}
	if !reflect.DeepEqual(loaded.LanguageOverrides, cfg.LanguageOverrides) {
		t.Fatalf("LanguageOverrides = %#v, want %#v", loaded.LanguageOverrides, cfg.LanguageOverrides)
	}
	if loaded.Embedding.Provider != cfg.Embedding.Provider || loaded.Embedding.Model != cfg.Embedding.Model || loaded.Embedding.BaseURL != cfg.Embedding.BaseURL {
		t.Fatalf("Embedding = %#v, want %#v", loaded.Embedding, cfg.Embedding)
	}
}

func TestInitProjectCreatesGitignore(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := initProject(root); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "/.codeindex/") {
		t.Fatalf("gitignore missing .codeindex entry: %q", content)
	}
}

func TestAPIKeyResolution(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "from-env")
	if got := apiKey(EmbeddingConfig{Provider: "openai"}); got != "from-env" {
		t.Fatalf("apiKey() = %q, want from-env", got)
	}
	if got := apiKey(EmbeddingConfig{Provider: "openai", APIKey: "explicit"}); got != "explicit" {
		t.Fatalf("apiKey() = %q, want explicit", got)
	}
}
