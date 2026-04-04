package main

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func approxEqual(a, b float32) bool {
	return math.Abs(float64(a-b)) < 1e-4
}

func TestNormalizeVector(t *testing.T) {
	got := normalizeVector([]float32{3, 4})
	if len(got) != 2 || !approxEqual(got[0], 0.6) || !approxEqual(got[1], 0.8) {
		t.Fatalf("normalizeVector() = %#v", got)
	}
}

func TestOpenAICompatibleProviderEmbed(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embeddings" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if gotAuth = r.Header.Get("Authorization"); gotAuth != "Bearer secret" {
			t.Fatalf("auth = %q", gotAuth)
		}
		var body struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Model != "embed-1" || !reflect.DeepEqual(body.Input, []string{"hello", "world"}) {
			t.Fatalf("body = %#v", body)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"embedding": []float32{3, 4}},
				{"embedding": []float32{0, 5}},
			},
		})
	}))
	defer server.Close()

	p := newOpenAICompatibleProvider(EmbeddingConfig{BaseURL: server.URL, Model: "embed-1", APIKey: "secret"})
	vecs, err := p.Embed(context.Background(), []string{"hello", "world"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vecs) != 2 || !approxEqual(vecs[0][0], 0.6) || !approxEqual(vecs[0][1], 0.8) || !approxEqual(vecs[1][0], 0) || !approxEqual(vecs[1][1], 1) {
		t.Fatalf("vecs = %#v", vecs)
	}
}

func TestGeminiProviderEmbed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models/gem-embed:embedContent" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		var body struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.Content.Parts) != 1 || body.Content.Parts[0].Text != "query" {
			t.Fatalf("body = %#v", body)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"embedding": map[string]any{"values": []float32{1, 2, 2}}})
	}))
	defer server.Close()

	p := newGeminiProvider(EmbeddingConfig{BaseURL: server.URL, Model: "gem-embed"})
	vecs, err := p.Embed(context.Background(), []string{"query"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vecs) != 1 || !approxEqual(vecs[0][0], 1.0/3.0) || !approxEqual(vecs[0][1], 2.0/3.0) || !approxEqual(vecs[0][2], 2.0/3.0) {
		t.Fatalf("vecs = %#v", vecs)
	}
}

func TestOllamaProviderEmbed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embeddings" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		var body struct {
			Model  string `json:"model"`
			Prompt string `json:"prompt"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Model != "ollama-embed" || body.Prompt != "hello" {
			t.Fatalf("body = %#v", body)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"embedding": []float32{2, 0}})
	}))
	defer server.Close()

	p := newOllamaProvider(EmbeddingConfig{BaseURL: server.URL, Model: "ollama-embed"})
	vecs, err := p.Embed(context.Background(), []string{"hello"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vecs) != 1 || !approxEqual(vecs[0][0], 1) || !approxEqual(vecs[0][1], 0) {
		t.Fatalf("vecs = %#v", vecs)
	}
}

func TestUnsupportedEmbeddingProvider(t *testing.T) {
	if _, err := newEmbeddingProvider(EmbeddingConfig{Provider: "nope"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestSimpleRateLimiterNil(t *testing.T) {
	limiter := newSimpleRateLimiter(0)
	if limiter != nil {
		t.Fatal("expected nil limiter for rate_limit=0")
	}
	if err := limiter.wait(context.Background()); err != nil {
		t.Fatal("nil limiter should not error")
	}
}

func TestSimpleRateLimiterBasic(t *testing.T) {
	limiter := newSimpleRateLimiter(10)
	if limiter == nil {
		t.Fatal("expected non-nil limiter")
	}
	ctx := context.Background()
	start := time.Now()
	for i := 0; i < 3; i++ {
		if err := limiter.wait(ctx); err != nil {
			t.Fatal(err)
		}
	}
	elapsed := time.Since(start)
	if elapsed < 200*time.Millisecond {
		t.Fatalf("elapsed = %v, expected >= 200ms for 3 requests at 10/sec", elapsed)
	}
}

func TestSimpleRateLimiterContextCancel(t *testing.T) {
	limiter := newSimpleRateLimiter(1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := limiter.wait(ctx); err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestOpenAICompatibleProviderWithRateLimit(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"embedding": []float32{1, 0}}},
		})
	}))
	defer server.Close()

	p := newOpenAICompatibleProvider(EmbeddingConfig{
		BaseURL:   server.URL,
		Model:     "test",
		RateLimit: 5,
	})
	start := time.Now()
	for i := 0; i < 3; i++ {
		_, err := p.Embed(context.Background(), []string{"test"})
		if err != nil {
			t.Fatal(err)
		}
	}
	elapsed := time.Since(start)
	if requestCount != 3 {
		t.Fatalf("requestCount = %d, want 3", requestCount)
	}
	if elapsed < 300*time.Millisecond {
		t.Fatalf("elapsed = %v, expected >= 400ms for 3 requests at 5/sec", elapsed)
	}
}

func TestOpenAICompatibleProviderWithTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"embedding": []float32{1, 0}}},
		})
	}))
	defer server.Close()

	p := newOpenAICompatibleProvider(EmbeddingConfig{
		BaseURL: server.URL,
		Model:   "test",
		Timeout: "100ms",
	})
	_, err := p.Embed(context.Background(), []string{"test"})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestGeminiProviderWithRateLimit(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		_ = json.NewEncoder(w).Encode(map[string]any{"embedding": map[string]any{"values": []float32{1, 0}}})
	}))
	defer server.Close()

	p := newGeminiProvider(EmbeddingConfig{
		BaseURL:   server.URL,
		Model:     "test",
		RateLimit: 5,
	})
	start := time.Now()
	_, err := p.Embed(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	if requestCount != 2 {
		t.Fatalf("requestCount = %d, want 2", requestCount)
	}
	if elapsed < 150*time.Millisecond {
		t.Fatalf("elapsed = %v, expected >= 200ms for 2 requests at 5/sec", elapsed)
	}
}

func TestOllamaProviderWithRateLimit(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		_ = json.NewEncoder(w).Encode(map[string]any{"embedding": []float32{1, 0}})
	}))
	defer server.Close()

	p := newOllamaProvider(EmbeddingConfig{
		BaseURL:   server.URL,
		Model:     "test",
		RateLimit: 5,
	})
	start := time.Now()
	_, err := p.Embed(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	if requestCount != 2 {
		t.Fatalf("requestCount = %d, want 2", requestCount)
	}
	if elapsed < 150*time.Millisecond {
		t.Fatalf("elapsed = %v, expected >= 200ms for 2 requests at 5/sec", elapsed)
	}
}
