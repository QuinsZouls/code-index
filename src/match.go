package main

import (
	"path"
	"strings"
)

func matchPattern(pattern, relPath string) bool {
	pattern = strings.TrimSpace(filepathToSlash(pattern))
	relPath = filepathToSlash(relPath)
	if pattern == "" {
		return false
	}
	return matchSegments(strings.Split(pattern, "/"), strings.Split(relPath, "/"))
}

func matchSegments(patternSegs, pathSegs []string) bool {
	if len(patternSegs) == 0 {
		return len(pathSegs) == 0
	}
	if patternSegs[0] == "**" {
		if len(patternSegs) == 1 {
			return true
		}
		for i := 0; i <= len(pathSegs); i++ {
			if matchSegments(patternSegs[1:], pathSegs[i:]) {
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
	return matchSegments(patternSegs[1:], pathSegs[1:])
}

func filepathToSlash(s string) string {
	return strings.ReplaceAll(s, "\\", "/")
}
