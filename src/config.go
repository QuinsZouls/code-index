package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	settingsDirName  = ".codeindex"
	settingsFileName = "settings.json"
	indexFileName    = "index.gob"
	configVersion    = 1
)

var defaultIncludePatterns = []string{
	"**/*.go",
	"**/*.rs",
	"**/*.py",
	"**/*.js",
	"**/*.jsx",
	"**/*.ts",
	"**/*.tsx",
	"**/*.java",
	"**/*.c",
	"**/*.h",
	"**/*.cpp",
	"**/*.hpp",
	"**/*.cs",
	"**/*.sh",
	"**/*.md",
	"**/*.yaml",
	"**/*.yml",
	"**/*.toml",
	"**/*.json",
	"**/*.sql",
	"**/*.html",
	"**/*.css",
}

var defaultExcludePatterns = []string{
	"**/.git",
	"**/.codeindex",
	"**/node_modules",
	"**/dist",
	"**/build",
	"**/target",
	"**/vendor",
	"**/__pycache__",
}

type Config struct {
	Version           int               `json:"version"`
	IncludePatterns   []string          `json:"include_patterns"`
	ExcludePatterns   []string          `json:"exclude_patterns"`
	LanguageOverrides map[string]string `json:"language_overrides,omitempty"`
	ChunkSize         int               `json:"chunk_size"`
	ChunkOverlap      int               `json:"chunk_overlap"`
	MinChunkSize      int               `json:"min_chunk_size"`
	ContextSize       int               `json:"context_size"`
	WorkerCount       int               `json:"worker_count,omitempty"`
	CheckpointEvery   int               `json:"checkpoint_every,omitempty"`
	SearchLimit       int               `json:"search_limit,omitempty"`
	ScoreThreshold    float64           `json:"score_threshold,omitempty"`
	HybridSearch      bool              `json:"hybrid_search,omitempty"`
	VectorWeight      float64           `json:"vector_weight,omitempty"`
	KeywordWeight     float64           `json:"keyword_weight,omitempty"`
	Embedding         EmbeddingConfig   `json:"embedding"`
}

type EmbeddingConfig struct {
	Provider          string            `json:"provider"`
	Model             string            `json:"model"`
	BaseURL           string            `json:"base_url,omitempty"`
	APIKey            string            `json:"api_key,omitempty"`
	APIKeyEnv         string            `json:"api_key_env,omitempty"`
	Headers           map[string]string `json:"headers,omitempty"`
	RateLimit         int               `json:"rate_limit,omitempty"`
	Timeout           string            `json:"timeout,omitempty"`
	MaxRetries        int               `json:"max_retries,omitempty"`
	RetryInitialDelay string            `json:"retry_initial_delay,omitempty"`
	RetryMaxDelay     string            `json:"retry_max_delay,omitempty"`
}

func defaultConfig() Config {
	return Config{
		Version:           configVersion,
		IncludePatterns:   append([]string{}, defaultIncludePatterns...),
		ExcludePatterns:   append([]string{}, defaultExcludePatterns...),
		LanguageOverrides: map[string]string{},
		ChunkSize:         120,
		ChunkOverlap:      20,
		MinChunkSize:      8,
		ContextSize:       0,
		WorkerCount:       0,
		CheckpointEvery:   0,
		SearchLimit:       5,
		ScoreThreshold:    0.3,
		HybridSearch:      false,
		VectorWeight:      0.7,
		KeywordWeight:     0.3,
		Embedding: EmbeddingConfig{
			Provider:  "openai",
			Model:     "text-embedding-3-small",
			APIKeyEnv: "OPENAI_API_KEY",
		},
	}
}

func (c *Config) normalize() {
	if c.Version == 0 {
		c.Version = configVersion
	}
	if len(c.IncludePatterns) == 0 {
		c.IncludePatterns = append([]string{}, defaultIncludePatterns...)
	}
	if len(c.ExcludePatterns) == 0 {
		c.ExcludePatterns = append([]string{}, defaultExcludePatterns...)
	}
	if c.LanguageOverrides == nil {
		c.LanguageOverrides = map[string]string{}
	}
	if c.ChunkSize <= 0 {
		c.ChunkSize = 120
	}
	if c.ChunkOverlap < 0 {
		c.ChunkOverlap = 20
	}
	if c.MinChunkSize <= 0 {
		c.MinChunkSize = 8
	}
	if c.ContextSize < 0 {
		c.ContextSize = 0
	}
	if c.WorkerCount < 0 {
		c.WorkerCount = 0
	}
	if c.CheckpointEvery < 0 {
		c.CheckpointEvery = 0
	}
	if c.SearchLimit <= 0 {
		c.SearchLimit = 5
	}
	if c.ScoreThreshold < 0 || c.ScoreThreshold > 1 {
		c.ScoreThreshold = 0.3
	}
	if c.VectorWeight < 0 {
		c.VectorWeight = 0.7
	}
	if c.KeywordWeight < 0 {
		c.KeywordWeight = 0.3
	}
	if c.VectorWeight+c.KeywordWeight > 0 {
		total := c.VectorWeight + c.KeywordWeight
		c.VectorWeight /= total
		c.KeywordWeight /= total
	} else {
		c.VectorWeight = 0.7
		c.KeywordWeight = 0.3
	}
	c.Embedding.normalize()
}

func (e *EmbeddingConfig) normalize() {
	e.Provider = strings.ToLower(strings.TrimSpace(e.Provider))
	if e.Provider == "" {
		e.Provider = "openai"
	}
	if e.Model == "" {
		e.Model = "text-embedding-3-small"
	}
	if e.Headers == nil {
		e.Headers = map[string]string{}
	}
	if e.APIKeyEnv == "" {
		switch e.Provider {
		case "openrouter":
			e.APIKeyEnv = "OPENROUTER_API_KEY"
		case "mistral":
			e.APIKeyEnv = "MISTRAL_API_KEY"
		case "gemini":
			e.APIKeyEnv = "GEMINI_API_KEY"
		case "ollama", "lmstudio", "llamacpp":
			e.APIKeyEnv = ""
		default:
			e.APIKeyEnv = "OPENAI_API_KEY"
		}
	}
	if e.BaseURL == "" {
		switch e.Provider {
		case "openai", "openai-compatible":
			e.BaseURL = "https://api.openai.com/v1"
		case "openrouter":
			e.BaseURL = "https://openrouter.ai/api/v1"
		case "mistral":
			e.BaseURL = "https://api.mistral.ai/v1"
		case "gemini":
			e.BaseURL = "https://generativelanguage.googleapis.com/v1beta"
		case "ollama":
			e.BaseURL = "http://localhost:11434"
		case "lmstudio":
			e.BaseURL = "http://localhost:1234/v1"
		case "llamacpp":
			e.BaseURL = "http://localhost:8080/v1"
		}
	}
	if e.Timeout == "" {
		e.Timeout = "60s"
	}
	if e.MaxRetries < 0 {
		e.MaxRetries = 0
	}
	if e.RetryInitialDelay == "" {
		e.RetryInitialDelay = "1s"
	}
	if e.RetryMaxDelay == "" {
		e.RetryMaxDelay = "30s"
	}
}

func loadConfig(projectRoot string) (Config, error) {
	path := settingsPath(projectRoot)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg := defaultConfig()
			return cfg, nil
		}
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	cfg.normalize()
	return cfg, nil
}

func saveConfig(projectRoot string, cfg Config) error {
	cfg.normalize()
	if err := os.MkdirAll(settingsDir(projectRoot), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(settingsPath(projectRoot), data, 0o644)
}

func initProject(projectRoot string) (Config, error) {
	cfg, _ := loadUserDefaultConfig()
	if err := saveConfig(projectRoot, cfg); err != nil {
		return Config{}, err
	}
	if err := ensureGitignore(projectRoot); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func loadUserDefaultConfig() (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		cfg := defaultConfig()
		return cfg, nil
	}
	path := filepath.Join(home, settingsDirName, "default_settings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg := defaultConfig()
			return cfg, nil
		}
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse user default config %s: %w", path, err)
	}
	cfg.normalize()
	return cfg, nil
}

func saveUserDefaultConfig(cfg Config) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, settingsDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, "default_settings.json")
	cfg.normalize()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func userDefaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, settingsDirName, "default_settings.json"), nil
}

func settingsDir(projectRoot string) string {
	return filepath.Join(projectRoot, settingsDirName)
}

func settingsPath(projectRoot string) string {
	return filepath.Join(settingsDir(projectRoot), settingsFileName)
}

func indexPath(projectRoot string) string {
	return filepath.Join(settingsDir(projectRoot), indexFileName)
}

func ensureGitignore(projectRoot string) error {
	if _, err := os.Stat(filepath.Join(projectRoot, ".git")); err != nil {
		return nil
	}
	path := filepath.Join(projectRoot, ".gitignore")
	entry := "/" + settingsDirName + "/"
	content, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == entry {
			return nil
		}
	}
	if len(content) > 0 && !strings.HasSuffix(string(content), "\n") {
		content = append(content, '\n')
	}
	content = append(content, []byte("# cocoindex-code\n")...)
	content = append(content, []byte(entry+"\n")...)
	return os.WriteFile(path, content, 0o644)
}

func apiKey(cfg EmbeddingConfig) string {
	if cfg.APIKey != "" {
		return cfg.APIKey
	}
	if cfg.APIKeyEnv != "" {
		return os.Getenv(cfg.APIKeyEnv)
	}
	switch cfg.Provider {
	case "openai", "openai-compatible":
		return os.Getenv("OPENAI_API_KEY")
	case "openrouter":
		return os.Getenv("OPENROUTER_API_KEY")
	case "mistral":
		return os.Getenv("MISTRAL_API_KEY")
	case "gemini":
		return os.Getenv("GEMINI_API_KEY")
	default:
		return ""
	}
}

func (c Config) embeddingSignature() string {
	b, _ := json.Marshal(c.Embedding)
	return string(b)
}
