package main

import (
	"context"
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

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "init":
		runInit(os.Args[2:])
	case "index":
		runIndex(os.Args[2:])
	case "search":
		runSearch(os.Args[2:])
	case "status":
		runStatus(os.Args[2:])
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "ccc commands: init, index, search, status")
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

func mustProjectRoot() string {
	if root := os.Getenv("COCOINDEX_CODE_ROOT_PATH"); root != "" {
		return root
	}
	root, err := findProjectRoot(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return root
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
	indexer.progressFn = func(p IndexProgress) {
		printer.Emit(p)
	}
	if err := indexer.Index(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printer.Done()
	status := indexer.Status()
	fmt.Printf("indexed %d files, %d chunks in %s using %d workers\n", status.Files, status.Chunks, time.Since(start).Round(time.Millisecond), runtime.GOMAXPROCS(0))
}

func runSearch(args []string) {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	path := fs.String("path", ".", "project root")
	limit := fs.Int("limit", 5, "max results")
	offset := fs.Int("offset", 0, "offset")
	langArg := fs.String("lang", "", "comma-separated languages")
	pathArg := fs.String("glob", "", "comma-separated path globs")
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	results, err := indexer.Search(ctx, SearchOptions{Query: query, Limit: *limit, Offset: *offset, Languages: langs, Paths: globs})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(results) == 0 {
		fmt.Println("No results found.")
		return
	}
	for i, r := range results {
		fmt.Printf("\n--- Result %d (score: %.3f) ---\n", i+1, r.Score)
		fmt.Printf("File: %s:%d-%d [%s]\n", r.FilePath, r.StartLine, r.EndLine, r.Language)
		fmt.Println(r.Content)
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
