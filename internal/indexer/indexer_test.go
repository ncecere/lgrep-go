package indexer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nickcecere/lgrep/internal/config"
	"github.com/nickcecere/lgrep/internal/embeddings"
	"github.com/nickcecere/lgrep/internal/store"
)

// mockEmbedder implements embeddings.Service for testing.
type mockEmbedder struct {
	model      string
	dimensions int
	embedCalls int
}

func (m *mockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	m.embedCalls++
	return m.generateEmbedding(), nil
}

func (m *mockEmbedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	m.embedCalls++
	return m.generateEmbedding(), nil
}

func (m *mockEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	m.embedCalls++
	result := make([][]float32, len(texts))
	for i := range texts {
		result[i] = m.generateEmbedding()
	}
	return result, nil
}

func (m *mockEmbedder) Dimensions() int {
	return m.dimensions
}

func (m *mockEmbedder) Provider() embeddings.Provider {
	return embeddings.ProviderOllama // Use a valid provider
}

func (m *mockEmbedder) ModelName() string {
	return m.model
}

func (m *mockEmbedder) generateEmbedding() []float32 {
	emb := make([]float32, m.dimensions)
	for i := range emb {
		emb[i] = float32(i) * 0.01
	}
	return emb
}

// Verify mockEmbedder implements embeddings.Service
var _ embeddings.Service = (*mockEmbedder)(nil)

// createTestEnv creates a test environment with temp directory and files.
func createTestEnv(t *testing.T) (string, func()) {
	tmpDir := t.TempDir()

	// Create test files
	files := map[string]string{
		"main.go": `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
`,
		"utils.go": `package main

func helper() string {
	return "helper"
}
`,
		"lib/lib.go": `package lib

func LibFunc() {
}
`,
		"README.md": "# Test Project\n\nThis is a test.",
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		dir := filepath.Dir(fullPath)
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(fullPath, []byte(content), 0644))
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

// createTestConfig creates a test configuration.
func createTestConfig() *config.Config {
	return &config.Config{
		Indexing: config.IndexingConfig{
			MaxFileSize:  1024 * 1024,
			MaxFileCount: 1000,
			ChunkSize:    1000,
			ChunkOverlap: 100,
		},
		Ignore: []string{},
	}
}

// TestIndexerCreation tests indexer creation.
func TestIndexerCreation(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	st, err := store.NewSQLiteStore(dbPath)
	require.NoError(t, err)
	defer st.Close()

	emb := &mockEmbedder{model: "test-model", dimensions: 768}
	cfg := createTestConfig()

	idx := New(st, emb, cfg)
	require.NotNil(t, idx)
}

// TestIndexDirectory tests indexing a directory.
func TestIndexDirectory(t *testing.T) {
	testDir, cleanup := createTestEnv(t)
	defer cleanup()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	st, err := store.NewSQLiteStore(dbPath)
	require.NoError(t, err)
	defer st.Close()

	emb := &mockEmbedder{model: "test-model", dimensions: 768}
	cfg := createTestConfig()

	idx := New(st, emb, cfg)

	// Index the test directory
	err = idx.Index(context.Background(), IndexOptions{
		StoreName: "test-store",
		Path:      testDir,
		BatchSize: 10,
	})
	require.NoError(t, err)

	// Verify store was created
	stores, err := idx.List()
	require.NoError(t, err)
	require.Len(t, stores, 1)
	assert.Equal(t, "test-store", stores[0].Name)

	// Verify stats
	stats, err := idx.Stats("test-store")
	require.NoError(t, err)
	assert.Greater(t, stats.FileCount, 0)
	assert.Greater(t, stats.ChunkCount, 0)
}

// TestIndexSkipsUnchangedFiles tests that unchanged files are skipped.
func TestIndexSkipsUnchangedFiles(t *testing.T) {
	testDir, cleanup := createTestEnv(t)
	defer cleanup()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	st, err := store.NewSQLiteStore(dbPath)
	require.NoError(t, err)
	defer st.Close()

	emb := &mockEmbedder{model: "test-model", dimensions: 768}
	cfg := createTestConfig()

	idx := New(st, emb, cfg)

	// First index
	err = idx.Index(context.Background(), IndexOptions{
		StoreName: "test-store",
		Path:      testDir,
	})
	require.NoError(t, err)

	firstEmbedCalls := emb.embedCalls

	// Second index (should skip unchanged)
	err = idx.Index(context.Background(), IndexOptions{
		StoreName: "test-store",
		Path:      testDir,
	})
	require.NoError(t, err)

	// No new embed calls should have been made
	assert.Equal(t, firstEmbedCalls, emb.embedCalls, "should skip unchanged files")
}

// TestIndexForce tests force re-indexing.
func TestIndexForce(t *testing.T) {
	testDir, cleanup := createTestEnv(t)
	defer cleanup()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	st, err := store.NewSQLiteStore(dbPath)
	require.NoError(t, err)
	defer st.Close()

	emb := &mockEmbedder{model: "test-model", dimensions: 768}
	cfg := createTestConfig()

	idx := New(st, emb, cfg)

	// First index
	err = idx.Index(context.Background(), IndexOptions{
		StoreName: "test-store",
		Path:      testDir,
	})
	require.NoError(t, err)

	firstEmbedCalls := emb.embedCalls

	// Force re-index
	err = idx.Index(context.Background(), IndexOptions{
		StoreName: "test-store",
		Path:      testDir,
		Force:     true,
	})
	require.NoError(t, err)

	// Should have more embed calls
	assert.Greater(t, emb.embedCalls, firstEmbedCalls, "force should re-index all files")
}

// TestIndexWithExtensionFilter tests extension filtering.
func TestIndexWithExtensionFilter(t *testing.T) {
	testDir, cleanup := createTestEnv(t)
	defer cleanup()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	st, err := store.NewSQLiteStore(dbPath)
	require.NoError(t, err)
	defer st.Close()

	emb := &mockEmbedder{model: "test-model", dimensions: 768}
	cfg := createTestConfig()

	idx := New(st, emb, cfg)

	// Index only .go files
	err = idx.Index(context.Background(), IndexOptions{
		StoreName:  "test-store",
		Path:       testDir,
		Extensions: []string{".go"},
	})
	require.NoError(t, err)

	// Check stats - should only have Go files
	stats, err := idx.Stats("test-store")
	require.NoError(t, err)
	assert.Equal(t, 3, stats.FileCount, "should only index .go files") // main.go, utils.go, lib/lib.go
}

// TestIndexProgress tests progress callback.
func TestIndexProgress(t *testing.T) {
	testDir, cleanup := createTestEnv(t)
	defer cleanup()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	st, err := store.NewSQLiteStore(dbPath)
	require.NoError(t, err)
	defer st.Close()

	emb := &mockEmbedder{model: "test-model", dimensions: 768}
	cfg := createTestConfig()

	idx := New(st, emb, cfg)

	var progressCalls int
	var lastProgress Progress

	err = idx.Index(context.Background(), IndexOptions{
		StoreName: "test-store",
		Path:      testDir,
		OnProgress: func(p Progress) {
			progressCalls++
			lastProgress = p
		},
	})
	require.NoError(t, err)

	assert.Greater(t, progressCalls, 0, "progress callback should be called")
	assert.Greater(t, lastProgress.ProcessedFiles, 0)
}

// TestIndexCancellation tests context cancellation.
func TestIndexCancellation(t *testing.T) {
	testDir, cleanup := createTestEnv(t)
	defer cleanup()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	st, err := store.NewSQLiteStore(dbPath)
	require.NoError(t, err)
	defer st.Close()

	emb := &mockEmbedder{model: "test-model", dimensions: 768}
	cfg := createTestConfig()

	idx := New(st, emb, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err = idx.Index(ctx, IndexOptions{
		StoreName: "test-store",
		Path:      testDir,
	})

	assert.ErrorIs(t, err, context.Canceled)
}

// TestIndexerDelete tests store deletion.
func TestIndexerDelete(t *testing.T) {
	testDir, cleanup := createTestEnv(t)
	defer cleanup()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	st, err := store.NewSQLiteStore(dbPath)
	require.NoError(t, err)
	defer st.Close()

	emb := &mockEmbedder{model: "test-model", dimensions: 768}
	cfg := createTestConfig()

	idx := New(st, emb, cfg)

	// Create store
	err = idx.Index(context.Background(), IndexOptions{
		StoreName: "test-store",
		Path:      testDir,
	})
	require.NoError(t, err)

	// Delete store
	err = idx.Delete("test-store")
	require.NoError(t, err)

	// Verify deleted
	stores, err := idx.List()
	require.NoError(t, err)
	assert.Len(t, stores, 0)
}

// TestIndexerClear tests clearing a store.
func TestIndexerClear(t *testing.T) {
	testDir, cleanup := createTestEnv(t)
	defer cleanup()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	st, err := store.NewSQLiteStore(dbPath)
	require.NoError(t, err)
	defer st.Close()

	emb := &mockEmbedder{model: "test-model", dimensions: 768}
	cfg := createTestConfig()

	idx := New(st, emb, cfg)

	// Create store
	err = idx.Index(context.Background(), IndexOptions{
		StoreName: "test-store",
		Path:      testDir,
	})
	require.NoError(t, err)

	// Clear store
	err = idx.Clear("test-store")
	require.NoError(t, err)

	// Verify cleared (store exists but no files)
	stats, err := idx.Stats("test-store")
	require.NoError(t, err)
	assert.Equal(t, 0, stats.FileCount)
	assert.Equal(t, 0, stats.ChunkCount)
}

// TestIndexInvalidPath tests error handling for invalid path.
func TestIndexInvalidPath(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	st, err := store.NewSQLiteStore(dbPath)
	require.NoError(t, err)
	defer st.Close()

	emb := &mockEmbedder{model: "test-model", dimensions: 768}
	cfg := createTestConfig()

	idx := New(st, emb, cfg)

	err = idx.Index(context.Background(), IndexOptions{
		StoreName: "test-store",
		Path:      "/nonexistent/path",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

// TestDefaultIndexOptions tests default options.
func TestDefaultIndexOptions(t *testing.T) {
	opts := DefaultIndexOptions()
	assert.Equal(t, 50, opts.BatchSize)
}

// TestProgressStruct tests Progress struct fields.
func TestProgressStruct(t *testing.T) {
	p := Progress{
		TotalFiles:      10,
		ProcessedFiles:  5,
		SkippedFiles:    2,
		TotalChunks:     100,
		ProcessedChunks: 50,
		Errors:          1,
		StartTime:       time.Now(),
		CurrentFile:     "test.go",
	}

	assert.Equal(t, 10, p.TotalFiles)
	assert.Equal(t, 5, p.ProcessedFiles)
	assert.Equal(t, 2, p.SkippedFiles)
	assert.Equal(t, 100, p.TotalChunks)
	assert.Equal(t, 50, p.ProcessedChunks)
	assert.Equal(t, 1, p.Errors)
	assert.Equal(t, "test.go", p.CurrentFile)
}
