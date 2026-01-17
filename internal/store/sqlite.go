package store

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	_ "github.com/mattn/go-sqlite3"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

func init() {
	// Register sqlite-vec extension
	sqlite_vec.Auto()
}

// SQLiteStore implements the Store interface using SQLite and sqlite-vec.
type SQLiteStore struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewSQLiteStore creates a new SQLite store at the given path.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open database with foreign keys enabled
	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on&_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Initialize schema
	if err := initSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	log.Debug("Opened SQLite store", "path", dbPath)

	return &SQLiteStore{db: db}, nil
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// CreateStore creates a new store record.
func (s *SQLiteStore) CreateStore(name, rootPath string, provider EmbeddingProvider, model string, dimensions int) (*StoreRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Ensure vector table exists with correct dimensions
	if err := ensureVectorTable(s.db, dimensions); err != nil {
		return nil, fmt.Errorf("failed to ensure vector table: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.Exec(`
		INSERT INTO stores (name, root_path, embedding_provider, embedding_model, embedding_dimensions, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, name, rootPath, string(provider), model, dimensions, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create store: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get store ID: %w", err)
	}

	createdAt, _ := time.Parse(time.RFC3339, now)
	return &StoreRecord{
		ID:                  id,
		Name:                name,
		RootPath:            rootPath,
		EmbeddingProvider:   provider,
		EmbeddingModel:      model,
		EmbeddingDimensions: dimensions,
		CreatedAt:           createdAt,
		UpdatedAt:           createdAt,
	}, nil
}

// GetStore retrieves a store by name.
func (s *SQLiteStore) GetStore(name string) (*StoreRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var record StoreRecord
	var createdAt, updatedAt string
	var provider string

	err := s.db.QueryRow(`
		SELECT id, name, root_path, embedding_provider, embedding_model, embedding_dimensions, created_at, updated_at
		FROM stores WHERE name = ?
	`, name).Scan(
		&record.ID, &record.Name, &record.RootPath,
		&provider, &record.EmbeddingModel, &record.EmbeddingDimensions,
		&createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get store: %w", err)
	}

	record.EmbeddingProvider = EmbeddingProvider(provider)
	record.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	record.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	return &record, nil
}

// GetStoreByID retrieves a store by ID.
func (s *SQLiteStore) GetStoreByID(id int64) (*StoreRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var record StoreRecord
	var createdAt, updatedAt string
	var provider string

	err := s.db.QueryRow(`
		SELECT id, name, root_path, embedding_provider, embedding_model, embedding_dimensions, created_at, updated_at
		FROM stores WHERE id = ?
	`, id).Scan(
		&record.ID, &record.Name, &record.RootPath,
		&provider, &record.EmbeddingModel, &record.EmbeddingDimensions,
		&createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get store: %w", err)
	}

	record.EmbeddingProvider = EmbeddingProvider(provider)
	record.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	record.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	return &record, nil
}

// DeleteStore deletes a store and all its files/chunks.
func (s *SQLiteStore) DeleteStore(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get store ID first
	var storeID int64
	err := s.db.QueryRow("SELECT id FROM stores WHERE name = ?", name).Scan(&storeID)
	if err == sql.ErrNoRows {
		return nil // Store doesn't exist
	}
	if err != nil {
		return fmt.Errorf("failed to get store ID: %w", err)
	}

	// Delete vectors for this store's chunks
	_, err = s.db.Exec(`
		DELETE FROM chunk_vectors WHERE chunk_id IN (
			SELECT c.id FROM chunks c
			JOIN files f ON f.id = c.file_id
			WHERE f.store_id = ?
		)
	`, storeID)
	if err != nil {
		return fmt.Errorf("failed to delete vectors: %w", err)
	}

	// Delete store (cascades to files and chunks)
	_, err = s.db.Exec("DELETE FROM stores WHERE id = ?", storeID)
	if err != nil {
		return fmt.Errorf("failed to delete store: %w", err)
	}

	return nil
}

// ListStores returns all stores.
func (s *SQLiteStore) ListStores() ([]StoreRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, name, root_path, embedding_provider, embedding_model, embedding_dimensions, created_at, updated_at
		FROM stores ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list stores: %w", err)
	}
	defer rows.Close()

	var stores []StoreRecord
	for rows.Next() {
		var record StoreRecord
		var createdAt, updatedAt string
		var provider string

		if err := rows.Scan(
			&record.ID, &record.Name, &record.RootPath,
			&provider, &record.EmbeddingModel, &record.EmbeddingDimensions,
			&createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan store: %w", err)
		}

		record.EmbeddingProvider = EmbeddingProvider(provider)
		record.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		record.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

		stores = append(stores, record)
	}

	return stores, rows.Err()
}

// UpdateStoreTimestamp updates the store's updated_at timestamp.
func (s *SQLiteStore) UpdateStoreTimestamp(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec("UPDATE stores SET updated_at = ? WHERE id = ?", now, id)
	return err
}

// UpsertFile inserts or updates a file with its chunks and embeddings.
func (s *SQLiteStore) UpsertFile(storeID int64, file FileInput, chunks []Chunk, embeddings [][]float32) error {
	if len(chunks) != len(embeddings) {
		return fmt.Errorf("chunks and embeddings count mismatch: %d != %d", len(chunks), len(embeddings))
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Check if file exists
	var existingFileID int64
	err = tx.QueryRow("SELECT id FROM files WHERE store_id = ? AND external_id = ?", storeID, file.ExternalID).Scan(&existingFileID)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to check existing file: %w", err)
	}

	// If file exists, delete old chunks and vectors
	if existingFileID > 0 {
		// Delete vectors for old chunks
		_, err = tx.Exec("DELETE FROM chunk_vectors WHERE chunk_id IN (SELECT id FROM chunks WHERE file_id = ?)", existingFileID)
		if err != nil {
			return fmt.Errorf("failed to delete old vectors: %w", err)
		}

		// Delete old chunks
		_, err = tx.Exec("DELETE FROM chunks WHERE file_id = ?", existingFileID)
		if err != nil {
			return fmt.Errorf("failed to delete old chunks: %w", err)
		}

		// Update file record
		now := time.Now().UTC().Format(time.RFC3339)
		_, err = tx.Exec(`
			UPDATE files SET path = ?, relative_path = ?, hash = ?, file_size = ?, indexed_at = ?
			WHERE id = ?
		`, file.Path, file.RelativePath, file.Hash, file.FileSize, now, existingFileID)
		if err != nil {
			return fmt.Errorf("failed to update file: %w", err)
		}
	} else {
		// Insert new file
		now := time.Now().UTC().Format(time.RFC3339)
		result, err := tx.Exec(`
			INSERT INTO files (store_id, external_id, path, relative_path, hash, file_size, indexed_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, storeID, file.ExternalID, file.Path, file.RelativePath, file.Hash, file.FileSize, now)
		if err != nil {
			return fmt.Errorf("failed to insert file: %w", err)
		}
		existingFileID, _ = result.LastInsertId()
	}

	// Insert chunks and vectors
	for i, chunk := range chunks {
		// Insert chunk
		result, err := tx.Exec(`
			INSERT INTO chunks (file_id, chunk_index, content, start_line, end_line)
			VALUES (?, ?, ?, ?, ?)
		`, existingFileID, chunk.ChunkIndex, chunk.Content, chunk.StartLine, chunk.EndLine)
		if err != nil {
			return fmt.Errorf("failed to insert chunk %d: %w", i, err)
		}

		chunkID, _ := result.LastInsertId()

		// Insert vector
		embeddingBlob := serializeEmbedding(embeddings[i])
		_, err = tx.Exec(`
			INSERT INTO chunk_vectors (chunk_id, embedding)
			VALUES (?, ?)
		`, chunkID, embeddingBlob)
		if err != nil {
			return fmt.Errorf("failed to insert vector for chunk %d: %w", i, err)
		}
	}

	return tx.Commit()
}

// DeleteFile deletes a file and its chunks/vectors.
func (s *SQLiteStore) DeleteFile(storeID int64, externalID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get file ID
	var fileID int64
	err := s.db.QueryRow("SELECT id FROM files WHERE store_id = ? AND external_id = ?", storeID, externalID).Scan(&fileID)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get file ID: %w", err)
	}

	// Delete vectors
	_, err = s.db.Exec("DELETE FROM chunk_vectors WHERE chunk_id IN (SELECT id FROM chunks WHERE file_id = ?)", fileID)
	if err != nil {
		return fmt.Errorf("failed to delete vectors: %w", err)
	}

	// Delete file (cascades to chunks)
	_, err = s.db.Exec("DELETE FROM files WHERE id = ?", fileID)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// GetFileByExternalID retrieves a file by its external ID.
func (s *SQLiteStore) GetFileByExternalID(storeID int64, externalID string) (*FileRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var record FileRecord
	var indexedAt string

	err := s.db.QueryRow(`
		SELECT id, store_id, external_id, path, relative_path, hash, file_size, indexed_at
		FROM files WHERE store_id = ? AND external_id = ?
	`, storeID, externalID).Scan(
		&record.ID, &record.StoreID, &record.ExternalID,
		&record.Path, &record.RelativePath, &record.Hash,
		&record.FileSize, &indexedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get file: %w", err)
	}

	record.IndexedAt, _ = time.Parse(time.RFC3339, indexedAt)
	return &record, nil
}

// GetFileByHash retrieves a file by its content hash.
func (s *SQLiteStore) GetFileByHash(storeID int64, hash string) (*FileRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var record FileRecord
	var indexedAt string

	err := s.db.QueryRow(`
		SELECT id, store_id, external_id, path, relative_path, hash, file_size, indexed_at
		FROM files WHERE store_id = ? AND hash = ?
	`, storeID, hash).Scan(
		&record.ID, &record.StoreID, &record.ExternalID,
		&record.Path, &record.RelativePath, &record.Hash,
		&record.FileSize, &indexedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get file by hash: %w", err)
	}

	record.IndexedAt, _ = time.Parse(time.RFC3339, indexedAt)
	return &record, nil
}

// ListFiles returns files for a store.
func (s *SQLiteStore) ListFiles(storeID int64, opts *ListFilesOptions) ([]FileRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT id, store_id, external_id, path, relative_path, hash, file_size, indexed_at
		FROM files WHERE store_id = ? ORDER BY relative_path
	`

	if opts != nil && opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", opts.Limit)
		if opts.Offset > 0 {
			query += fmt.Sprintf(" OFFSET %d", opts.Offset)
		}
	}

	rows, err := s.db.Query(query, storeID)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}
	defer rows.Close()

	var files []FileRecord
	for rows.Next() {
		var record FileRecord
		var indexedAt string

		if err := rows.Scan(
			&record.ID, &record.StoreID, &record.ExternalID,
			&record.Path, &record.RelativePath, &record.Hash,
			&record.FileSize, &indexedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan file: %w", err)
		}

		record.IndexedAt, _ = time.Parse(time.RFC3339, indexedAt)
		files = append(files, record)
	}

	return files, rows.Err()
}

// Search performs a vector similarity search.
func (s *SQLiteStore) Search(storeID int64, queryEmbedding []float32, topK int) ([]SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Serialize the query embedding
	queryBlob := serializeEmbedding(queryEmbedding)

	// Perform vector search using sqlite-vec
	// Note: sqlite-vec filters happen AFTER k results are selected from the vector index.
	// To ensure we get topK results after filtering by store_id, we request more from
	// the vector index (topK * 10) and let the SQL LIMIT clause enforce the final count.
	kForVec := topK * 10
	if kForVec > 1000 {
		kForVec = 1000
	}
	rows, err := s.db.Query(`
		SELECT 
			c.id, c.file_id, c.chunk_index, c.content, c.start_line, c.end_line,
			f.id, f.store_id, f.external_id, f.path, f.relative_path, f.hash, f.file_size, f.indexed_at,
			cv.distance
		FROM chunk_vectors cv
		JOIN chunks c ON c.id = cv.chunk_id
		JOIN files f ON f.id = c.file_id
		WHERE f.store_id = ?
			AND cv.embedding MATCH ?
			AND k = ?
		ORDER BY cv.distance ASC
		LIMIT ?
	`, storeID, queryBlob, kForVec, topK)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var result SearchResult
		var indexedAt string

		if err := rows.Scan(
			&result.Chunk.ID, &result.Chunk.FileID, &result.Chunk.ChunkIndex,
			&result.Chunk.Content, &result.Chunk.StartLine, &result.Chunk.EndLine,
			&result.File.ID, &result.File.StoreID, &result.File.ExternalID,
			&result.File.Path, &result.File.RelativePath, &result.File.Hash,
			&result.File.FileSize, &indexedAt,
			&result.Distance,
		); err != nil {
			return nil, fmt.Errorf("failed to scan search result: %w", err)
		}

		result.File.IndexedAt, _ = time.Parse(time.RFC3339, indexedAt)
		result.Score = 1 - result.Distance // Convert distance to similarity

		results = append(results, result)
	}

	return results, rows.Err()
}

// GetStats returns statistics for a store.
func (s *SQLiteStore) GetStats(storeID int64) (*StoreStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var stats StoreStats
	stats.StoreID = storeID

	// Get store name
	err := s.db.QueryRow("SELECT name FROM stores WHERE id = ?", storeID).Scan(&stats.StoreName)
	if err != nil {
		return nil, fmt.Errorf("failed to get store name: %w", err)
	}

	// Get file count and total size
	err = s.db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(file_size), 0)
		FROM files WHERE store_id = ?
	`, storeID).Scan(&stats.FileCount, &stats.TotalSize)
	if err != nil {
		return nil, fmt.Errorf("failed to get file stats: %w", err)
	}

	// Get chunk count
	err = s.db.QueryRow(`
		SELECT COUNT(*) FROM chunks c
		JOIN files f ON f.id = c.file_id
		WHERE f.store_id = ?
	`, storeID).Scan(&stats.ChunkCount)
	if err != nil {
		return nil, fmt.Errorf("failed to get chunk count: %w", err)
	}

	return &stats, nil
}

// ClearStore removes all files and chunks from a store.
func (s *SQLiteStore) ClearStore(storeID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Delete vectors
	_, err := s.db.Exec(`
		DELETE FROM chunk_vectors WHERE chunk_id IN (
			SELECT c.id FROM chunks c
			JOIN files f ON f.id = c.file_id
			WHERE f.store_id = ?
		)
	`, storeID)
	if err != nil {
		return fmt.Errorf("failed to delete vectors: %w", err)
	}

	// Delete files (cascades to chunks)
	_, err = s.db.Exec("DELETE FROM files WHERE store_id = ?", storeID)
	if err != nil {
		return fmt.Errorf("failed to delete files: %w", err)
	}

	return nil
}

// serializeEmbedding converts a float32 slice to bytes for sqlite-vec.
func serializeEmbedding(embedding []float32) []byte {
	buf := make([]byte, len(embedding)*4)
	for i, v := range embedding {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}
