package index

import (
	"encoding/gob"
	"errors"
	"os"
	"path/filepath"

	"github.com/QuinsZouls/code-index/internal/types"
)

type IndexData struct {
	Version            int                            `json:"version"`
	EmbeddingSignature string                         `json:"embedding_signature"`
	Files              map[string]types.FileState     `json:"files"`
	ChunksByFile       map[string][]types.ChunkRecord `json:"chunks_by_file"`
}

func newIndexData(signature string) *IndexData {
	return &IndexData{
		Version:            1,
		EmbeddingSignature: signature,
		Files:              map[string]types.FileState{},
		ChunksByFile:       map[string][]types.ChunkRecord{},
	}
}

func loadIndex(path string) (*IndexData, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()
	var data IndexData
	if err := gob.NewDecoder(file).Decode(&data); err != nil {
		return nil, err
	}
	if data.Files == nil {
		data.Files = map[string]types.FileState{}
	}
	if data.ChunksByFile == nil {
		data.ChunksByFile = map[string][]types.ChunkRecord{}
	}
	if data.Version != 1 {
		return nil, nil
	}
	return &data, nil
}

func SaveIndex(path string, data *IndexData) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	enc := gob.NewEncoder(file)
	return enc.Encode(data)
}

func saveIndex(path string, data *IndexData) error {
	return SaveIndex(path, data)
}
