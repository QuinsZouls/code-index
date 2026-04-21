package main

import "strings"

func chunkText(text string, maxLines, overlap, contextSize int) []Chunk {
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

	// If contextSize is set, check if we need to limit by character count
	// This handles embedding models with small context windows (e.g., 512 tokens)
	if contextSize > 0 && len(text) > contextSize {
		return chunkByContextSize(text, contextSize, overlap)
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

// chunkByContextSize splits text into chunks that fit within contextSize characters,
// respecting line boundaries when possible.
func chunkByContextSize(text string, contextSize, overlap int) []Chunk {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return nil
	}

	var chunks []Chunk
	startLine := 0

	for startLine < len(lines) {
		chunkStart := startLine
		currentSize := 0
		endLine := chunkStart

		// Build chunk line by line until we hit contextSize
		for endLine < len(lines) {
			lineLen := len(lines[endLine]) + 1 // +1 for newline
			if currentSize+lineLen > contextSize && currentSize > 0 {
				break
			}
			currentSize += lineLen
			endLine++
		}

		// Ensure at least one line per chunk (even if it exceeds contextSize)
		if endLine == chunkStart {
			endLine = chunkStart + 1
		}

		content := strings.Join(lines[chunkStart:endLine], "\n")
		chunks = append(chunks, Chunk{
			Content:   content,
			StartLine: chunkStart + 1,
			EndLine:   endLine,
		})

		// Move start forward, accounting for overlap
		step := (endLine - chunkStart) - overlap
		if step <= 0 {
			step = 1 // Ensure progress
		}
		startLine = chunkStart + step
	}

	return chunks
}
