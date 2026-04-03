package main

type Chunk struct {
	Content   string
	StartLine int
	EndLine   int
}

type ChunkRecord struct {
	FilePath  string    `json:"file_path"`
	Language  string    `json:"language"`
	Content   string    `json:"content"`
	StartLine int       `json:"start_line"`
	EndLine   int       `json:"end_line"`
	Score     float64   `json:"score,omitempty"`
	Embedding []float32 `json:"-"`
	ChunkHash string    `json:"chunk_hash"`
}

type FileState struct {
	Hash            string `json:"hash"`
	ChunkCount      int    `json:"chunk_count"`
	Size            int64  `json:"size"`
	ModTimeUnixNano int64  `json:"mod_time_unix_nano"`
}

type SearchResult struct {
	FilePath  string
	Language  string
	Content   string
	StartLine int
	EndLine   int
	Score     float64
}

type Status struct {
	Files    int
	Chunks   int
	Langs    map[string]int
	Indexing bool
}

type IndexProgress struct {
	Current int
	Total   int
	File    string
	Action  string
	Kind    string
	Chunks  int
}
