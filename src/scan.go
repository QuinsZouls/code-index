package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var ignoredDirs = map[string]struct{}{
	".git":         {},
	".codeindex":   {},
	"node_modules": {},
	"dist":         {},
	"build":        {},
	"target":       {},
	"vendor":       {},
}

func collectGitignorePatterns(projectRoot string) []string {
	path := filepath.Join(projectRoot, ".gitignore")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var patterns []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		if strings.HasPrefix(line, "/") {
			line = strings.TrimPrefix(line, "/")
		}
		if strings.HasSuffix(line, "/") {
			line = strings.TrimSuffix(line, "/") + "/**"
		}
		patterns = append(patterns, line)
	}
	return patterns
}

func shouldExclude(relPath string, isDir bool, cfg Config, gitignorePatterns []string) bool {
	relPath = filepathToSlash(relPath)
	base := filepath.Base(relPath)
	if isDir {
		if _, ok := ignoredDirs[base]; ok {
			return true
		}
	}
	for _, pattern := range append(append([]string{}, cfg.ExcludePatterns...), gitignorePatterns...) {
		if matchPattern(pattern, relPath) {
			return true
		}
	}
	return false
}

func shouldInclude(relPath string, cfg Config) bool {
	if len(cfg.IncludePatterns) == 0 {
		return true
	}
	for _, pattern := range cfg.IncludePatterns {
		if matchPattern(pattern, relPath) {
			return true
		}
	}
	return false
}

func fileHash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func walkFiles(projectRoot string, cfg Config) ([]string, error) {
	gitignorePatterns := collectGitignorePatterns(projectRoot)
	var files []string
	err := filepath.WalkDir(projectRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == projectRoot {
			return nil
		}
		rel, err := filepath.Rel(projectRoot, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			if shouldExclude(rel, true, cfg, gitignorePatterns) {
				return fs.SkipDir
			}
			return nil
		}
		if shouldExclude(rel, false, cfg, gitignorePatterns) {
			return nil
		}
		if !shouldInclude(rel, cfg) {
			return nil
		}
		files = append(files, rel)
		return nil
	})
	sort.Strings(files)
	return files, err
}
