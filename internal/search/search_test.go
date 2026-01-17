package search

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nickcecere/lgrep/internal/embeddings"
	"github.com/nickcecere/lgrep/internal/store"
)

// mockEmbedder implements embeddings.Service for testing.
type mockEmbedder struct {
	model      string
	dimensions int
}

func (m *mockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return m.generateEmbedding(text), nil
}

func (m *mockEmbedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	return m.generateEmbedding(text), nil
}

func (m *mockEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i, text := range texts {
		result[i] = m.generateEmbedding(text)
	}
	return result, nil
}

func (m *mockEmbedder) Dimensions() int {
	return m.dimensions
}

func (m *mockEmbedder) Provider() embeddings.Provider {
	return embeddings.ProviderOllama
}

func (m *mockEmbedder) ModelName() string {
	return m.model
}

// generateEmbedding creates a deterministic embedding based on text.
func (m *mockEmbedder) generateEmbedding(text string) []float32 {
	emb := make([]float32, m.dimensions)
	// Create a simple hash-based embedding for deterministic results
	hash := 0
	for _, c := range text {
		hash = hash*31 + int(c)
	}
	for i := range emb {
		emb[i] = float32((hash+i)%100) / 100.0
	}
	return emb
}

// Verify mockEmbedder implements embeddings.Service
var _ embeddings.Service = (*mockEmbedder)(nil)

// createTestStore creates a test store with sample data.
func createTestStore(t *testing.T) (store.Store, string, func()) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	st, err := store.NewSQLiteStore(dbPath)
	require.NoError(t, err)

	// Create a store
	storeRecord, err := st.CreateStore(
		"test-store",
		tmpDir,
		store.ProviderOllama,
		"test-model",
		768,
	)
	require.NoError(t, err)

	// Create test files in the temp directory
	testFile := filepath.Join(tmpDir, "main.go")
	testContent := `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}

func helper() {
	// do something helpful
}
`
	err = os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

	// Add file with chunks to the store
	emb := &mockEmbedder{model: "test-model", dimensions: 768}

	chunks := []store.Chunk{
		{Content: "package main\nimport \"fmt\"", StartLine: 1, EndLine: 3, ChunkIndex: 0},
		{Content: "func main() {\n\tfmt.Println(\"Hello, World!\")\n}", StartLine: 5, EndLine: 7, ChunkIndex: 1},
		{Content: "func helper() {\n\t// do something helpful\n}", StartLine: 9, EndLine: 11, ChunkIndex: 2},
	}

	embeddings := make([][]float32, len(chunks))
	for i, c := range chunks {
		embeddings[i] = emb.generateEmbedding(c.Content)
	}

	err = st.UpsertFile(storeRecord.ID, store.FileInput{
		ExternalID:   "main.go",
		Path:         testFile,
		RelativePath: "main.go",
		Hash:         "testhash123",
		FileSize:     int64(len(testContent)),
	}, chunks, embeddings)
	require.NoError(t, err)

	cleanup := func() {
		st.Close()
		os.RemoveAll(tmpDir)
	}

	return st, tmpDir, cleanup
}

// TestSearcherCreation tests searcher creation.
func TestSearcherCreation(t *testing.T) {
	st, _, cleanup := createTestStore(t)
	defer cleanup()

	emb := &mockEmbedder{model: "test-model", dimensions: 768}
	searcher := New(st, emb)

	require.NotNil(t, searcher)
}

// TestSearch tests basic search functionality.
func TestSearch(t *testing.T) {
	st, _, cleanup := createTestStore(t)
	defer cleanup()

	emb := &mockEmbedder{model: "test-model", dimensions: 768}
	searcher := New(st, emb)

	results, err := searcher.Search(context.Background(), "hello world", SearchOptions{
		StoreName:      "test-store",
		TopK:           10,
		IncludeContent: true,
	})
	require.NoError(t, err)

	// Should return results
	require.NotEmpty(t, results)

	// Check result structure
	for _, r := range results {
		assert.NotEmpty(t, r.FilePath)
		assert.NotEmpty(t, r.RelativePath)
		assert.Greater(t, r.StartLine, 0)
		assert.GreaterOrEqual(t, r.EndLine, r.StartLine)
		assert.GreaterOrEqual(t, r.Score, 0.0)
		assert.LessOrEqual(t, r.Score, 1.0)
	}
}

// TestSearchWithMinScore tests score filtering.
func TestSearchWithMinScore(t *testing.T) {
	st, _, cleanup := createTestStore(t)
	defer cleanup()

	emb := &mockEmbedder{model: "test-model", dimensions: 768}
	searcher := New(st, emb)

	// Search with high min score - should return fewer or no results
	results, err := searcher.Search(context.Background(), "test query", SearchOptions{
		StoreName: "test-store",
		TopK:      10,
		MinScore:  0.99, // Very high threshold
	})
	require.NoError(t, err)

	// With such a high threshold, we likely get no results
	// (depends on the mock embedding similarity)
	assert.True(t, len(results) == 0 || results[0].Score >= 0.99)
}

// TestSearchWithContent tests content inclusion.
func TestSearchWithContent(t *testing.T) {
	st, _, cleanup := createTestStore(t)
	defer cleanup()

	emb := &mockEmbedder{model: "test-model", dimensions: 768}
	searcher := New(st, emb)

	// Without content
	resultsNoContent, err := searcher.Search(context.Background(), "main function", SearchOptions{
		StoreName:      "test-store",
		TopK:           10,
		IncludeContent: false,
	})
	require.NoError(t, err)

	// With content
	resultsWithContent, err := searcher.Search(context.Background(), "main function", SearchOptions{
		StoreName:      "test-store",
		TopK:           10,
		IncludeContent: true,
	})
	require.NoError(t, err)

	// Both should have same number of results
	assert.Equal(t, len(resultsNoContent), len(resultsWithContent))

	// Without content, Content should be empty
	for _, r := range resultsNoContent {
		assert.Empty(t, r.Content)
	}

	// With content, Content should be populated
	for _, r := range resultsWithContent {
		assert.NotEmpty(t, r.Content)
	}
}

// TestSearchEmptyQuery tests error handling for empty query.
func TestSearchEmptyQuery(t *testing.T) {
	st, _, cleanup := createTestStore(t)
	defer cleanup()

	emb := &mockEmbedder{model: "test-model", dimensions: 768}
	searcher := New(st, emb)

	_, err := searcher.Search(context.Background(), "", SearchOptions{
		StoreName: "test-store",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

// TestSearchNonexistentStore tests error handling for missing store.
func TestSearchNonexistentStore(t *testing.T) {
	st, _, cleanup := createTestStore(t)
	defer cleanup()

	emb := &mockEmbedder{model: "test-model", dimensions: 768}
	searcher := New(st, emb)

	_, err := searcher.Search(context.Background(), "test query", SearchOptions{
		StoreName: "nonexistent-store",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestSearchTopK tests result limiting.
func TestSearchTopK(t *testing.T) {
	st, _, cleanup := createTestStore(t)
	defer cleanup()

	emb := &mockEmbedder{model: "test-model", dimensions: 768}
	searcher := New(st, emb)

	// Request only 1 result
	results, err := searcher.Search(context.Background(), "function", SearchOptions{
		StoreName:      "test-store",
		TopK:           1,
		IncludeContent: true,
	})
	require.NoError(t, err)

	// Should return at most 1 result
	assert.LessOrEqual(t, len(results), 1)
}

// TestGetStoreForPath tests store detection from path.
func TestGetStoreForPath(t *testing.T) {
	st, tmpDir, cleanup := createTestStore(t)
	defer cleanup()

	emb := &mockEmbedder{model: "test-model", dimensions: 768}
	searcher := New(st, emb)

	// Should find store for its root path
	storeRecord, err := searcher.GetStoreForPath(tmpDir)
	require.NoError(t, err)
	require.NotNil(t, storeRecord)
	assert.Equal(t, "test-store", storeRecord.Name)

	// Should find store for a file within the root
	filePath := filepath.Join(tmpDir, "main.go")
	storeRecord, err = searcher.GetStoreForPath(filePath)
	require.NoError(t, err)
	require.NotNil(t, storeRecord)
	assert.Equal(t, "test-store", storeRecord.Name)

	// Should return nil for unrelated path
	storeRecord, err = searcher.GetStoreForPath("/some/other/path")
	require.NoError(t, err)
	assert.Nil(t, storeRecord)
}

// TestDefaultSearchOptions tests default options.
func TestDefaultSearchOptions(t *testing.T) {
	opts := DefaultSearchOptions()

	assert.Equal(t, 10, opts.TopK)
	assert.Equal(t, 0.0, opts.MinScore)
	assert.True(t, opts.IncludeContent)
	assert.Equal(t, 0, opts.ContextLines)
}

// TestResultStruct tests Result struct fields.
func TestResultStruct(t *testing.T) {
	r := Result{
		FilePath:      "/path/to/file.go",
		RelativePath:  "file.go",
		Content:       "func test() {}",
		StartLine:     10,
		EndLine:       12,
		Score:         0.85,
		Distance:      0.15,
		ContextBefore: "// before",
		ContextAfter:  "// after",
	}

	assert.Equal(t, "/path/to/file.go", r.FilePath)
	assert.Equal(t, "file.go", r.RelativePath)
	assert.Equal(t, "func test() {}", r.Content)
	assert.Equal(t, 10, r.StartLine)
	assert.Equal(t, 12, r.EndLine)
	assert.Equal(t, 0.85, r.Score)
	assert.Equal(t, 0.15, r.Distance)
	assert.Equal(t, "// before", r.ContextBefore)
	assert.Equal(t, "// after", r.ContextAfter)
}

// TestSearchCancellation tests context cancellation.
func TestSearchCancellation(t *testing.T) {
	st, _, cleanup := createTestStore(t)
	defer cleanup()

	emb := &mockEmbedder{model: "test-model", dimensions: 768}
	searcher := New(st, emb)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Note: The search may complete before checking context since it's fast.
	// In a real scenario with slow embedding, this would fail.
	// For testing, we just verify it doesn't panic.
	_, _ = searcher.Search(ctx, "test", SearchOptions{
		StoreName: "test-store",
	})
}

// TestSortByScore tests result sorting.
func TestSortByScore(t *testing.T) {
	results := []Result{
		{Score: 0.5},
		{Score: 0.9},
		{Score: 0.3},
		{Score: 0.7},
	}

	sortByScore(results)

	// Should be sorted descending
	assert.Equal(t, 0.9, results[0].Score)
	assert.Equal(t, 0.7, results[1].Score)
	assert.Equal(t, 0.5, results[2].Score)
	assert.Equal(t, 0.3, results[3].Score)
}

// TestTruncate tests string truncation.
func TestTruncate(t *testing.T) {
	// Short string - no truncation
	assert.Equal(t, "hello", truncate("hello", 10))

	// Long string - truncated
	result := truncate("hello world this is a long string", 10)
	assert.Len(t, result, 10)
	assert.True(t, strings.HasSuffix(result, "..."))

	// Exact length
	assert.Equal(t, "hello", truncate("hello", 5))
}
