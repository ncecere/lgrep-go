package store

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSQLiteStore(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewSQLiteStore(dbPath)
	require.NoError(t, err)
	defer store.Close()

	// Verify database file was created
	_, err = os.Stat(dbPath)
	assert.NoError(t, err)
}

func TestStoreCreateAndGet(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	// Create a store
	created, err := store.CreateStore("test-project", "/path/to/project", ProviderOllama, "nomic-embed-text", 768)
	require.NoError(t, err)
	assert.Equal(t, "test-project", created.Name)
	assert.Equal(t, "/path/to/project", created.RootPath)
	assert.Equal(t, ProviderOllama, created.EmbeddingProvider)
	assert.Equal(t, "nomic-embed-text", created.EmbeddingModel)
	assert.Equal(t, 768, created.EmbeddingDimensions)

	// Get the store
	retrieved, err := store.GetStore("test-project")
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, created.ID, retrieved.ID)
	assert.Equal(t, created.Name, retrieved.Name)

	// Get by ID
	retrievedByID, err := store.GetStoreByID(created.ID)
	require.NoError(t, err)
	require.NotNil(t, retrievedByID)
	assert.Equal(t, created.ID, retrievedByID.ID)

	// Get non-existent store
	notFound, err := store.GetStore("non-existent")
	require.NoError(t, err)
	assert.Nil(t, notFound)
}

func TestStoreList(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	// Create multiple stores
	_, err := store.CreateStore("project-a", "/path/a", ProviderOllama, "model", 768)
	require.NoError(t, err)
	_, err = store.CreateStore("project-b", "/path/b", ProviderOpenAI, "model", 1536)
	require.NoError(t, err)

	// List stores
	stores, err := store.ListStores()
	require.NoError(t, err)
	assert.Len(t, stores, 2)

	// Should be sorted by name
	assert.Equal(t, "project-a", stores[0].Name)
	assert.Equal(t, "project-b", stores[1].Name)
}

func TestStoreDelete(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	// Create a store
	_, err := store.CreateStore("to-delete", "/path", ProviderOllama, "model", 768)
	require.NoError(t, err)

	// Delete it
	err = store.DeleteStore("to-delete")
	require.NoError(t, err)

	// Verify it's gone
	deleted, err := store.GetStore("to-delete")
	require.NoError(t, err)
	assert.Nil(t, deleted)

	// Delete non-existent should not error
	err = store.DeleteStore("non-existent")
	require.NoError(t, err)
}

func TestFileUpsertAndGet(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	// Create a store
	storeRecord, err := store.CreateStore("test", "/path", ProviderOllama, "model", 4)
	require.NoError(t, err)

	// Insert a file with chunks
	file := FileInput{
		ExternalID:   "src/main.go",
		Path:         "/path/src/main.go",
		RelativePath: "src/main.go",
		Hash:         "xxh64:1234567890abcdef",
		FileSize:     1024,
	}
	chunks := []Chunk{
		{Content: "package main", StartLine: 1, EndLine: 5, ChunkIndex: 0},
		{Content: "func main() {}", StartLine: 6, EndLine: 10, ChunkIndex: 1},
	}
	embeddings := [][]float32{
		{0.1, 0.2, 0.3, 0.4},
		{0.5, 0.6, 0.7, 0.8},
	}

	err = store.UpsertFile(storeRecord.ID, file, chunks, embeddings)
	require.NoError(t, err)

	// Get file by external ID
	retrieved, err := store.GetFileByExternalID(storeRecord.ID, "src/main.go")
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, "src/main.go", retrieved.ExternalID)
	assert.Equal(t, "xxh64:1234567890abcdef", retrieved.Hash)
	assert.Equal(t, int64(1024), retrieved.FileSize)

	// Get by hash
	byHash, err := store.GetFileByHash(storeRecord.ID, "xxh64:1234567890abcdef")
	require.NoError(t, err)
	require.NotNil(t, byHash)
	assert.Equal(t, retrieved.ID, byHash.ID)

	// List files
	files, err := store.ListFiles(storeRecord.ID, nil)
	require.NoError(t, err)
	assert.Len(t, files, 1)
}

func TestFileUpsertUpdate(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	storeRecord, err := store.CreateStore("test", "/path", ProviderOllama, "model", 4)
	require.NoError(t, err)

	// Insert initial file
	file := FileInput{
		ExternalID:   "src/main.go",
		Path:         "/path/src/main.go",
		RelativePath: "src/main.go",
		Hash:         "hash1",
		FileSize:     100,
	}
	chunks := []Chunk{{Content: "v1", StartLine: 1, EndLine: 1, ChunkIndex: 0}}
	embeddings := [][]float32{{0.1, 0.2, 0.3, 0.4}}

	err = store.UpsertFile(storeRecord.ID, file, chunks, embeddings)
	require.NoError(t, err)

	// Update with new content
	file.Hash = "hash2"
	file.FileSize = 200
	chunks = []Chunk{
		{Content: "v2-a", StartLine: 1, EndLine: 5, ChunkIndex: 0},
		{Content: "v2-b", StartLine: 6, EndLine: 10, ChunkIndex: 1},
	}
	embeddings = [][]float32{
		{0.1, 0.2, 0.3, 0.4},
		{0.5, 0.6, 0.7, 0.8},
	}

	err = store.UpsertFile(storeRecord.ID, file, chunks, embeddings)
	require.NoError(t, err)

	// Verify update
	retrieved, err := store.GetFileByExternalID(storeRecord.ID, "src/main.go")
	require.NoError(t, err)
	assert.Equal(t, "hash2", retrieved.Hash)
	assert.Equal(t, int64(200), retrieved.FileSize)
}

func TestFileDelete(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	storeRecord, err := store.CreateStore("test", "/path", ProviderOllama, "model", 4)
	require.NoError(t, err)

	// Insert a file
	file := FileInput{ExternalID: "to-delete.go", Path: "/path/to-delete.go", RelativePath: "to-delete.go", Hash: "h", FileSize: 1}
	chunks := []Chunk{{Content: "x", StartLine: 1, EndLine: 1, ChunkIndex: 0}}
	embeddings := [][]float32{{0.1, 0.2, 0.3, 0.4}}

	err = store.UpsertFile(storeRecord.ID, file, chunks, embeddings)
	require.NoError(t, err)

	// Delete it
	err = store.DeleteFile(storeRecord.ID, "to-delete.go")
	require.NoError(t, err)

	// Verify it's gone
	deleted, err := store.GetFileByExternalID(storeRecord.ID, "to-delete.go")
	require.NoError(t, err)
	assert.Nil(t, deleted)
}

func TestVectorSearch(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	storeRecord, err := store.CreateStore("test", "/path", ProviderOllama, "model", 4)
	require.NoError(t, err)

	// Insert files with known embeddings
	files := []struct {
		name      string
		embedding []float32
	}{
		{"file1.go", []float32{1, 0, 0, 0}},     // "north"
		{"file2.go", []float32{0, 1, 0, 0}},     // "east"
		{"file3.go", []float32{0.7, 0.7, 0, 0}}, // "northeast"
	}

	for _, f := range files {
		file := FileInput{
			ExternalID: f.name, Path: "/path/" + f.name, RelativePath: f.name,
			Hash: "h-" + f.name, FileSize: 100,
		}
		chunks := []Chunk{{Content: "content of " + f.name, StartLine: 1, EndLine: 10, ChunkIndex: 0}}
		err := store.UpsertFile(storeRecord.ID, file, chunks, [][]float32{f.embedding})
		require.NoError(t, err)
	}

	// Search for something similar to "north"
	query := []float32{0.9, 0.1, 0, 0}
	results, err := store.Search(storeRecord.ID, query, 3)
	require.NoError(t, err)
	require.Len(t, results, 3)

	// First result should be file1 (closest to north)
	assert.Equal(t, "file1.go", results[0].File.ExternalID)

	// Scores should be in descending order (most similar first)
	assert.True(t, results[0].Score >= results[1].Score)
	assert.True(t, results[1].Score >= results[2].Score)
}

func TestGetStats(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	storeRecord, err := store.CreateStore("test", "/path", ProviderOllama, "model", 4)
	require.NoError(t, err)

	// Insert multiple files
	for i := 0; i < 3; i++ {
		name := string(rune('a'+i)) + ".go"
		file := FileInput{ExternalID: name, Path: "/path/" + name, RelativePath: name, Hash: "h", FileSize: int64(100 * (i + 1))}
		chunks := []Chunk{
			{Content: "c1", StartLine: 1, EndLine: 5, ChunkIndex: 0},
			{Content: "c2", StartLine: 6, EndLine: 10, ChunkIndex: 1},
		}
		embeddings := [][]float32{{0.1, 0.2, 0.3, 0.4}, {0.5, 0.6, 0.7, 0.8}}
		err := store.UpsertFile(storeRecord.ID, file, chunks, embeddings)
		require.NoError(t, err)
	}

	// Get stats
	stats, err := store.GetStats(storeRecord.ID)
	require.NoError(t, err)

	assert.Equal(t, 3, stats.FileCount)
	assert.Equal(t, 6, stats.ChunkCount)         // 3 files * 2 chunks each
	assert.Equal(t, int64(600), stats.TotalSize) // 100 + 200 + 300
}

func TestClearStore(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	storeRecord, err := store.CreateStore("test", "/path", ProviderOllama, "model", 4)
	require.NoError(t, err)

	// Insert a file
	file := FileInput{ExternalID: "a.go", Path: "/path/a.go", RelativePath: "a.go", Hash: "h", FileSize: 100}
	chunks := []Chunk{{Content: "x", StartLine: 1, EndLine: 1, ChunkIndex: 0}}
	embeddings := [][]float32{{0.1, 0.2, 0.3, 0.4}}
	err = store.UpsertFile(storeRecord.ID, file, chunks, embeddings)
	require.NoError(t, err)

	// Clear store
	err = store.ClearStore(storeRecord.ID)
	require.NoError(t, err)

	// Verify files are gone but store remains
	files, err := store.ListFiles(storeRecord.ID, nil)
	require.NoError(t, err)
	assert.Empty(t, files)

	retrieved, err := store.GetStore("test")
	require.NoError(t, err)
	assert.NotNil(t, retrieved)
}

func TestSerializeEmbedding(t *testing.T) {
	embedding := []float32{1.0, 2.0, 3.0, 4.0}
	serialized := serializeEmbedding(embedding)

	// Each float32 is 4 bytes
	assert.Len(t, serialized, 16)

	// Verify it's little-endian
	// 1.0f = 0x3f800000
	assert.Equal(t, byte(0x00), serialized[0])
	assert.Equal(t, byte(0x00), serialized[1])
	assert.Equal(t, byte(0x80), serialized[2])
	assert.Equal(t, byte(0x3f), serialized[3])
}

func TestChunkEmbeddingMismatch(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	storeRecord, err := store.CreateStore("test", "/path", ProviderOllama, "model", 4)
	require.NoError(t, err)

	file := FileInput{ExternalID: "a.go", Path: "/path/a.go", RelativePath: "a.go", Hash: "h", FileSize: 100}
	chunks := []Chunk{{Content: "x", StartLine: 1, EndLine: 1, ChunkIndex: 0}}
	embeddings := [][]float32{} // Empty - mismatch!

	err = store.UpsertFile(storeRecord.ID, file, chunks, embeddings)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mismatch")
}

// Helper function to create a test store
func setupTestStore(t *testing.T) *SQLiteStore {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewSQLiteStore(dbPath)
	require.NoError(t, err)

	return store
}

// normalizeVector normalizes a vector to unit length (for testing).
func normalizeVector(v []float32) []float32 {
	var sum float32
	for _, x := range v {
		sum += x * x
	}
	norm := float32(math.Sqrt(float64(sum)))
	result := make([]float32, len(v))
	for i, x := range v {
		result[i] = x / norm
	}
	return result
}
