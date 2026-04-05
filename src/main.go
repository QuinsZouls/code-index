package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const appVersion = "0.1.3"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "version":
		fmt.Println(appVersion)
	case "init":
		runInit(os.Args[2:])
	case "index":
		runIndex(os.Args[2:])
	case "search":
		runSearch(os.Args[2:])
	case "status":
		runStatus(os.Args[2:])
	case "doctor":
		runDoctor(os.Args[2:])
	case "clear":
		runClear(os.Args[2:])
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "codeindex commands: version, init, index, search, status, doctor, clear")
}

func findProjectRoot(start string) (string, error) {
	cur, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(settingsPath(cur)); err == nil {
			return cur, nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return start, nil
		}
		cur = parent
	}
}

func runInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	path := fs.String("path", ".", "project root")
	_ = fs.Parse(args)
	root, err := filepath.Abs(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if _, err := initProject(root); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("initialized:", root)
}

func runIndex(args []string) {
	fs := flag.NewFlagSet("index", flag.ExitOnError)
	path := fs.String("path", ".", "project root")
	verbose := fs.Bool("verbose", false, "show per-file progress")
	_ = fs.Parse(args)
	root, err := filepath.Abs(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cfg, err := loadConfig(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	indexer, err := newIndexer(root, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	start := time.Now()
	printer := newProgressPrinter(os.Stdout, os.Stderr, "Indexing")
	defer printer.Stop()
	var indexed int
	indexer.progressFn = func(p IndexProgress) {
		if *verbose {
			printer.Emit(p)
		}
		if p.Action == "indexed" || p.Action == "skipped" {
			indexed++
		}
	}
	if err := indexer.Index(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printer.Done()
	status := indexer.Status()
	if *verbose {
		fmt.Printf("indexed %d files, %d chunks in %s using %d workers\n", status.Files, status.Chunks, time.Since(start).Round(time.Millisecond), runtime.GOMAXPROCS(0))
		return
	}
	fmt.Printf("indexed %d files in %s\n", indexed, time.Since(start).Round(time.Millisecond))
}

func runSearch(args []string) {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	path := fs.String("path", ".", "project root")
	limit := fs.Int("limit", 0, "max results (0 = use config default)")
	offset := fs.Int("offset", 0, "offset")
	langArg := fs.String("lang", "", "comma-separated languages")
	pathArg := fs.String("glob", "", "comma-separated path globs")
	filesOnly := fs.Bool("files", false, "show only file paths without content")
	hybrid := fs.Bool("hybrid", false, "use hybrid search (vector + keyword)")
	_ = fs.Parse(args)
	query := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if query == "" {
		fmt.Fprintln(os.Stderr, "search query required")
		os.Exit(1)
	}
	root, err := filepath.Abs(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cfg, err := loadConfig(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	indexer, err := newIndexer(root, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	langs := splitCSV(*langArg)
	globs := splitCSV(*pathArg)
	searchLimit := *limit
	if searchLimit <= 0 {
		searchLimit = cfg.SearchLimit
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	fetchLimit := searchLimit * 3
	if fetchLimit < 50 {
		fetchLimit = 50
	}
	results, err := indexer.Search(ctx, SearchOptions{
		Query:     query,
		Limit:     fetchLimit,
		Offset:    *offset,
		Languages: langs,
		Paths:     globs,
		UseHybrid: cfg.HybridSearch || *hybrid,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	filtered := make([]SearchResult, 0, len(results))
	for _, r := range results {
		if r.Score >= cfg.ScoreThreshold {
			filtered = append(filtered, r)
		}
	}
	if len(filtered) > searchLimit {
		filtered = filtered[:searchLimit]
	}
	if len(filtered) == 0 {
		fmt.Println("No results found.")
		return
	}
	if *filesOnly {
		for i, r := range filtered {
			fmt.Printf("%d. %s:%d-%d [%s] (score: %.3f)\n", i+1, r.FilePath, r.StartLine, r.EndLine, r.Language, r.Score)
		}
		return
	}
	for i, r := range filtered {
		content := readChunkContent(root, r.FilePath, r.StartLine, r.EndLine)
		fmt.Printf("\n--- Result %d (score: %.3f) ---\n", i+1, r.Score)
		fmt.Printf("File: %s:%d-%d [%s]\n", r.FilePath, r.StartLine, r.EndLine, r.Language)
		fmt.Println(content)
	}
}

func runStatus(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	path := fs.String("path", ".", "project root")
	_ = fs.Parse(args)
	root, err := filepath.Abs(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cfg, err := loadConfig(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	indexer, err := newIndexer(root, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	status := indexer.Status()
	fmt.Printf("Project: %s\n", root)
	fmt.Printf("Files: %d\n", status.Files)
	fmt.Printf("Chunks: %d\n", status.Chunks)
	langs := make([]string, 0, len(status.Langs))
	for lang := range status.Langs {
		langs = append(langs, lang)
	}
	sort.Strings(langs)
	for _, lang := range langs {
		fmt.Printf("  %s: %d\n", lang, status.Langs[lang])
	}
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func readChunkContent(projectRoot, relPath string, startLine, endLine int) string {
	absPath := filepath.Join(projectRoot, relPath)
	data, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Sprintf("[file unavailable: %v]", err)
	}
	lines := strings.Split(string(data), "\n")
	if startLine < 1 || startLine > len(lines) {
		return "[line range invalid]"
	}
	if endLine > len(lines) {
		endLine = len(lines)
	}
	return strings.Join(lines[startLine-1:endLine], "\n")
}

type progressPrinter struct {
	out   *os.File
	err   *os.File
	label string
	mu    sync.Mutex
	done  chan struct{}
	once  sync.Once
}

func newProgressPrinter(out, err *os.File, label string) *progressPrinter {
	p := &progressPrinter{out: out, err: err, label: label, done: make(chan struct{})}
	go p.spin()
	return p
}

func (p *progressPrinter) spin() {
	frames := []string{"|", "/", "-", "\\"}
	ticker := time.NewTicker(120 * time.Millisecond)
	defer ticker.Stop()
	idx := 0
	for {
		select {
		case <-p.done:
			p.mu.Lock()
			fmt.Fprint(p.err, "\r")
			fmt.Fprint(p.err, strings.Repeat(" ", len(p.label)+8))
			fmt.Fprint(p.err, "\r")
			p.mu.Unlock()
			return
		case <-ticker.C:
			p.mu.Lock()
			fmt.Fprintf(p.err, "\r%s %s", frames[idx%len(frames)], p.label)
			p.mu.Unlock()
			idx++
		}
	}
}

func (p *progressPrinter) Emit(progress IndexProgress) {
	if progress.File == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	switch progress.Action {
	case "queued":
		fmt.Fprintf(p.out, "[%d/%d] %s\n", progress.Current, progress.Total, progress.File)
	case "skipped":
		fmt.Fprintf(p.out, "[=] %s (unchanged)\n", progress.File)
	case "indexed":
		switch progress.Kind {
		case "new":
			fmt.Fprintf(p.out, "[+] %s (new, %d chunks)\n", progress.File, progress.Chunks)
		case "modified":
			fmt.Fprintf(p.out, "[~] %s (updated, %d chunks)\n", progress.File, progress.Chunks)
		default:
			fmt.Fprintf(p.out, "[+] %s (%d chunks)\n", progress.File, progress.Chunks)
		}
	}
}

func (p *progressPrinter) Done() {
	p.once.Do(func() {
		close(p.done)
		time.Sleep(20 * time.Millisecond)
	})
}

func (p *progressPrinter) Stop() {
	p.Done()
}

func runDoctor(args []string) {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	path := fs.String("path", ".", "project root")
	_ = fs.Parse(args)
	root, err := filepath.Abs(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("codeindex doctor")
	fmt.Println("===============")
	fmt.Println()
	hasErrors := false
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("[FAIL] Home directory: %v\n", err)
		hasErrors = true
	} else {
		fmt.Printf("[OK]   Home directory: %s\n", home)
		globalPath := filepath.Join(home, settingsDirName, "default_settings.json")
		if data, err := os.ReadFile(globalPath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				fmt.Printf("[WARN] Global settings: not found at %s\n", globalPath)
			} else {
				fmt.Printf("[FAIL] Global settings: %v\n", err)
				hasErrors = true
			}
		} else {
			fmt.Printf("[OK]   Global settings: %s\n", globalPath)
			var cfg Config
			if err := json.Unmarshal(data, &cfg); err != nil {
				fmt.Printf("[FAIL] Global settings parse: %v\n", err)
				hasErrors = true
			} else {
				cfg.normalize()
				fmt.Printf("       Provider: %s\n", cfg.Embedding.Provider)
				fmt.Printf("       Model: %s\n", cfg.Embedding.Model)
				fmt.Printf("       Base URL: %s\n", cfg.Embedding.BaseURL)
				key := apiKey(cfg.Embedding)
				if key == "" && cfg.Embedding.Provider != "ollama" && cfg.Embedding.Provider != "lmstudio" && cfg.Embedding.Provider != "llamacpp" {
					fmt.Printf("[FAIL] API key: not set (env: %s)\n", cfg.Embedding.APIKeyEnv)
					hasErrors = true
				} else if key != "" {
					fmt.Printf("[OK]   API key: set (env: %s, length: %d)\n", cfg.Embedding.APIKeyEnv, len(key))
				} else {
					fmt.Printf("[OK]   API key: not required for %s\n", cfg.Embedding.Provider)
				}
				provider, err := newEmbeddingProvider(cfg.Embedding)
				if err != nil {
					fmt.Printf("[FAIL] Provider init: %v\n", err)
					hasErrors = true
				} else {
					fmt.Printf("[OK]   Provider init: %s\n", cfg.Embedding.Provider)
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					vecs, err := provider.Embed(ctx, []string{"test"})
					if err != nil {
						fmt.Printf("[FAIL] Test embedding: %v\n", err)
						hasErrors = true
					} else if len(vecs) > 0 && len(vecs[0]) > 0 {
						fmt.Printf("[OK]   Test embedding: dimension %d\n", len(vecs[0]))
					} else {
						fmt.Printf("[FAIL] Test embedding: empty response\n")
						hasErrors = true
					}
				}
			}
		}
	}
	projectSettings := settingsPath(root)
	if data, err := os.ReadFile(projectSettings); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Printf("[WARN] Project settings: not found at %s\n", projectSettings)
		} else {
			fmt.Printf("[FAIL] Project settings: %v\n", err)
			hasErrors = true
		}
	} else {
		fmt.Printf("[OK]   Project settings: %s\n", projectSettings)
		var cfg Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			fmt.Printf("[FAIL] Project settings parse: %v\n", err)
			hasErrors = true
		}
	}
	indexFile := indexPath(root)
	if _, err := os.Stat(indexFile); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Printf("[WARN] Index file: not found at %s\n", indexFile)
		} else {
			fmt.Printf("[FAIL] Index file: %v\n", err)
			hasErrors = true
		}
	} else {
		fmt.Printf("[OK]   Index file: %s\n", indexFile)
	}
	fmt.Println()
	if hasErrors {
		fmt.Println("[FAIL] Doctor found errors")
		os.Exit(1)
	}
	fmt.Println("[OK]  Doctor passed")
}

func runClear(args []string) {
	fs := flag.NewFlagSet("clear", flag.ExitOnError)
	path := fs.String("path", ".", "project root")
	_ = fs.Parse(args)
	root, err := filepath.Abs(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	indexFile := indexPath(root)
	if _, err := os.Stat(indexFile); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Println("no index to clear")
			return
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.Remove(indexFile); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("cleared:", indexFile)
}
