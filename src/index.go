package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

type SearchOptions struct {
	Query     string
	Limit     int
	Offset    int
	Languages []string
	Paths     []string
	UseHybrid bool
}

type Indexer struct {
	projectRoot string
	cfg         Config
	provider    EmbeddingProvider
	index       *IndexData
	progressFn  func(IndexProgress)
}

func newIndexer(projectRoot string, cfg Config) (*Indexer, error) {
	provider, err := newEmbeddingProvider(cfg.Embedding)
	if err != nil {
		return nil, err
	}
	idx, err := loadIndex(indexPath(projectRoot))
	if err != nil {
		return nil, err
	}
	sig := cfg.embeddingSignature()
	if idx == nil || idx.EmbeddingSignature != sig {
		idx = newIndexData(sig)
	}
	return &Indexer{projectRoot: projectRoot, cfg: cfg, provider: provider, index: idx}, nil
}

func (i *Indexer) Index(ctx context.Context) error {
	files, err := walkFiles(i.projectRoot, i.cfg)
	if err != nil {
		return err
	}
	currentSig := i.cfg.embeddingSignature()
	checkpointEvery := i.cfg.CheckpointEvery
	if checkpointEvery <= 0 {
		checkpointEvery = len(files) / 50
		if checkpointEvery < 10 {
			checkpointEvery = 10
		}
		if checkpointEvery > 100 {
			checkpointEvery = 100
		}
	}
	prevFiles := make(map[string]FileState, len(i.index.Files))
	for path, state := range i.index.Files {
		prevFiles[path] = state
	}
	seen := map[string]struct{}{}
	type fileJob struct {
		rel string
		idx int
	}
	type fileResult struct {
		rel     string
		hash    string
		size    int64
		modNano int64
		kind    string
		records []ChunkRecord
		skipped bool
		err     error
	}
	workerCount := i.cfg.WorkerCount
	if workerCount <= 0 {
		workerCount = runtime.GOMAXPROCS(0) * 2
		if workerCount < 4 {
			workerCount = 4
		}
		if workerCount > 16 {
			workerCount = 16
		}
	}
	if len(files) < workerCount {
		workerCount = len(files)
	}
	if workerCount == 0 {
		workerCount = 1
	}
	jobs := make(chan fileJob, workerCount*2)
	results := make(chan fileResult, workerCount*2)
	var wg sync.WaitGroup
	var lastFlush = time.Now()
	var pendingWrites int
	flushIndex := func(force bool) error {
		if !force && pendingWrites < checkpointEvery && time.Since(lastFlush) < 3*time.Second {
			return nil
		}
		if err := saveIndex(indexPath(i.projectRoot), i.index); err != nil {
			return err
		}
		pendingWrites = 0
		lastFlush = time.Now()
		return nil
	}
	worker := func() {
		defer wg.Done()
		for job := range jobs {
			select {
			case <-ctx.Done():
				results <- fileResult{rel: job.rel, err: ctx.Err()}
				continue
			default:
			}
			abs := filepath.Join(i.projectRoot, job.rel)
			info, err := os.Stat(abs)
			if err != nil {
				results <- fileResult{rel: job.rel, err: err}
				continue
			}
			modNano := info.ModTime().UTC().UnixNano()
			size := info.Size()
			prev, ok := prevFiles[job.rel]
			if ok && prev.Size == size && prev.ModTimeUnixNano == modNano && i.index.EmbeddingSignature == currentSig {
				results <- fileResult{rel: job.rel, hash: prev.Hash, size: size, modNano: modNano, kind: "unchanged", skipped: true}
				continue
			}
			data, err := os.ReadFile(abs)
			if err != nil {
				results <- fileResult{rel: job.rel, err: err}
				continue
			}
			hash := fileHash(data)
			chunks := i.fileChunks(job.rel, string(data))
			texts := make([]string, 0, len(chunks))
			for _, ch := range chunks {
				texts = append(texts, ch.Content)
			}
			vecs, err := i.provider.Embed(ctx, texts)
			if err != nil {
				results <- fileResult{rel: job.rel, err: fmt.Errorf("embed %s: %w", job.rel, err)}
				continue
			}
			records := make([]ChunkRecord, 0, len(chunks))
			lang := i.languageFor(job.rel)
			for idx, ch := range chunks {
				records = append(records, ChunkRecord{
					FilePath:  job.rel,
					Language:  lang,
					StartLine: ch.StartLine,
					EndLine:   ch.EndLine,
					Embedding: vecs[idx],
					ChunkHash: fileHash([]byte(ch.Content)),
				})
			}
			kind := "modified"
			if !ok {
				kind = "new"
			}
			results <- fileResult{rel: job.rel, hash: hash, size: size, modNano: modNano, kind: kind, records: records}
		}
	}
	for range make([]struct{}, workerCount) {
		wg.Add(1)
		go worker()
	}
	go func() {
		for idx, rel := range files {
			if i.progressFn != nil {
				i.progressFn(IndexProgress{Current: idx + 1, Total: len(files), File: rel, Action: "queued"})
			}
			jobs <- fileJob{rel: rel, idx: idx}
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()
	var firstErr error
	for result := range results {
		if result.err != nil {
			if firstErr == nil {
				firstErr = result.err
			}
			continue
		}
		seen[result.rel] = struct{}{}
		if result.skipped {
			if i.progressFn != nil {
				i.progressFn(IndexProgress{File: result.rel, Action: "skipped", Kind: result.kind})
			}
			continue
		}
		i.index.Files[result.rel] = FileState{Hash: result.hash, ChunkCount: len(result.records), Size: result.size, ModTimeUnixNano: result.modNano}
		i.index.ChunksByFile[result.rel] = result.records
		pendingWrites++
		if err := flushIndex(false); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if i.progressFn != nil {
			i.progressFn(IndexProgress{File: result.rel, Action: "indexed", Kind: result.kind, Chunks: len(result.records)})
		}
	}
	for file := range i.index.Files {
		if _, ok := seen[file]; !ok {
			delete(i.index.Files, file)
			delete(i.index.ChunksByFile, file)
		}
	}
	i.index.EmbeddingSignature = i.cfg.embeddingSignature()
	if err := flushIndex(true); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (i *Indexer) fileChunks(relPath, content string) []Chunk {
	return chunkText(content, i.cfg.ChunkSize, i.cfg.ChunkOverlap)
}

func (i *Indexer) languageFor(relPath string) string {
	ext := strings.ToLower(filepath.Ext(relPath))
	if lang, ok := i.cfg.LanguageOverrides[strings.TrimPrefix(ext, ".")]; ok {
		return lang
	}
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js", ".jsx":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".c", ".h", ".cpp", ".hpp":
		return "c-cpp"
	case ".md":
		return "markdown"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	default:
		return "text"
	}
}

func (i *Indexer) Search(ctx context.Context, opts SearchOptions) ([]SearchResult, error) {
	if i.index == nil {
		return nil, errors.New("index not loaded")
	}
	if opts.Limit <= 0 {
		opts.Limit = 5
	}
	if opts.Offset < 0 {
		opts.Offset = 0
	}

	useHybrid := i.cfg.HybridSearch || opts.UseHybrid

	queryVecs, err := i.provider.Embed(ctx, []string{opts.Query})
	if err != nil {
		return nil, err
	}
	if len(queryVecs) == 0 {
		return nil, nil
	}
	queryVec := queryVecs[0]

	var hybridScorer *HybridScorer
	var queryTerms []string

	if useHybrid {
		queryTerms = extractQueryTerms(opts.Query)
		hybridScorer = newHybridScorer(i.cfg.VectorWeight, i.cfg.KeywordWeight, i.index.ChunksByFile, i.projectRoot)
	}

	results := make([]SearchResult, 0, 32)
	for _, chunks := range i.index.ChunksByFile {
		for _, ch := range chunks {
			if len(opts.Languages) > 0 && !containsString(opts.Languages, ch.Language) {
				continue
			}
			if len(opts.Paths) > 0 && !matchesAnyGlob(opts.Paths, ch.FilePath) {
				continue
			}
			vectorScore := cosine(queryVec, ch.Embedding)

			var finalScore float64
			if useHybrid && hybridScorer != nil {
				finalScore = hybridScorer.combineScores(vectorScore, ch.FilePath, ch.StartLine, ch.EndLine, queryTerms)
			} else {
				finalScore = vectorScore
			}

			results = append(results, SearchResult{
				FilePath:  ch.FilePath,
				Language:  ch.Language,
				StartLine: ch.StartLine,
				EndLine:   ch.EndLine,
				Score:     finalScore,
			})
		}
	}
	sort.Slice(results, func(a, b int) bool { return results[a].Score > results[b].Score })
	if opts.Offset > len(results) {
		return nil, nil
	}
	results = results[opts.Offset:]
	if opts.Limit < len(results) {
		results = results[:opts.Limit]
	}
	return results, nil
}

func (i *Indexer) Status() Status {
	status := Status{Langs: map[string]int{}}
	if i.index == nil {
		return status
	}
	status.Files = len(i.index.Files)
	for _, chunks := range i.index.ChunksByFile {
		status.Chunks += len(chunks)
		for _, ch := range chunks {
			status.Langs[ch.Language]++
		}
	}
	return status
}

func containsString(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}

func matchesAnyGlob(patterns []string, relPath string) bool {
	for _, pattern := range patterns {
		if matchPattern(pattern, relPath) {
			return true
		}
	}
	return false
}

func cosine(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	if len(a) > len(b) {
		a = a[:len(b)]
	} else if len(b) > len(a) {
		b = b[:len(a)]
	}
	var sum float64
	for i := range a {
		sum += float64(a[i] * b[i])
	}
	return sum
}
