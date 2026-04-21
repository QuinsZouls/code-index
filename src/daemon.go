package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"
)

type FileChangeType int

const (
	FileModified FileChangeType = iota
	FileDeleted
)

type Daemon struct {
	projectRoot string
	cfg         Config
	indexer     *Indexer
	interval    time.Duration
	debounce    time.Duration
	verbose     bool
	pending     map[string]FileChangeType
	mu          sync.Mutex
	done        chan struct{}
	releaseLock func()
}

func runDaemon(args []string) {
	if len(args) < 1 {
		printDaemonUsage()
		os.Exit(1)
	}
	switch args[0] {
	case "start":
		runDaemonStart(args[1:])
	case "stop":
		runDaemonStop(args[1:])
	case "list":
		runDaemonList(args[1:])
	case "status":
		runDaemonStatus(args[1:])
	default:
		printDaemonUsage()
		os.Exit(1)
	}
}

func printDaemonUsage() {
	fmt.Fprintln(os.Stderr, "codeindex daemon commands:")
	fmt.Fprintln(os.Stderr, "  start [--path .] [--interval 2s] [--debounce 500ms] [--verbose]")
	fmt.Fprintln(os.Stderr, "  stop [--path .] [pid]")
	fmt.Fprintln(os.Stderr, "  list")
	fmt.Fprintln(os.Stderr, "  status [--path .]")
}

func runDaemonStart(args []string) {
	fs := flag.NewFlagSet("daemon start", flag.ExitOnError)
	path := fs.String("path", ".", "project root")
	intervalStr := fs.String("interval", "2s", "scan interval")
	debounceStr := fs.String("debounce", "500ms", "debounce duration")
	verbose := fs.Bool("verbose", false, "show indexing activity")
	fs.Parse(args)

	interval, err := time.ParseDuration(*intervalStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid interval: %v\n", err)
		os.Exit(1)
	}
	debounce, err := time.ParseDuration(*debounceStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid debounce: %v\n", err)
		os.Exit(1)
	}

	root, err := filepath.Abs(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if os.Getenv("CODEINDEX_DAEMON_FORK") == "1" {
		runDaemonProcess(root, interval, debounce, *verbose)
		return
	}

	cfg, err := loadConfig(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	_ = cfg

	lockPath := lockFilePath(root)
	data, err := os.ReadFile(lockPath)
	if err == nil {
		var existingPID int
		if _, err := fmt.Sscanf(string(data), "%d", &existingPID); err == nil {
			if isProcessAlive(existingPID) {
				fmt.Fprintf(os.Stderr, "daemon already running for %s (PID %d)\n", root, existingPID)
				os.Exit(1)
			}
		}
	}

	indexFile := indexPath(root)
	if _, err := os.Stat(indexFile); os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "No index found. Run 'codeindex index' first.")
		os.Exit(1)
	}

	self, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	cmdArgs := []string{"daemon", "start", "--path", root, "--interval", *intervalStr, "--debounce", *debounceStr}
	if *verbose {
		cmdArgs = append(cmdArgs, "--verbose")
	}
	cmd := exec.Command(self, cmdArgs...)
	cmd.Env = append(os.Environ(), "CODEINDEX_DAEMON_FORK=1")
	cmd.Stdout = nil
	cmd.Stderr = nil
	configureDaemonCmd(cmd)

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to start daemon: %v\n", err)
		os.Exit(1)
	}

	info := DaemonInfo{
		PID:         cmd.Process.Pid,
		ProjectRoot: root,
		StartedAt:   time.Now(),
		Interval:    *intervalStr,
		Debounce:    *debounceStr,
	}
	if err := addDaemon(info); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to register daemon: %v\n", err)
	}

	fmt.Printf("daemon started (PID %d)\n", cmd.Process.Pid)
}

func runDaemonProcess(root string, interval, debounce time.Duration, verbose bool) {
	cfg, err := loadConfig(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	indexer, err := newIndexer(root, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create indexer: %v\n", err)
		os.Exit(1)
	}

	releaseLock, err := acquireLock(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to acquire lock: %v\n", err)
		os.Exit(1)
	}

	d := &Daemon{
		projectRoot: root,
		cfg:         cfg,
		indexer:     indexer,
		interval:    interval,
		debounce:    debounce,
		verbose:     verbose,
		pending:     make(map[string]FileChangeType),
		done:        make(chan struct{}),
		releaseLock: releaseLock,
	}

	sigChan := make(chan os.Signal, 1)
	if runtime.GOOS == "windows" {
		signal.Notify(sigChan, os.Interrupt)
	} else {
		signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	}

	go func() {
		<-sigChan
		d.Stop()
	}()

	d.Run()
}

func runDaemonStop(args []string) {
	fs := flag.NewFlagSet("daemon stop", flag.ExitOnError)
	path := fs.String("path", ".", "project root")
	fs.Parse(args)

	root, err := filepath.Abs(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var pid int
	if len(fs.Args()) > 0 {
		if _, err := fmt.Sscanf(fs.Arg(0), "%d", &pid); err != nil {
			fmt.Fprintf(os.Stderr, "invalid PID: %v\n", err)
			os.Exit(1)
		}
	} else {
		info, err := findDaemonByProject(root)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if info == nil {
			fmt.Println("no daemon running for this project")
			return
		}
		pid = info.PID
	}

	if !isProcessAlive(pid) {
		removeDaemon(pid)
		fmt.Println("daemon process not running (cleaned registry)")
		return
	}

	if err := stopProcess(pid); err != nil {
		fmt.Fprintf(os.Stderr, "failed to stop daemon: %v\n", err)
		os.Exit(1)
	}

	removeDaemon(pid)
	fmt.Printf("daemon stopped (PID %d)\n", pid)
}

func runDaemonList(args []string) {
	_ = args
	if err := cleanDeadDaemons(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to clean registry: %v\n", err)
	}

	daemons, err := loadRegistry()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load registry: %v\n", err)
		os.Exit(1)
	}

	if len(daemons) == 0 {
		fmt.Println("no daemons running")
		return
	}

	fmt.Printf("%-8s %-40s %-10s %s\n", "PID", "PROJECT", "STATUS", "STARTED")
	for _, d := range daemons {
		status := "running"
		if !isProcessAlive(d.PID) {
			status = "dead"
		}
		fmt.Printf("%-8d %-40s %-10s %s\n", d.PID, truncate(d.ProjectRoot, 40), status, d.StartedAt.Format("2006-01-02 15:04:05"))
	}
}

func runDaemonStatus(args []string) {
	fs := flag.NewFlagSet("daemon status", flag.ExitOnError)
	path := fs.String("path", ".", "project root")
	fs.Parse(args)

	root, err := filepath.Abs(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	info, err := findDaemonByProject(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if info == nil {
		fmt.Println("no daemon running for this project")
		return
	}

	status := "running"
	if !isProcessAlive(info.PID) {
		status = "dead"
		removeDaemon(info.PID)
	}

	fmt.Printf("PID:      %d\n", info.PID)
	fmt.Printf("Project:  %s\n", info.ProjectRoot)
	fmt.Printf("Status:   %s\n", status)
	fmt.Printf("Started:  %s\n", info.StartedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Interval: %s\n", info.Interval)
	fmt.Printf("Debounce: %s\n", info.Debounce)

	if status == "running" {
		cfg, err := loadConfig(root)
		if err == nil {
			indexer, err := newIndexer(root, cfg)
			if err == nil {
				s := indexer.Status()
				fmt.Printf("Files:    %d\n", s.Files)
				fmt.Printf("Chunks:   %d\n", s.Chunks)
			}
		}
	}
}

func (d *Daemon) Run() {
	defer d.releaseLock()
	removeSelf := func() {
		removeDaemon(os.Getpid())
		lockPath := lockFilePath(d.projectRoot)
		os.Remove(lockPath)
	}
	defer removeSelf()

	if d.verbose {
		fmt.Printf("daemon started for %s (interval: %s, debounce: %s)\n", d.projectRoot, d.interval, d.debounce)
	}

	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	debounceTimer := time.NewTimer(d.debounce)
	defer debounceTimer.Stop()

	debounceTimer.Stop()

	for {
		select {
		case <-d.done:
			if d.verbose {
				fmt.Println("daemon stopping...")
			}
			return
		case <-ticker.C:
			changes := d.scan()
			if len(changes) > 0 {
				d.mu.Lock()
				for path, change := range changes {
					d.pending[path] = change
				}
				pendingCount := len(d.pending)
				d.mu.Unlock()

				if d.verbose && pendingCount > 0 {
					fmt.Printf("detected %d file(s) changed\n", pendingCount)
				}

				if !debounceTimer.Reset(d.debounce) {
					debounceTimer = time.NewTimer(d.debounce)
				}
			}
		case <-debounceTimer.C:
			d.processBatch()
		}
	}
}

func (d *Daemon) Stop() {
	close(d.done)
}

func (d *Daemon) scan() map[string]FileChangeType {
	changes := make(map[string]FileChangeType)

	files, err := walkFiles(d.projectRoot, d.cfg)
	if err != nil {
		return changes
	}

	fileSet := make(map[string]struct{}, len(files))
	for _, f := range files {
		fileSet[f] = struct{}{}
	}

	for _, rel := range files {
		abs := filepath.Join(d.projectRoot, rel)
		info, err := os.Stat(abs)
		if err != nil {
			continue
		}

		prevState, exists := d.indexer.index.Files[rel]
		if !exists {
			changes[rel] = FileModified
			continue
		}

		modNano := info.ModTime().UTC().UnixNano()
		size := info.Size()
		if prevState.Size != size || prevState.ModTimeUnixNano != modNano {
			changes[rel] = FileModified
		}
	}

	for rel := range d.indexer.index.Files {
		if _, exists := fileSet[rel]; !exists {
			changes[rel] = FileDeleted
		}
	}

	return changes
}

func (d *Daemon) processBatch() {
	d.mu.Lock()
	if len(d.pending) == 0 {
		d.mu.Unlock()
		return
	}
	pending := d.pending
	d.pending = make(map[string]FileChangeType)
	d.mu.Unlock()

	var modified []string
	var deleted []string

	for path, change := range pending {
		if change == FileDeleted {
			deleted = append(deleted, path)
		} else {
			modified = append(modified, path)
		}
	}

	if len(deleted) > 0 {
		for _, path := range deleted {
			delete(d.indexer.index.Files, path)
			delete(d.indexer.index.ChunksByFile, path)
			if d.verbose {
				fmt.Printf("[~] %s (deleted)\n", path)
			}
		}
	}

	if len(modified) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		for _, rel := range modified {
			abs := filepath.Join(d.projectRoot, rel)
			data, err := os.ReadFile(abs)
			if err != nil {
				if d.verbose {
					fmt.Printf("[!] %s: %v\n", rel, err)
				}
				continue
			}

			info, err := os.Stat(abs)
			if err != nil {
				continue
			}

			chunks := d.indexer.fileChunks(rel, string(data))
			if len(chunks) == 0 {
				continue
			}

			texts := make([]string, 0, len(chunks))
			for _, ch := range chunks {
				texts = append(texts, ch.Content)
			}

			vecs, err := d.indexer.provider.Embed(ctx, texts)
			if err != nil {
				if d.verbose {
					fmt.Printf("[!] %s: embed failed: %v\n", rel, err)
				}
				continue
			}

			records := make([]ChunkRecord, 0, len(chunks))
			lang := d.indexer.languageFor(rel)
			for idx, ch := range chunks {
				records = append(records, ChunkRecord{
					FilePath:  rel,
					Language:  lang,
					StartLine: ch.StartLine,
					EndLine:   ch.EndLine,
					Embedding: vecs[idx],
					ChunkHash: fileHash([]byte(ch.Content)),
				})
			}

			hash := fileHash(data)
			_, existed := d.indexer.index.Files[rel]
			d.indexer.index.Files[rel] = FileState{
				Hash:            hash,
				ChunkCount:      len(records),
				Size:            info.Size(),
				ModTimeUnixNano: info.ModTime().UTC().UnixNano(),
			}
			d.indexer.index.ChunksByFile[rel] = records

			if d.verbose {
				if !existed {
					fmt.Printf("[+] %s (%d chunks)\n", rel, len(records))
				} else {
					fmt.Printf("[~] %s (%d chunks)\n", rel, len(records))
				}
			}
		}
	}

	if len(modified) > 0 || len(deleted) > 0 {
		if err := saveIndex(indexPath(d.projectRoot), d.indexer.index); err != nil {
			if d.verbose {
				fmt.Printf("[!] failed to save index: %v\n", err)
			}
		}
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return "..." + s[len(s)-maxLen+3:]
}
