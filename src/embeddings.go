package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type EmbeddingProvider interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

func newEmbeddingProvider(cfg EmbeddingConfig) (EmbeddingProvider, error) {
	cfg.normalize()
	switch cfg.Provider {
	case "openai", "openai-compatible", "openrouter", "mistral", "lmstudio", "llamacpp":
		return newOpenAICompatibleProvider(cfg), nil
	case "gemini":
		return newGeminiProvider(cfg), nil
	case "ollama":
		return newOllamaProvider(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported embedding provider %q", cfg.Provider)
	}
}

type simpleRateLimiter struct {
	interval time.Duration
	mu       sync.Mutex
	lastReq  time.Time
}

func newSimpleRateLimiter(requestsPerSecond int) *simpleRateLimiter {
	if requestsPerSecond <= 0 {
		return nil
	}
	return &simpleRateLimiter{
		interval: time.Second / time.Duration(requestsPerSecond),
	}
}

func (r *simpleRateLimiter) wait(ctx context.Context) error {
	if r == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	elapsed := time.Since(r.lastReq)
	if elapsed < r.interval {
		select {
		case <-time.After(r.interval - elapsed):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	r.lastReq = time.Now()
	return nil
}

type retryConfig struct {
	maxRetries     int
	initialDelay   time.Duration
	maxDelay       time.Duration
	currentAttempt int
	currentDelay   time.Duration
}

func newRetryConfig(cfg EmbeddingConfig) *retryConfig {
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		return nil
	}
	initialDelay, _ := time.ParseDuration(cfg.RetryInitialDelay)
	if initialDelay == 0 {
		initialDelay = 1 * time.Second
	}
	maxDelay, _ := time.ParseDuration(cfg.RetryMaxDelay)
	if maxDelay == 0 {
		maxDelay = 30 * time.Second
	}
	return &retryConfig{
		maxRetries:   maxRetries,
		initialDelay: initialDelay,
		maxDelay:     maxDelay,
		currentDelay: initialDelay,
	}
}

func (r *retryConfig) shouldRetry(err error) bool {
	if r == nil {
		return false
	}
	if r.currentAttempt >= r.maxRetries {
		return false
	}
	return isRetryableError(err)
}

func (r *retryConfig) nextDelay() time.Duration {
	if r == nil {
		return 0
	}
	delay := r.currentDelay
	r.currentDelay = r.currentDelay * 2
	if r.currentDelay > r.maxDelay {
		r.currentDelay = r.maxDelay
	}
	r.currentAttempt++
	return delay
}

func (r *retryConfig) reset() {
	if r != nil {
		r.currentAttempt = 0
		r.currentDelay = r.initialDelay
	}
}

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return false
	}
	errStr := err.Error()
	if strings.Contains(errStr, "rate limit") {
		return true
	}
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
		return true
	}
	if strings.Contains(errStr, "connection refused") || strings.Contains(errStr, "connection reset") {
		return true
	}
	if strings.Contains(errStr, "temporary") || strings.Contains(errStr, "transient") {
		return true
	}
	if strings.Contains(errStr, "API error") {
		if strings.Contains(errStr, "503") || strings.Contains(errStr, "502") || strings.Contains(errStr, "429") {
			return true
		}
	}
	return false
}

func retryWithBackoff(ctx context.Context, retryCfg *retryConfig, fn func() error) error {
	retryCfg.reset()
	for {
		err := fn()
		if err == nil {
			return nil
		}
		if !retryCfg.shouldRetry(err) {
			return err
		}
		delay := retryCfg.nextDelay()
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

type openAICompatibleProvider struct {
	baseURL  string
	model    string
	apiKey   string
	headers  map[string]string
	client   *http.Client
	limiter  *simpleRateLimiter
	retryCfg *retryConfig
}

func newOpenAICompatibleProvider(cfg EmbeddingConfig) *openAICompatibleProvider {
	timeout, _ := time.ParseDuration(cfg.Timeout)
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	return &openAICompatibleProvider{
		baseURL:  strings.TrimRight(cfg.BaseURL, "/"),
		model:    cfg.Model,
		apiKey:   apiKey(cfg),
		headers:  cfg.Headers,
		client:   &http.Client{Timeout: timeout},
		limiter:  newSimpleRateLimiter(cfg.RateLimit),
		retryCfg: newRetryConfig(cfg),
	}
}

func (p *openAICompatibleProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	if p.baseURL == "" {
		return nil, errors.New("embedding base_url is required")
	}
	var vecs [][]float32
	var lastErr error
	err := retryWithBackoff(ctx, p.retryCfg, func() error {
		if err := p.limiter.wait(ctx); err != nil {
			return fmt.Errorf("rate limit: %w", err)
		}
		vecs, lastErr = p.doRequest(ctx, texts)
		return lastErr
	})
	if err != nil {
		return nil, err
	}
	return vecs, nil
}

func (p *openAICompatibleProvider) doRequest(ctx context.Context, texts []string) ([][]float32, error) {
	body := map[string]any{"model": p.model, "input": texts}
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/embeddings", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
	for k, v := range p.headers {
		req.Header.Set(k, v)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("embedding API error: %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	var out struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Data) != len(texts) {
		return nil, fmt.Errorf("embedding response count mismatch: got %d want %d", len(out.Data), len(texts))
	}
	vecs := make([][]float32, len(out.Data))
	for i := range out.Data {
		vecs[i] = normalizeVector(out.Data[i].Embedding)
	}
	return vecs, nil
}

type geminiProvider struct {
	baseURL  string
	model    string
	apiKey   string
	client   *http.Client
	limiter  *simpleRateLimiter
	retryCfg *retryConfig
}

func newGeminiProvider(cfg EmbeddingConfig) *geminiProvider {
	timeout, _ := time.ParseDuration(cfg.Timeout)
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	return &geminiProvider{
		baseURL:  strings.TrimRight(cfg.BaseURL, "/"),
		model:    cfg.Model,
		apiKey:   apiKey(cfg),
		client:   &http.Client{Timeout: timeout},
		limiter:  newSimpleRateLimiter(cfg.RateLimit),
		retryCfg: newRetryConfig(cfg),
	}
}

func (p *geminiProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	if p.baseURL == "" {
		return nil, errors.New("embedding base_url is required")
	}
	vecs := make([][]float32, 0, len(texts))
	for _, text := range texts {
		var vec []float32
		var lastErr error
		err := retryWithBackoff(ctx, p.retryCfg, func() error {
			if err := p.limiter.wait(ctx); err != nil {
				return fmt.Errorf("rate limit: %w", err)
			}
			vec, lastErr = p.doRequest(ctx, text)
			return lastErr
		})
		if err != nil {
			return nil, err
		}
		vecs = append(vecs, vec)
	}
	return vecs, nil
}

func (p *geminiProvider) doRequest(ctx context.Context, text string) ([]float32, error) {
	body := map[string]any{
		"content": map[string]any{
			"parts": []map[string]string{{"text": text}},
		},
	}
	data, _ := json.Marshal(body)
	endpoint, err := url.JoinPath(p.baseURL, "models", p.model+":embedContent")
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("x-goog-api-key", p.apiKey)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("gemini embedding error: %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	var out struct {
		Embedding struct {
			Values []float32 `json:"values"`
		} `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return normalizeVector(out.Embedding.Values), nil
}

type ollamaProvider struct {
	baseURL  string
	model    string
	client   *http.Client
	limiter  *simpleRateLimiter
	retryCfg *retryConfig
}

func newOllamaProvider(cfg EmbeddingConfig) *ollamaProvider {
	timeout, _ := time.ParseDuration(cfg.Timeout)
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	return &ollamaProvider{
		baseURL:  strings.TrimRight(cfg.BaseURL, "/"),
		model:    cfg.Model,
		client:   &http.Client{Timeout: timeout},
		limiter:  newSimpleRateLimiter(cfg.RateLimit),
		retryCfg: newRetryConfig(cfg),
	}
}

func (p *ollamaProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	if p.baseURL == "" {
		return nil, errors.New("embedding base_url is required")
	}
	vecs := make([][]float32, 0, len(texts))
	for _, text := range texts {
		var vec []float32
		var lastErr error
		err := retryWithBackoff(ctx, p.retryCfg, func() error {
			if err := p.limiter.wait(ctx); err != nil {
				return fmt.Errorf("rate limit: %w", err)
			}
			vec, lastErr = p.doRequest(ctx, text)
			return lastErr
		})
		if err != nil {
			return nil, err
		}
		vecs = append(vecs, vec)
	}
	return vecs, nil
}

func (p *ollamaProvider) doRequest(ctx context.Context, text string) ([]float32, error) {
	body := map[string]any{"model": p.model, "prompt": text}
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/embeddings", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("ollama embedding error: %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	var out struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return normalizeVector(out.Embedding), nil
}

func normalizeVector(vec []float32) []float32 {
	var sum float64
	for _, v := range vec {
		sum += float64(v * v)
	}
	if sum == 0 {
		return append([]float32(nil), vec...)
	}
	inv := float32(1 / math.Sqrt(sum))
	out := make([]float32, len(vec))
	for i, v := range vec {
		out[i] = v * inv
	}
	return out
}
