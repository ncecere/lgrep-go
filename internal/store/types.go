// Package store provides vector storage and retrieval using SQLite and sqlite-vec.
package store

import "time"

// EmbeddingProvider represents the provider used for embeddings.
type EmbeddingProvider string

const (
	ProviderOllama EmbeddingProvider = "ollama"
	ProviderOpenAI EmbeddingProvider = "openai"
)

// StoreRecord represents a stored index (a project/directory that has been indexed).
type StoreRecord struct {
	ID                  int64             `json:"id"`
	Name                string            `json:"name"`
	RootPath            string            `json:"root_path"`
	EmbeddingProvider   EmbeddingProvider `json:"embedding_provider"`
	EmbeddingModel      string            `json:"embedding_model"`
	EmbeddingDimensions int               `json:"embedding_dimensions"`
	CreatedAt           time.Time         `json:"created_at"`
	UpdatedAt           time.Time         `json:"updated_at"`
}

// FileRecord represents an indexed file.
type FileRecord struct {
	ID           int64     `json:"id"`
	StoreID      int64     `json:"store_id"`
	ExternalID   string    `json:"external_id"`   // Relative path used as identifier
	Path         string    `json:"path"`          // Absolute path
	RelativePath string    `json:"relative_path"` // Relative path from store root
	Hash         string    `json:"hash"`          // Content hash (xxh64:...)
	FileSize     int64     `json:"file_size"`
	IndexedAt    time.Time `json:"indexed_at"`
}

// ChunkRecord represents a chunk of a file.
type ChunkRecord struct {
	ID         int64  `json:"id"`
	FileID     int64  `json:"file_id"`
	ChunkIndex int    `json:"chunk_index"`
	Content    string `json:"content"`
	StartLine  int    `json:"start_line"` // 1-indexed
	EndLine    int    `json:"end_line"`   // 1-indexed
}

// Chunk represents a chunk to be stored (input for upsert).
type Chunk struct {
	Content    string `json:"content"`
	StartLine  int    `json:"start_line"`
	EndLine    int    `json:"end_line"`
	ChunkIndex int    `json:"chunk_index"`
}

// FileInput represents file data for upserting.
type FileInput struct {
	ExternalID   string `json:"external_id"`
	Path         string `json:"path"`
	RelativePath string `json:"relative_path"`
	Hash         string `json:"hash"`
	FileSize     int64  `json:"file_size"`
}

// SearchResult represents a search result with chunk, file, and similarity score.
type SearchResult struct {
	Chunk    ChunkRecord `json:"chunk"`
	File     FileRecord  `json:"file"`
	Distance float64     `json:"distance"` // Cosine distance from sqlite-vec
	Score    float64     `json:"score"`    // 1 - distance (similarity)
}

// StoreStats contains statistics about a store.
type StoreStats struct {
	StoreID    int64  `json:"store_id"`
	StoreName  string `json:"store_name"`
	FileCount  int    `json:"file_count"`
	ChunkCount int    `json:"chunk_count"`
	TotalSize  int64  `json:"total_size"` // Total file size in bytes
}

// ListFilesOptions contains options for listing files.
type ListFilesOptions struct {
	Limit  int
	Offset int
}
