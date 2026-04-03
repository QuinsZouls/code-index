package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type stubEmbeddingProvider struct {
	calls int
	texts []string
	vecs  map[string][]float32
}

func (s *stubEmbeddingProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	s.calls++
	s.texts = append(s.texts, texts...)
	out := make([][]float32, len(texts))
	for i, text := range texts {
		vec, ok := s.vecs[text]
		if !ok {
			vec = []float32{0}
		}
		out[i] = append([]float32(nil), vec...)
	}
	return out, nil
}

func TestIndexerIndexSearchAndSkipUnchanged(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "hello.go"), []byte("alpha"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := defaultConfig()
	cfg.IncludePatterns = []string{"**/*.go"}
	cfg.Embedding = EmbeddingConfig{Provider: "openai-compatible", Model: "fake"}
	provider := &stubEmbeddingProvider{vecs: map[string][]float32{"alpha": {1, 0}}}
	indexer := &Indexer{
		projectRoot: root,
		cfg:         cfg,
		provider:    provider,
		index:       newIndexData(cfg.embeddingSignature()),
	}
	if err := indexer.Index(context.Background()); err != nil {
		t.Fatal(err)
	}
	if provider.calls != 1 {
		t.Fatalf("Embed calls = %d, want 1", provider.calls)
	}
	if got := indexer.index.Files["hello.go"]; got.Size == 0 || got.ModTimeUnixNano == 0 {
		t.Fatalf("file state = %#v", got)
	}
	firstStat, err := os.Stat(filepath.Join(root, "hello.go"))
	if err != nil {
		t.Fatal(err)
	}
	if firstState := indexer.index.Files["hello.go"]; firstState.Size != firstStat.Size() || firstState.ModTimeUnixNano == 0 {
		t.Fatalf("file state = %#v", firstState)
	}
	if _, err := os.Stat(indexPath(root)); err != nil {
		t.Fatal(err)
	}
	searchProvider := &stubEmbeddingProvider{vecs: map[string][]float32{"alpha": {1, 0}}}
	indexer.provider = searchProvider
	results, err := indexer.Search(context.Background(), SearchOptions{Query: "alpha", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].FilePath != "hello.go" || results[0].Language != "go" {
		t.Fatalf("results = %#v", results)
	}
	if results[0].Score < 0.99 {
		t.Fatalf("score = %v, want close to 1", results[0].Score)
	}
	indexer.provider = provider
	if err := indexer.Index(context.Background()); err != nil {
		t.Fatal(err)
	}
	if provider.calls != 1 {
		t.Fatalf("Embed calls = %d, want 1 on unchanged reindex", provider.calls)
	}
	if err := os.WriteFile(filepath.Join(root, "hello.go"), []byte("beta"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "hello.go"), []byte("beta-beta"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := indexer.Index(context.Background()); err != nil {
		t.Fatal(err)
	}
	if provider.calls != 2 {
		t.Fatalf("Embed calls = %d, want 2 after modified file", provider.calls)
	}
}

func TestIndexerHelpers(t *testing.T) {
	idx := &Indexer{cfg: defaultConfig()}
	if got := idx.languageFor("src/main.go"); got != "go" {
		t.Fatalf("languageFor() = %q", got)
	}
	if !containsString([]string{"go", "python"}, "python") {
		t.Fatal("containsString() should be true")
	}
	if !matchesAnyGlob([]string{"**/*.go"}, "src/main.go") {
		t.Fatal("matchesAnyGlob() should be true")
	}
	if score := cosine([]float32{1, 0}, []float32{1, 0}); score < 0.99 {
		t.Fatalf("cosine() = %v", score)
	}
	if got := idx.Status(); !reflect.DeepEqual(got.Langs, map[string]int{}) {
		t.Fatalf("Status() = %#v", got)
	}
}
