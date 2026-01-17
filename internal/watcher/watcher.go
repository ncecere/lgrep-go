// Package watcher provides file system watching with automatic re-indexing.
package watcher

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/fsnotify/fsnotify"

	"github.com/nickcecere/lgrep/internal/config"
	"github.com/nickcecere/lgrep/internal/embeddings"
	"github.com/nickcecere/lgrep/internal/fs"
	"github.com/nickcecere/lgrep/internal/indexer"
	"github.com/nickcecere/lgrep/internal/store"
)

// Watcher watches for file changes and triggers re-indexing.
type Watcher struct {
	root      string
	storeName string
	store     store.Store
	embedder  embeddings.Service
	indexer   *indexer.Indexer
	cfg       *config.Config

	// debounce holds pending file events to batch process
	debounce     map[string]fsnotify.Op
	debounceMu   sync.Mutex
	debounceTime time.Duration

	// callback for status updates
	onEvent func(event string, path string)
}

// Option configures the watcher.
type Option func(*Watcher)

// WithDebounceTime sets the debounce duration for batching events.
func WithDebounceTime(d time.Duration) Option {
	return func(w *Watcher) {
		w.debounceTime = d
	}
}

// WithEventCallback sets a callback for file events.
func WithEventCallback(fn func(event string, path string)) Option {
	return func(w *Watcher) {
		w.onEvent = fn
	}
}

// New creates a new file watcher.
func New(root string, storeName string, st store.Store, emb embeddings.Service, cfg *config.Config, opts ...Option) (*Watcher, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	idx := indexer.New(st, emb, cfg)

	w := &Watcher{
		root:         absRoot,
		storeName:    storeName,
		store:        st,
		embedder:     emb,
		indexer:      idx,
		cfg:          cfg,
		debounce:     make(map[string]fsnotify.Op),
		debounceTime: 500 * time.Millisecond,
		onEvent:      func(string, string) {}, // noop default
	}

	for _, opt := range opts {
		opt(w)
	}

	return w, nil
}

// Start begins watching for file changes. Blocks until context is cancelled.
func (w *Watcher) Start(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	// Add all directories recursively
	if err := w.addDirectories(watcher); err != nil {
		return err
	}

	log.Info("Watching for file changes", "root", w.root)

	// Start debounce processor
	go w.processDebounced(ctx)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			w.handleEvent(event, watcher)

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			log.Error("Watcher error", "error", err)
		}
	}
}

// addDirectories recursively adds all directories to the watcher.
func (w *Watcher) addDirectories(watcher *fsnotify.Watcher) error {
	return filepath.WalkDir(w.root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if !d.IsDir() {
			return nil
		}

		// Skip hidden directories and common ignores
		name := d.Name()
		if strings.HasPrefix(name, ".") && name != "." {
			return filepath.SkipDir
		}
		if w.shouldSkipDir(name) {
			return filepath.SkipDir
		}

		if err := watcher.Add(path); err != nil {
			log.Debug("Failed to watch directory", "path", path, "error", err)
		}
		return nil
	})
}

// shouldSkipDir returns true if directory should not be watched.
func (w *Watcher) shouldSkipDir(name string) bool {
	skipDirs := []string{
		"node_modules", "vendor", "dist", "build", "out", "target",
		"bin", "obj", ".git", ".idea", ".vscode", "__pycache__",
		"coverage", ".nyc_output",
	}
	for _, skip := range skipDirs {
		if name == skip {
			return true
		}
	}
	return false
}

// handleEvent processes a single file system event.
func (w *Watcher) handleEvent(event fsnotify.Event, watcher *fsnotify.Watcher) {
	path := event.Name

	// Get relative path
	relPath, err := filepath.Rel(w.root, path)
	if err != nil {
		relPath = path
	}

	// Skip hidden files
	if strings.HasPrefix(filepath.Base(path), ".") {
		return
	}

	// For new directories, add to watcher
	if event.Has(fsnotify.Create) {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			if !w.shouldSkipDir(filepath.Base(path)) {
				watcher.Add(path)
				log.Debug("Added directory to watch", "path", relPath)
			}
			return
		}
	}

	// Skip directories for file operations
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return
	}

	// Skip non-indexable files
	if !w.isIndexableFile(path) {
		return
	}

	// Add to debounce queue
	w.debounceMu.Lock()
	w.debounce[path] = event.Op
	w.debounceMu.Unlock()
}

// isIndexableFile checks if a file should be indexed.
func (w *Watcher) isIndexableFile(path string) bool {
	// Check extension
	ext := strings.ToLower(filepath.Ext(path))
	if ext == "" {
		return false
	}

	// Check if it's a known language
	lang := fs.DetectLanguage(path)
	if lang == fs.LangUnknown {
		return false
	}

	// Check file size
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if w.cfg.Indexing.MaxFileSize > 0 && info.Size() > int64(w.cfg.Indexing.MaxFileSize) {
		return false
	}

	return true
}

// GetStoreName returns the store name for this watcher.
func (w *Watcher) GetStoreName() string {
	return w.storeName
}

// processDebounced processes debounced file events periodically.
func (w *Watcher) processDebounced(ctx context.Context) {
	ticker := time.NewTicker(w.debounceTime)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.flushDebounced(ctx)
		}
	}
}

// flushDebounced processes all pending debounced events.
func (w *Watcher) flushDebounced(ctx context.Context) {
	w.debounceMu.Lock()
	if len(w.debounce) == 0 {
		w.debounceMu.Unlock()
		return
	}

	// Copy and clear the map
	events := make(map[string]fsnotify.Op)
	for k, v := range w.debounce {
		events[k] = v
	}
	w.debounce = make(map[string]fsnotify.Op)
	w.debounceMu.Unlock()

	// Process each event
	for path, op := range events {
		select {
		case <-ctx.Done():
			return
		default:
		}

		relPath, _ := filepath.Rel(w.root, path)

		if op.Has(fsnotify.Remove) || op.Has(fsnotify.Rename) {
			// File was deleted or renamed away
			if err := w.handleDelete(ctx, path); err != nil {
				log.Error("Failed to handle delete", "path", relPath, "error", err)
			} else {
				w.onEvent("delete", relPath)
				log.Info("Removed from index", "file", relPath)
			}
		} else if op.Has(fsnotify.Create) || op.Has(fsnotify.Write) {
			// File was created or modified
			if err := w.handleModify(ctx, path); err != nil {
				log.Error("Failed to handle modify", "path", relPath, "error", err)
			} else {
				w.onEvent("index", relPath)
				log.Info("Indexed", "file", relPath)
			}
		}
	}
}

// handleModify re-indexes a modified or new file.
func (w *Watcher) handleModify(ctx context.Context, path string) error {
	// First delete any existing chunks for this file
	relPath, _ := filepath.Rel(w.root, path)
	_ = w.indexer.DeleteFile(w.storeName, relPath)

	// Now index the file
	return w.indexer.IndexSingleFile(ctx, w.storeName, w.root, path)
}

// handleDelete removes a file from the index.
func (w *Watcher) handleDelete(ctx context.Context, path string) error {
	relPath, _ := filepath.Rel(w.root, path)
	return w.indexer.DeleteFile(w.storeName, relPath)
}
