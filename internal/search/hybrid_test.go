package search_test

import (
	"testing"

	"github.com/QuinsZouls/code-index/internal/search"
	"github.com/QuinsZouls/code-index/internal/types"
)

func TestTokenize(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"Hello World", []string{"hello", "world"}},
		{"API-Endpoint_Handler", []string{"api", "endpoint_handler"}},
		{"func main() {}", []string{"func", "main"}},
		{"test123", []string{"test123"}},
		{"", []string{}},
	}

	for _, tt := range tests {
		got := search.Tokenize(tt.input)
		if len(got) != len(tt.expected) {
			t.Errorf("Tokenize(%q) = %v, want %v", tt.input, got, tt.expected)
			continue
		}
		for i, v := range got {
			if v != tt.expected[i] {
				t.Errorf("Tokenize(%q)[%d] = %q, want %q", tt.input, i, v, tt.expected[i])
			}
		}
	}
}

func TestExtractQueryTerms(t *testing.T) {
	query := "database connection pooling"
	terms := search.ExtractQueryTerms(query)
	expected := []string{"database", "connection", "pooling"}

	if len(terms) != len(expected) {
		t.Fatalf("ExtractQueryTerms(%q) = %v, want %v", query, terms, expected)
	}

	for i, term := range terms {
		if term != expected[i] {
			t.Errorf("ExtractQueryTerms(%q)[%d] = %q, want %q", query, i, term, expected[i])
		}
	}
}

func TestHybridScorerTFIDF(t *testing.T) {
	projectRoot := t.TempDir()
	chunks := map[string][]types.ChunkRecord{
		"test.go": {
			{FilePath: "test.go", StartLine: 1, EndLine: 10, Language: "go"},
		},
	}

	scorer := search.NewHybridScorer(0.7, 0.3, chunks, projectRoot)

	content := "package main\n\nfunc main() {\n\tfmt.Println(\"hello world\")\n}"
	queryTerms := []string{"main", "func"}

	score := scorer.TFIDFScore(content, queryTerms)

	if score < 0 || score > 1 {
		t.Errorf("TFIDFScore() = %v, want between 0 and 1", score)
	}
}

func TestHybridScorerCombineScores(t *testing.T) {
	projectRoot := t.TempDir()
	chunks := map[string][]types.ChunkRecord{
		"test.go": {
			{FilePath: "test.go", StartLine: 1, EndLine: 10, Language: "go"},
		},
	}

	scorer := search.NewHybridScorer(0.7, 0.3, chunks, projectRoot)

	vectorScore := 0.8
	queryTerms := []string{"main"}

	combined := scorer.CombineScores(vectorScore, "test.go", 1, 10, queryTerms)

	if combined < 0 || combined > 1 {
		t.Errorf("CombineScores() = %v, want between 0 and 1", combined)
	}

	if combined < vectorScore*0.7 {
		t.Errorf("CombineScores() = %v, should be at least 0.7 * vectorScore", combined)
	}
}

func TestHybridScorerZeroKeywordWeight(t *testing.T) {
	projectRoot := t.TempDir()
	chunks := map[string][]types.ChunkRecord{}

	scorer := search.NewHybridScorer(1.0, 0.0, chunks, projectRoot)

	vectorScore := 0.8
	combined := scorer.CombineScores(vectorScore, "test.go", 1, 10, []string{"query"})

	if combined != vectorScore {
		t.Errorf("CombineScores() with zero keyword weight = %v, want %v", combined, vectorScore)
	}
}
