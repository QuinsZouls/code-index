package main

import "testing"

func TestChunkTextSingleChunk(t *testing.T) {
	got := chunkText("a\nb\nc", 10, 2)
	if len(got) != 1 {
		t.Fatalf("len(chunkText) = %d, want 1", len(got))
	}
	if got[0].StartLine != 1 || got[0].EndLine != 3 || got[0].Content != "a\nb\nc" {
		t.Fatalf("unexpected chunk: %#v", got[0])
	}
}

func TestChunkTextOverlap(t *testing.T) {
	got := chunkText("1\n2\n3\n4\n5", 3, 1)
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
