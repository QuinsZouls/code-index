package search

import (
	"regexp"
	"strings"

	"github.com/QuinsZouls/code-index/internal/types"
	"github.com/QuinsZouls/code-index/internal/utils"
)

type HybridScorer struct {
	vectorWeight  float64
	keywordWeight float64
	docFreq       map[string]int
	totalDocs     int
	projectRoot   string
}

func NewHybridScorer(vectorWeight, keywordWeight float64, chunksByFile map[string][]types.ChunkRecord, projectRoot string) *HybridScorer {
	if vectorWeight < 0 {
		vectorWeight = 0
	}
	if keywordWeight < 0 {
		keywordWeight = 0
	}
	total := vectorWeight + keywordWeight
	if total > 0 {
		vectorWeight /= total
		keywordWeight /= total
	} else {
		vectorWeight = 0.7
		keywordWeight = 0.3
	}

	docFreq := make(map[string]int)
	totalDocs := 0
	for _, chunks := range chunksByFile {
		totalDocs += len(chunks)
		for _, ch := range chunks {
			content := utils.ReadChunkContent(projectRoot, ch.FilePath, ch.StartLine, ch.EndLine)
			terms := tokenize(content)
			seen := make(map[string]bool)
			for _, term := range terms {
				if !seen[term] {
					docFreq[term]++
					seen[term] = true
				}
			}
		}
	}

	return &HybridScorer{
		vectorWeight:  vectorWeight,
		keywordWeight: keywordWeight,
		docFreq:       docFreq,
		totalDocs:     totalDocs,
		projectRoot:   projectRoot,
	}
}

func (h *HybridScorer) CombineScores(vectorScore float64, filePath string, startLine, endLine int, queryTerms []string) float64 {
	if h.keywordWeight == 0 {
		return vectorScore
	}
	content := utils.ReadChunkContent(h.projectRoot, filePath, startLine, endLine)
	keywordScore := h.tfidfScore(content, queryTerms)
	return h.vectorWeight*vectorScore + h.keywordWeight*keywordScore
}

// TFIDFScore computes the TF-IDF keyword score for the given content and query terms.
func (h *HybridScorer) TFIDFScore(content string, queryTerms []string) float64 {
	return h.tfidfScore(content, queryTerms)
}

func (h *HybridScorer) tfidfScore(content string, queryTerms []string) float64 {
	if len(queryTerms) == 0 || h.totalDocs == 0 {
		return 0
	}

	contentTerms := tokenize(content)
	termFreq := make(map[string]int)
	for _, term := range contentTerms {
		termFreq[term]++
	}

	var score float64
	for _, qTerm := range queryTerms {
		tf := float64(termFreq[qTerm])
		if tf == 0 {
			continue
		}
		df := float64(h.docFreq[qTerm])
		if df == 0 {
			df = 1
		}
		idf := 1 + float64(h.totalDocs)/df
		tfidf := tf * idf
		score += tfidf
	}

	if len(contentTerms) > 0 {
		score /= float64(len(contentTerms))
	}
	if score > 1 {
		score = 1
	}
	return score
}

func tokenize(text string) []string {
	text = strings.ToLower(text)
	re := regexp.MustCompile(`[a-z0-9_]+`)
	terms := re.FindAllString(text, -1)
	return terms
}

// Tokenize splits text into lowercase tokens.
func Tokenize(text string) []string {
	return tokenize(text)
}

func ExtractQueryTerms(query string) []string {
	return tokenize(query)
}
