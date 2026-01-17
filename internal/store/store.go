package store

// Store defines the interface for vector storage operations.
type Store interface {
	// Store management
	CreateStore(name, rootPath string, provider EmbeddingProvider, model string, dimensions int) (*StoreRecord, error)
	GetStore(name string) (*StoreRecord, error)
	GetStoreByID(id int64) (*StoreRecord, error)
	DeleteStore(name string) error
	ListStores() ([]StoreRecord, error)
	UpdateStoreTimestamp(id int64) error

	// File operations
	UpsertFile(storeID int64, file FileInput, chunks []Chunk, embeddings [][]float32) error
	DeleteFile(storeID int64, externalID string) error
	GetFileByExternalID(storeID int64, externalID string) (*FileRecord, error)
	GetFileByHash(storeID int64, hash string) (*FileRecord, error)
	ListFiles(storeID int64, opts *ListFilesOptions) ([]FileRecord, error)

	// Search
	Search(storeID int64, queryEmbedding []float32, topK int) ([]SearchResult, error)

	// Stats
	GetStats(storeID int64) (*StoreStats, error)

	// Maintenance
	ClearStore(storeID int64) error
	Close() error
}
