// Package indexer provides the core indexing logic for lgrep.
package indexer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/nickcecere/lgrep/internal/config"
	"github.com/nickcecere/lgrep/internal/embeddings"
	"github.com/nickcecere/lgrep/internal/fs"
	"github.com/nickcecere/lgrep/internal/store"
)

// Indexer orchestrates the indexing of files into the vector store.
type Indexer struct {
	store    store.Store
	embedder embeddings.Service
	chunker  *fs.TextChunker
	cfg      *config.Config

	// Progress tracking
	progress Progress
	mu       sync.Mutex
}

// Progress tracks indexing progress.
type Progress struct {
	TotalFiles      int
	ProcessedFiles  int
	SkippedFiles    int
	TotalChunks     int
	ProcessedChunks int
	Errors          int
	StartTime       time.Time
	CurrentFile     string
}

// ProgressFunc is called to report progress during indexing.
type ProgressFunc func(Progress)

// IndexOptions configures the indexing process.
type IndexOptions struct {
	// StoreName is the name of the store to index into.
	StoreName string

	// Path is the directory to index.
	Path string

	// Extensions limits to specific file extensions.
	Extensions []string

	// IgnorePatterns are additional patterns to ignore.
	IgnorePatterns []string

	// Force re-indexes files even if unchanged.
	Force bool

	// BatchSize is the number of chunks to embed in a single batch.
	BatchSize int

	// OnProgress is called to report progress.
	OnProgress ProgressFunc
}

// DefaultIndexOptions returns sensible defaults.
func DefaultIndexOptions() IndexOptions {
	return IndexOptions{
		BatchSize: 50,
	}
}

// New creates a new Indexer.
func New(st store.Store, emb embeddings.Service, cfg *config.Config) *Indexer {
	return &Indexer{
		store:    st,
		embedder: emb,
		chunker: fs.NewTextChunker(fs.ChunkOptions{
			ChunkSize:    cfg.Indexing.ChunkSize,
			ChunkOverlap: cfg.Indexing.ChunkOverlap,
			MinChunkSize: 100,
		}),
		cfg: cfg,
	}
}

// Index indexes files from the given path into the store.
func (idx *Indexer) Index(ctx context.Context, opts IndexOptions) error {
	// Resolve path
	absPath, err := filepath.Abs(opts.Path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Check path exists
	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("path does not exist: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", absPath)
	}

	// Get or create the store
	storeName := opts.StoreName
	if storeName == "" {
		storeName = filepath.Base(absPath)
	}

	storeRecord, err := idx.getOrCreateStore(storeName, absPath)
	if err != nil {
		return err
	}

	// Check context
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Initialize progress
	idx.mu.Lock()
	idx.progress = Progress{
		StartTime: time.Now(),
	}
	idx.mu.Unlock()

	// Create file walker
	walker, err := fs.NewFileWalker(fs.WalkOptions{
		Root:           absPath,
		MaxFileSize:    int64(idx.cfg.Indexing.MaxFileSize),
		MaxFileCount:   idx.cfg.Indexing.MaxFileCount,
		IgnorePatterns: append(idx.cfg.Ignore, opts.IgnorePatterns...),
		UseGitignore:   true,
		Extensions:     opts.Extensions,
	})
	if err != nil {
		return fmt.Errorf("failed to create file walker: %w", err)
	}

	// First pass: collect files and count
	var files []fs.FileInfo
	err = walker.Walk(func(fi fs.FileInfo) error {
		files = append(files, fi)
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to walk directory: %w", err)
	}

	idx.mu.Lock()
	idx.progress.TotalFiles = len(files)
	idx.mu.Unlock()

	log.Info("Found files to index", "count", len(files))

	// Process files
	for _, fi := range files {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		idx.mu.Lock()
		idx.progress.CurrentFile = fi.RelPath
		idx.mu.Unlock()

		if err := idx.indexFile(ctx, storeRecord, fi, opts); err != nil {
			log.Warn("Failed to index file", "path", fi.RelPath, "error", err)
			idx.mu.Lock()
			idx.progress.Errors++
			idx.mu.Unlock()
			continue
		}

		idx.mu.Lock()
		idx.progress.ProcessedFiles++
		if opts.OnProgress != nil {
			opts.OnProgress(idx.progress)
		}
		idx.mu.Unlock()
	}

	// Update store timestamp
	if err := idx.store.UpdateStoreTimestamp(storeRecord.ID); err != nil {
		log.Warn("Failed to update store timestamp", "error", err)
	}

	// Get final stats
	stats, err := idx.store.GetStats(storeRecord.ID)
	if err == nil {
		log.Info("Indexing complete",
			"files", stats.FileCount,
			"chunks", stats.ChunkCount,
			"duration", time.Since(idx.progress.StartTime).Round(time.Millisecond),
		)
	}

	return nil
}

// getOrCreateStore gets an existing store or creates a new one.
func (idx *Indexer) getOrCreateStore(name, path string) (*store.StoreRecord, error) {
	// Check if store exists
	existing, err := idx.store.GetStore(name)
	if err != nil {
		return nil, fmt.Errorf("failed to check for existing store: %w", err)
	}
	if existing != nil {
		// Store exists, verify path matches
		if existing.RootPath != path {
			log.Warn("Store path mismatch", "stored", existing.RootPath, "requested", path)
		}
		return existing, nil
	}

	// Create new store
	log.Info("Creating new store", "name", name, "path", path)
	storeRecord, err := idx.store.CreateStore(
		name,
		path,
		store.EmbeddingProvider(string(idx.embedder.Provider())),
		idx.embedder.ModelName(),
		idx.embedder.Dimensions(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create store: %w", err)
	}

	return storeRecord, nil
}

// indexFile indexes a single file.
func (idx *Indexer) indexFile(ctx context.Context, storeRecord *store.StoreRecord, fi fs.FileInfo, opts IndexOptions) error {
	// Check if file needs re-indexing
	if !opts.Force {
		existing, err := idx.store.GetFileByExternalID(storeRecord.ID, fi.RelPath)
		if err != nil {
			log.Debug("Error checking existing file", "path", fi.RelPath, "error", err)
		} else if existing != nil && existing.Hash == fi.Hash {
			log.Debug("File unchanged, skipping", "path", fi.RelPath)
			idx.mu.Lock()
			idx.progress.SkippedFiles++
			idx.mu.Unlock()
			return nil
		}
	}

	// Read file content
	content, err := os.ReadFile(fi.Path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Chunk the content
	chunks := idx.chunker.Chunk(string(content), fi.Path)
	if len(chunks) == 0 {
		log.Debug("No chunks generated", "path", fi.RelPath)
		return nil
	}

	idx.mu.Lock()
	idx.progress.TotalChunks += len(chunks)
	idx.mu.Unlock()

	// Generate embeddings in batches
	batchSize := opts.BatchSize
	if batchSize <= 0 {
		batchSize = 50
	}

	var storeChunks []store.Chunk
	var allEmbeddings [][]float32

	for i := 0; i < len(chunks); i += batchSize {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		end := i + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}
		batch := chunks[i:end]

		// Extract text for embedding
		texts := make([]string, len(batch))
		for j, c := range batch {
			texts[j] = c.Content
		}

		// Generate embeddings
		embeddingVectors, err := idx.embedder.EmbedBatch(ctx, texts)
		if err != nil {
			return fmt.Errorf("failed to generate embeddings: %w", err)
		}

		// Create store chunks
		for j, c := range batch {
			storeChunks = append(storeChunks, store.Chunk{
				Content:    c.Content,
				StartLine:  c.StartLine,
				EndLine:    c.EndLine,
				ChunkIndex: c.ChunkIndex,
			})
			allEmbeddings = append(allEmbeddings, embeddingVectors[j])
		}

		idx.mu.Lock()
		idx.progress.ProcessedChunks += len(batch)
		if opts.OnProgress != nil {
			opts.OnProgress(idx.progress)
		}
		idx.mu.Unlock()
	}

	// Upsert file with chunks
	fileInput := store.FileInput{
		ExternalID:   fi.RelPath,
		Path:         fi.Path,
		RelativePath: fi.RelPath,
		Hash:         fi.Hash,
		FileSize:     fi.Size,
	}

	err = idx.store.UpsertFile(storeRecord.ID, fileInput, storeChunks, allEmbeddings)
	if err != nil {
		return fmt.Errorf("failed to store file: %w", err)
	}

	log.Debug("Indexed file", "path", fi.RelPath, "chunks", len(storeChunks))
	return nil
}

// Progress returns the current indexing progress.
func (idx *Indexer) Progress() Progress {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	return idx.progress
}

// IndexSingleFile indexes a single file by its absolute path.
// This is used by the watcher for incremental updates.
func (idx *Indexer) IndexSingleFile(ctx context.Context, storeName, rootPath, filePath string) error {
	// Get or create the store
	storeRecord, err := idx.getOrCreateStore(storeName, rootPath)
	if err != nil {
		return err
	}

	// Get file info
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Calculate relative path
	relPath, err := filepath.Rel(rootPath, filePath)
	if err != nil {
		return fmt.Errorf("failed to get relative path: %w", err)
	}

	// Compute file hash
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	hash := fs.HashContent(content)

	// Detect language
	lang := fs.DetectLanguage(filePath)

	fi := fs.FileInfo{
		Path:     filePath,
		RelPath:  relPath,
		Size:     info.Size(),
		ModTime:  info.ModTime(),
		Hash:     hash,
		Language: lang,
	}

	opts := IndexOptions{
		StoreName: storeName,
		Force:     true, // Always re-index when called from watcher
		BatchSize: 50,
	}

	return idx.indexFile(ctx, storeRecord, fi, opts)
}

// Delete removes a store and all its indexed data.
func (idx *Indexer) Delete(storeName string) error {
	return idx.store.DeleteStore(storeName)
}

// GetStoreRecord returns the store record for a given store name.
func (idx *Indexer) GetStoreRecord(storeName string) (*store.StoreRecord, error) {
	return idx.store.GetStore(storeName)
}

// DeleteFile removes a file from the index by its relative path.
func (idx *Indexer) DeleteFile(storeName, relPath string) error {
	storeRecord, err := idx.store.GetStore(storeName)
	if err != nil {
		return fmt.Errorf("store not found: %s", storeName)
	}
	if storeRecord == nil {
		return fmt.Errorf("store not found: %s", storeName)
	}
	return idx.store.DeleteFile(storeRecord.ID, relPath)
}

// Clear removes all indexed data from a store but keeps the store.
func (idx *Indexer) Clear(storeName string) error {
	storeRecord, err := idx.store.GetStore(storeName)
	if err != nil {
		return fmt.Errorf("store not found: %s", storeName)
	}
	return idx.store.ClearStore(storeRecord.ID)
}

// List returns all stores.
func (idx *Indexer) List() ([]store.StoreRecord, error) {
	return idx.store.ListStores()
}

// Stats returns statistics for a store.
func (idx *Indexer) Stats(storeName string) (*store.StoreStats, error) {
	storeRecord, err := idx.store.GetStore(storeName)
	if err != nil {
		return nil, fmt.Errorf("store not found: %s", storeName)
	}
	return idx.store.GetStats(storeRecord.ID)
}
