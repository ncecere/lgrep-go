// Package search provides semantic search functionality for lgrep.
package search

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/nickcecere/lgrep/internal/embeddings"
	"github.com/nickcecere/lgrep/internal/store"
)

// Searcher provides semantic search over indexed stores.
type Searcher struct {
	store    store.Store
	embedder embeddings.Service
}

// Result represents a search result with context.
type Result struct {
	// File information
	FilePath     string `json:"file_path"`
	RelativePath string `json:"relative_path"`

	// Chunk information
	Content   string `json:"content"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`

	// Similarity information
	Score    float64 `json:"score"`    // 0-1, higher is better
	Distance float64 `json:"distance"` // cosine distance

	// Context (optional, filled in by GetContext)
	ContextBefore string `json:"context_before,omitempty"`
	ContextAfter  string `json:"context_after,omitempty"`
}

// SearchOptions configures the search.
type SearchOptions struct {
	// StoreName is the name of the store to search.
	StoreName string

	// TopK is the maximum number of results to return.
	TopK int

	// MinScore filters results below this similarity score.
	MinScore float64

	// IncludeContent includes the chunk content in results.
	IncludeContent bool

	// ContextLines is the number of lines of context to include.
	ContextLines int
}

// DefaultSearchOptions returns sensible defaults.
func DefaultSearchOptions() SearchOptions {
	return SearchOptions{
		TopK:           10,
		MinScore:       0.0,
		IncludeContent: true,
		ContextLines:   0,
	}
}

// New creates a new Searcher.
func New(st store.Store, emb embeddings.Service) *Searcher {
	return &Searcher{
		store:    st,
		embedder: emb,
	}
}

// Search performs a semantic search with the given query.
func (s *Searcher) Search(ctx context.Context, query string, opts SearchOptions) ([]Result, error) {
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	// Get store
	storeRecord, err := s.store.GetStore(opts.StoreName)
	if err != nil {
		return nil, fmt.Errorf("failed to get store: %w", err)
	}
	if storeRecord == nil {
		return nil, fmt.Errorf("store not found: %s", opts.StoreName)
	}

	// Generate query embedding
	log.Debug("Generating query embedding", "query", truncate(query, 50))
	queryEmbedding, err := s.embedder.EmbedQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	// Search the store
	topK := opts.TopK
	if topK <= 0 {
		topK = 10
	}

	log.Debug("Searching store", "store", opts.StoreName, "topK", topK)
	searchResults, err := s.store.Search(storeRecord.ID, queryEmbedding, topK)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Convert to Result type and filter
	var results []Result
	for _, sr := range searchResults {
		// Filter by minimum score
		if sr.Score < opts.MinScore {
			continue
		}

		result := Result{
			FilePath:     sr.File.Path,
			RelativePath: sr.File.RelativePath,
			StartLine:    sr.Chunk.StartLine,
			EndLine:      sr.Chunk.EndLine,
			Score:        sr.Score,
			Distance:     sr.Distance,
		}

		if opts.IncludeContent {
			result.Content = sr.Chunk.Content
		}

		// Add context if requested
		if opts.ContextLines > 0 {
			before, after := s.getContext(sr.File.Path, sr.Chunk.StartLine, sr.Chunk.EndLine, opts.ContextLines)
			result.ContextBefore = before
			result.ContextAfter = after
		}

		results = append(results, result)
	}

	log.Debug("Search complete", "results", len(results))
	return results, nil
}

// SearchAll searches across all stores.
func (s *Searcher) SearchAll(ctx context.Context, query string, opts SearchOptions) ([]Result, error) {
	stores, err := s.store.ListStores()
	if err != nil {
		return nil, fmt.Errorf("failed to list stores: %w", err)
	}

	if len(stores) == 0 {
		return nil, fmt.Errorf("no indexed stores found")
	}

	// Generate query embedding once
	log.Debug("Generating query embedding", "query", truncate(query, 50))
	queryEmbedding, err := s.embedder.EmbedQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	topK := opts.TopK
	if topK <= 0 {
		topK = 10
	}

	// Search all stores and combine results
	var allResults []Result
	for _, storeRecord := range stores {
		searchResults, err := s.store.Search(storeRecord.ID, queryEmbedding, topK)
		if err != nil {
			log.Warn("Search failed for store", "store", storeRecord.Name, "error", err)
			continue
		}

		for _, sr := range searchResults {
			if sr.Score < opts.MinScore {
				continue
			}

			result := Result{
				FilePath:     sr.File.Path,
				RelativePath: sr.File.RelativePath,
				StartLine:    sr.Chunk.StartLine,
				EndLine:      sr.Chunk.EndLine,
				Score:        sr.Score,
				Distance:     sr.Distance,
			}

			if opts.IncludeContent {
				result.Content = sr.Chunk.Content
			}

			allResults = append(allResults, result)
		}
	}

	// Sort by score (descending) and limit to topK
	sortByScore(allResults)
	if len(allResults) > topK {
		allResults = allResults[:topK]
	}

	return allResults, nil
}

// getContext reads additional context lines from the file.
func (s *Searcher) getContext(filePath string, startLine, endLine, contextLines int) (before, after string) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", ""
	}

	lines := strings.Split(string(content), "\n")

	// Get lines before
	beforeStart := startLine - contextLines - 1
	if beforeStart < 0 {
		beforeStart = 0
	}
	beforeEnd := startLine - 1
	if beforeEnd > 0 && beforeEnd <= len(lines) {
		before = strings.Join(lines[beforeStart:beforeEnd], "\n")
	}

	// Get lines after
	afterStart := endLine
	if afterStart < len(lines) {
		afterEnd := afterStart + contextLines
		if afterEnd > len(lines) {
			afterEnd = len(lines)
		}
		after = strings.Join(lines[afterStart:afterEnd], "\n")
	}

	return before, after
}

// GetStoreForPath finds the store that contains the given path.
func (s *Searcher) GetStoreForPath(path string) (*store.StoreRecord, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	stores, err := s.store.ListStores()
	if err != nil {
		return nil, err
	}

	// Find store whose root path contains the given path
	for _, storeRecord := range stores {
		if strings.HasPrefix(absPath, storeRecord.RootPath) {
			return &storeRecord, nil
		}
	}

	// If path is a directory, find store with matching root
	for _, storeRecord := range stores {
		if storeRecord.RootPath == absPath {
			return &storeRecord, nil
		}
	}

	return nil, nil
}

// sortByScore sorts results by score in descending order.
func sortByScore(results []Result) {
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}

// truncate shortens a string for display.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
