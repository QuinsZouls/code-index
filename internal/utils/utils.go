package utils

import (
	"os"
	"path"
	"strings"
)

// ReadChunkContent reads file content by line range
func ReadChunkContent(projectRoot, relPath string, startLine, endLine int) string {
	absPath := projectRoot + "/" + relPath
	data, err := os.ReadFile(absPath)
	if err != nil {
		return "[file unavailable: " + err.Error() + "]"
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

// MatchPattern matches a pattern against a path
func MatchPattern(pattern, relPath string) bool {
	pattern = strings.TrimSpace(pattern)
	pattern = strings.ReplaceAll(pattern, "\\", "/")
	relPath = strings.ReplaceAll(relPath, "\\", "/")
	if pattern == "" {
		return false
	}
	return MatchSegments(strings.Split(pattern, "/"), strings.Split(relPath, "/"))
}

// MatchSegments matches path segments
func MatchSegments(patternSegs, pathSegs []string) bool {
	if len(patternSegs) == 0 {
		return len(pathSegs) == 0
	}
	if patternSegs[0] == "**" {
		if len(patternSegs) == 1 {
			return true
		}
		for i := 0; i <= len(pathSegs); i++ {
			if MatchSegments(patternSegs[1:], pathSegs[i:]) {
				return true
			}
		}
		return false
	}
	if len(pathSegs) == 0 {
		return false
	}
	ok, err := path.Match(patternSegs[0], pathSegs[0])
	if err != nil || !ok {
		return false
	}
	return MatchSegments(patternSegs[1:], pathSegs[1:])
}

// FilepathToSlash converts Windows path separators to Unix separators
func FilepathToSlash(s string) string {
	return strings.ReplaceAll(s, "\\", "/")
}
