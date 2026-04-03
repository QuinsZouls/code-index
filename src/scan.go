package main

import (
	"crypto/sha256"
	"encoding/hex"
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

func collectGitignorePatterns(dirAbs, dirRel string) []string {
	path := filepath.Join(dirAbs, ".gitignore")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var patterns []string
	for _, line := range strings.Split(string(data), "\n") {
		if compiled := compileGitignorePattern(dirRel, line); compiled != "" {
			patterns = append(patterns, compiled)
		}
	}
	return patterns
}

func compileGitignorePattern(dirRel, line string) string {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
		return ""
	}
	anchored := strings.HasPrefix(line, "/")
	if anchored {
		line = strings.TrimPrefix(line, "/")
	}
	dirPattern := strings.HasSuffix(line, "/")
	if dirPattern {
		line = strings.TrimSuffix(line, "/")
	}
	if line == "" {
		return ""
	}
	dirRel = filepathToSlash(dirRel)
	prefix := ""
	if dirRel != "" {
		prefix = dirRel + "/"
	}
	if dirPattern {
		if anchored {
			return prefix + line + "/**"
		}
		if strings.Contains(line, "/") {
			return prefix + line + "/**"
		}
		return prefix + "**/" + line + "/**"
	}
	if anchored || strings.Contains(line, "/") {
		return prefix + line
	}
	return prefix + "**/" + line
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
	var files []string
	if err := walkFilesDir(projectRoot, projectRoot, "", cfg, nil, &files); err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func walkFilesDir(projectRoot, dirAbs, dirRel string, cfg Config, inheritedPatterns []string, files *[]string) error {
	patterns := append([]string{}, inheritedPatterns...)
	patterns = append(patterns, collectGitignorePatterns(dirAbs, dirRel)...)
	entries, err := os.ReadDir(dirAbs)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == ".gitignore" {
			continue
		}
		rel := name
		if dirRel != "" {
			rel = filepath.Join(dirRel, name)
		}
		if entry.IsDir() {
			if shouldExclude(rel, true, cfg, patterns) {
				continue
			}
			if err := walkFilesDir(projectRoot, filepath.Join(dirAbs, name), rel, cfg, patterns, files); err != nil {
				return err
			}
			continue
		}
		if shouldExclude(rel, false, cfg, patterns) {
			continue
		}
		if !shouldInclude(rel, cfg) {
			continue
		}
		*files = append(*files, rel)
	}
	return nil
}
