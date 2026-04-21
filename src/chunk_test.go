package main

import "testing"

func TestChunkTextSingleChunk(t *testing.T) {
	got := chunkText("a\nb\nc", 10, 2, 0)
	if len(got) != 1 {
		t.Fatalf("len(chunkText) = %d, want 1", len(got))
	}
	if got[0].StartLine != 1 || got[0].EndLine != 3 || got[0].Content != "a\nb\nc" {
		t.Fatalf("unexpected chunk: %#v", got[0])
	}
}

func TestChunkTextOverlap(t *testing.T) {
	got := chunkText("1\n2\n3\n4\n5", 3, 1, 0)
	if len(got) != 2 {
		t.Fatalf("len(chunkText) = %d, want 2", len(got))
	}
	if got[0].StartLine != 1 || got[0].EndLine != 3 || got[0].Content != "1\n2\n3" {
		t.Fatalf("unexpected first chunk: %#v", got[0])
	}
	if got[1].StartLine != 3 || got[1].EndLine != 5 || got[1].Content != "3\n4\n5" {
		t.Fatalf("unexpected second chunk: %#v", got[1])
	}
}

func TestChunkTextContextSize(t *testing.T) {
	// Test with contextSize limiting
	text := "line1\nline2\nline3\nline4\nline5"
	// Each line is ~6 chars, with contextSize=15 we should get multiple chunks
	got := chunkText(text, 100, 0, 15)

	if len(got) < 2 {
		t.Fatalf("len(chunkText) = %d, want at least 2 chunks due to contextSize limit", len(got))
	}

	// Verify each chunk respects contextSize
	for i, chunk := range got {
		if len(chunk.Content) > 16 { // Allow small margin for final newline
			t.Errorf("chunk[%d] content length %d exceeds contextSize+margin", i, len(chunk.Content))
		}
	}
}

func TestChunkTextContextSizeRespectsLines(t *testing.T) {
	// Test that we don't split in the middle of a line
	text := "1234567890\n1234567890\n1234567890" // 3 lines, 10 chars each
	got := chunkText(text, 100, 0, 15)

	if len(got) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(got))
	}

	// Each chunk should be exactly one line (11 chars with newline)
	for i, chunk := range got {
		if chunk.StartLine != i+1 || chunk.EndLine != i+1 {
			t.Errorf("chunk[%d] has wrong line numbers: %d-%d", i, chunk.StartLine, chunk.EndLine)
		}
	}
}

func TestChunkTextContextSizeWithOverlap(t *testing.T) {
	text := "aaaa\nbbbb\ncccc\ndddd\neeee"
	// 5 chars per line, contextSize=12 should fit ~2 lines per chunk
	got := chunkText(text, 100, 1, 12)

	if len(got) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(got))
	}

	// Verify overlap is applied
	// With overlap=1, consecutive chunks should share at least one line
	for i := 1; i < len(got); i++ {
		prev := got[i-1]
		curr := got[i]
		if prev.EndLine < curr.StartLine {
			t.Errorf("no overlap between chunk %d and %d", i-1, i)
		}
	}
}

func TestChunkTextContextSizeZeroNoLimit(t *testing.T) {
	// contextSize=0 should mean no limit by character count
	longText := ""
	for i := 0; i < 100; i++ {
		longText += "line\n"
	}
	// With contextSize=0, it should use maxLines only
	got := chunkText(longText, 50, 0, 0)

	// 100 lines + 1 empty at end = 101 elements after split
	// With maxLines=50 and overlap=0, we get 3 chunks (50 + 50 + 1)
	if len(got) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(got))
	}
}

func TestChunkTextContextSizeOneLongLine(t *testing.T) {
	// Single line longer than contextSize should still be one chunk
	text := "this is a very long line that exceeds the context size limit"
	got := chunkText(text, 100, 0, 10)

	if len(got) != 1 {
		t.Fatalf("expected 1 chunk for single line, got %d", len(got))
	}
}

func TestChunkByContextSize(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		contextSize int
		overlap     int
		wantChunks  int
	}{
		{
			name:        "small text fits in one chunk",
			text:        "a\nb\nc",
			contextSize: 100,
			overlap:     0,
			wantChunks:  1,
		},
		{
			name:        "text needs splitting",
			text:        "line1\nline2\nline3\nline4\nline5",
			contextSize: 10,
			overlap:     0,
			wantChunks:  5, // Each line is ~6 chars
		},
		{
			name:        "overlap increases chunk count",
			text:        "a\nb\nc\nd\ne",
			contextSize: 5,
			overlap:     1,
			wantChunks:  5, // Each line is ~2 chars, but with overlap we still get 5 chunks
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := chunkByContextSize(tt.text, tt.contextSize, tt.overlap)
			if len(got) != tt.wantChunks {
				t.Errorf("got %d chunks, want %d", len(got), tt.wantChunks)
			}
		})
	}
}