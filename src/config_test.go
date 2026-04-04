package main

import (
	"encoding/json"
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
	if cfg.WorkerCount != 0 || cfg.CheckpointEvery != 0 {
		t.Fatalf("unexpected defaults: %#v", cfg)
	}
}

func TestConfigSupportsIndexerTuning(t *testing.T) {
	cfg := defaultConfig()
	cfg.WorkerCount = 12
	cfg.CheckpointEvery = 25
	cfg.normalize()
	if cfg.WorkerCount != 12 || cfg.CheckpointEvery != 25 {
		t.Fatalf("tuning values lost: %#v", cfg)
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
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, settingsDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	defaultCfg := defaultConfig()
	defaultCfg.Embedding.Model = "custom-model"
	data, err := json.MarshalIndent(defaultCfg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, settingsDirName, "default_settings.json"), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := initProject(root); err != nil {
		t.Fatal(err)
	}
	projectCfg, err := loadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if projectCfg.Embedding.Model != "custom-model" {
		t.Fatalf("expected init to copy user defaults, got %#v", projectCfg.Embedding)
	}
	gitignoreData, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(gitignoreData)
	if !strings.Contains(content, "/.codeindex/") {
		t.Fatalf("gitignore missing .codeindex entry: %q", content)
	}
}

func TestInitProjectIgnoresMissingUserDefaults(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	if _, err := initProject(root); err != nil {
		t.Fatal(err)
	}
	projectCfg, err := loadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if projectCfg.Embedding.Model != defaultConfig().Embedding.Model {
		t.Fatalf("unexpected default model: %#v", projectCfg.Embedding)
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

func TestEmbeddingConfigRateLimit(t *testing.T) {
	cfg := EmbeddingConfig{RateLimit: 10}
	cfg.normalize()
	if cfg.RateLimit != 10 {
		t.Fatalf("RateLimit = %d, want 10", cfg.RateLimit)
	}
}

func TestEmbeddingConfigTimeoutDefault(t *testing.T) {
	cfg := EmbeddingConfig{}
	cfg.normalize()
	if cfg.Timeout != "60s" {
		t.Fatalf("Timeout = %q, want 60s", cfg.Timeout)
	}
}

func TestEmbeddingConfigTimeoutCustom(t *testing.T) {
	cfg := EmbeddingConfig{Timeout: "30s"}
	cfg.normalize()
	if cfg.Timeout != "30s" {
		t.Fatalf("Timeout = %q, want 30s", cfg.Timeout)
	}
}

func TestConfigWithRateLimitRoundTrip(t *testing.T) {
	root := t.TempDir()
	cfg := defaultConfig()
	cfg.Embedding.RateLimit = 15
	cfg.Embedding.Timeout = "45s"
	if err := saveConfig(root, cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Embedding.RateLimit != 15 {
		t.Fatalf("RateLimit = %d, want 15", loaded.Embedding.RateLimit)
	}
	if loaded.Embedding.Timeout != "45s" {
		t.Fatalf("Timeout = %q, want 45s", loaded.Embedding.Timeout)
	}
}

func TestEmbeddingConfigRetryDefaults(t *testing.T) {
	cfg := EmbeddingConfig{}
	cfg.normalize()
	if cfg.MaxRetries != 0 {
		t.Fatalf("MaxRetries = %d, want 0 (disabled by default)", cfg.MaxRetries)
	}
	if cfg.RetryInitialDelay != "1s" {
		t.Fatalf("RetryInitialDelay = %q, want 1s", cfg.RetryInitialDelay)
	}
	if cfg.RetryMaxDelay != "30s" {
		t.Fatalf("RetryMaxDelay = %q, want 30s", cfg.RetryMaxDelay)
	}
}

func TestConfigWithRetryRoundTrip(t *testing.T) {
	root := t.TempDir()
	cfg := defaultConfig()
	cfg.Embedding.MaxRetries = 3
	cfg.Embedding.RetryInitialDelay = "2s"
	cfg.Embedding.RetryMaxDelay = "20s"
	if err := saveConfig(root, cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Embedding.MaxRetries != 3 {
		t.Fatalf("MaxRetries = %d, want 3", loaded.Embedding.MaxRetries)
	}
	if loaded.Embedding.RetryInitialDelay != "2s" {
		t.Fatalf("RetryInitialDelay = %q, want 2s", loaded.Embedding.RetryInitialDelay)
	}
	if loaded.Embedding.RetryMaxDelay != "20s" {
		t.Fatalf("RetryMaxDelay = %q, want 20s", loaded.Embedding.RetryMaxDelay)
	}
}
