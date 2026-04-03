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
	"time"
)

type EmbeddingProvider interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

func newEmbeddingProvider(cfg EmbeddingConfig) (EmbeddingProvider, error) {
	cfg.normalize()
	switch cfg.Provider {
	case "openai", "openai-compatible", "openrouter", "mistral", "lmstudio":
		return newOpenAICompatibleProvider(cfg), nil
	case "gemini":
		return newGeminiProvider(cfg), nil
	case "ollama":
		return newOllamaProvider(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported embedding provider %q", cfg.Provider)
	}
}

type openAICompatibleProvider struct {
	baseURL string
	model   string
	apiKey  string
	headers map[string]string
	client  *http.Client
}

func newOpenAICompatibleProvider(cfg EmbeddingConfig) *openAICompatibleProvider {
	return &openAICompatibleProvider{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		model:   cfg.Model,
		apiKey:  apiKey(cfg),
		headers: cfg.Headers,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (p *openAICompatibleProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	if p.baseURL == "" {
		return nil, errors.New("embedding base_url is required")
	}
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
	baseURL string
	model   string
	apiKey  string
	client  *http.Client
}

func newGeminiProvider(cfg EmbeddingConfig) *geminiProvider {
	return &geminiProvider{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		model:   cfg.Model,
		apiKey:  apiKey(cfg),
		client:  &http.Client{Timeout: 60 * time.Second},
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
		vecs = append(vecs, normalizeVector(out.Embedding.Values))
	}
	return vecs, nil
}

type ollamaProvider struct {
	baseURL string
	model   string
	client  *http.Client
}

func newOllamaProvider(cfg EmbeddingConfig) *ollamaProvider {
	return &ollamaProvider{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		model:   cfg.Model,
		client:  &http.Client{Timeout: 60 * time.Second},
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
		vecs = append(vecs, normalizeVector(out.Embedding))
	}
	return vecs, nil
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
