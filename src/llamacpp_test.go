package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLlamacppConfigNormalization(t *testing.T) {
	cfg := EmbeddingConfig{Provider: "llamacpp"}
	cfg.normalize()

	if cfg.BaseURL != "http://localhost:8080/v1" {
		t.Errorf("expected BaseURL http://localhost:8080/v1, got %s", cfg.BaseURL)
	}
	if cfg.APIKeyEnv != "" {
		t.Errorf("expected empty APIKeyEnv, got %s", cfg.APIKeyEnv)
	}
}

func TestLlamacppConfigCustomURL(t *testing.T) {
	cfg := EmbeddingConfig{
		Provider: "llamacpp",
		BaseURL:  "http://custom-host:9999/v1",
	}
	cfg.normalize()

	if cfg.BaseURL != "http://custom-host:9999/v1" {
		t.Errorf("expected custom BaseURL, got %s", cfg.BaseURL)
	}
}

func TestLlamacppProviderCreation(t *testing.T) {
	cfg := EmbeddingConfig{
		Provider: "llamacpp",
		Model:    "test-model",
		BaseURL:  "http://localhost:8080/v1",
	}
	cfg.normalize()

	provider, err := newEmbeddingProvider(cfg)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	if provider == nil {
		t.Fatal("provider is nil")
	}

	_, ok := provider.(*openAICompatibleProvider)
	if !ok {
		t.Error("expected openAICompatibleProvider type")
	}
}

func TestLlamacppEmbeddingRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		if req["model"] != "test-model" {
			t.Errorf("expected model test-model, got %v", req["model"])
		}

		input, ok := req["input"].([]any)
		if !ok || len(input) != 2 {
			t.Errorf("expected input array with 2 elements, got %v", req["input"])
		}

		resp := map[string]any{
			"data": []map[string]any{
				{"embedding": []float32{0.1, 0.2, 0.3}},
				{"embedding": []float32{0.4, 0.5, 0.6}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := EmbeddingConfig{
		Provider: "llamacpp",
		Model:    "test-model",
		BaseURL:  server.URL + "/v1",
	}
	cfg.normalize()

	provider, err := newEmbeddingProvider(cfg)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	vecs, err := provider.Embed(context.Background(), []string{"hello", "world"})
	if err != nil {
		t.Fatalf("embedding failed: %v", err)
	}

	if len(vecs) != 2 {
		t.Errorf("expected 2 embeddings, got %d", len(vecs))
	}

	for i, vec := range vecs {
		if len(vec) != 3 {
			t.Errorf("embedding %d: expected length 3, got %d", i, len(vec))
		}
	}
}

func TestLlamacppEmbeddingError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	cfg := EmbeddingConfig{
		Provider: "llamacpp",
		Model:    "test-model",
		BaseURL:  server.URL + "/v1",
	}
	cfg.normalize()

	provider, err := newEmbeddingProvider(cfg)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	_, err = provider.Embed(context.Background(), []string{"test"})
	if err == nil {
		t.Error("expected error from failed request")
	}
}
