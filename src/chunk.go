package main

import "strings"

func chunkText(text string, maxLines, overlap int) []Chunk {
	if maxLines <= 0 {
		maxLines = 120
	}
	if overlap < 0 {
		overlap = 0
	}
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return nil
	}
	if len(lines) <= maxLines {
		return []Chunk{{Content: text, StartLine: 1, EndLine: len(lines)}}
	}
	step := maxLines - overlap
	if step <= 0 {
		step = maxLines
	}
	chunks := make([]Chunk, 0, (len(lines)/step)+1)
	for start := 0; start < len(lines); start += step {
		end := start + maxLines
		if end > len(lines) {
			end = len(lines)
		}
		chunkLines := lines[start:end]
		if len(chunkLines) == 0 {
			break
		}
		content := strings.Join(chunkLines, "\n")
		chunks = append(chunks, Chunk{Content: content, StartLine: start + 1, EndLine: end})
		if end == len(lines) {
			break
		}
	}
	return chunks
}
