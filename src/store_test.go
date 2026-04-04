package main

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestIndexStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index.gob")
	original := &IndexData{
		Version:            1,
		EmbeddingSignature: "sig",
		Files: map[string]FileState{
			"a.go": {Hash: "hash", ChunkCount: 1},
		},
		ChunksByFile: map[string][]ChunkRecord{
			"a.go": {
				{
					FilePath:  "a.go",
					Language:  "go",
					StartLine: 1,
					EndLine:   1,
					Embedding: []float32{1, 2},
					ChunkHash: "chunk",
				},
			},
		},
	}
	if err := saveIndex(path, original); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadIndex(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil {
		t.Fatal("expected loaded index")
	}
	if loaded.Version != original.Version || loaded.EmbeddingSignature != original.EmbeddingSignature {
		t.Fatalf("loaded header = %#v", loaded)
	}
	if !reflect.DeepEqual(loaded.Files, original.Files) {
		t.Fatalf("Files = %#v, want %#v", loaded.Files, original.Files)
	}
	if got := loaded.ChunksByFile["a.go"][0]; got.FilePath != "a.go" || got.Language != "go" || got.StartLine != 1 || got.EndLine != 1 || got.ChunkHash != "chunk" {
		t.Fatalf("loaded chunk = %#v", got)
	}
}

func TestLoadIndexMissingReturnsNil(t *testing.T) {
	loaded, err := loadIndex(filepath.Join(t.TempDir(), "missing.gob"))
	if err != nil {
		t.Fatal(err)
	}
	if loaded != nil {
		t.Fatalf("expected nil, got %#v", loaded)
	}
}
