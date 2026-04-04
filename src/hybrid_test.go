package main

import (
	"testing"
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
		got := tokenize(tt.input)
		if len(got) != len(tt.expected) {
			t.Errorf("tokenize(%q) = %v, want %v", tt.input, got, tt.expected)
			continue
		}
		for i, v := range got {
			if v != tt.expected[i] {
				t.Errorf("tokenize(%q)[%d] = %q, want %q", tt.input, i, v, tt.expected[i])
			}
		}
	}
}

func TestExtractQueryTerms(t *testing.T) {
	query := "database connection pooling"
	terms := extractQueryTerms(query)
	expected := []string{"database", "connection", "pooling"}

	if len(terms) != len(expected) {
		t.Fatalf("extractQueryTerms(%q) = %v, want %v", query, terms, expected)
	}

	for i, term := range terms {
		if term != expected[i] {
			t.Errorf("extractQueryTerms(%q)[%d] = %q, want %q", query, i, term, expected[i])
		}
	}
}

func TestHybridScorerTFIDF(t *testing.T) {
	projectRoot := t.TempDir()
	chunks := map[string][]ChunkRecord{
		"test.go": {
			{FilePath: "test.go", StartLine: 1, EndLine: 10, Language: "go"},
		},
	}

	scorer := newHybridScorer(0.7, 0.3, chunks, projectRoot)

	content := "package main\n\nfunc main() {\n\tfmt.Println(\"hello world\")\n}"
	queryTerms := []string{"main", "func"}

	score := scorer.tfidfScore(content, queryTerms)

	if score < 0 || score > 1 {
		t.Errorf("tfidfScore() = %v, want between 0 and 1", score)
	}
}

func TestHybridScorerCombineScores(t *testing.T) {
	projectRoot := t.TempDir()
	chunks := map[string][]ChunkRecord{
		"test.go": {
			{FilePath: "test.go", StartLine: 1, EndLine: 10, Language: "go"},
		},
	}

	scorer := newHybridScorer(0.7, 0.3, chunks, projectRoot)

	vectorScore := 0.8
	queryTerms := []string{"main"}

	combined := scorer.combineScores(vectorScore, "test.go", 1, 10, queryTerms)

	if combined < 0 || combined > 1 {
		t.Errorf("combineScores() = %v, want between 0 and 1", combined)
	}

	if combined < vectorScore*0.7 {
		t.Errorf("combineScores() = %v, should be at least 0.7 * vectorScore", combined)
	}
}

func TestHybridScorerZeroKeywordWeight(t *testing.T) {
	projectRoot := t.TempDir()
	chunks := map[string][]ChunkRecord{}

	scorer := newHybridScorer(1.0, 0.0, chunks, projectRoot)

	vectorScore := 0.8
	combined := scorer.combineScores(vectorScore, "test.go", 1, 10, []string{"query"})

	if combined != vectorScore {
		t.Errorf("combineScores() with zero keyword weight = %v, want %v", combined, vectorScore)
	}
}
