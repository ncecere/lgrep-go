package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"

	"github.com/nickcecere/lgrep/internal/config"
	"github.com/nickcecere/lgrep/internal/embeddings"
	"github.com/nickcecere/lgrep/internal/fs"
	"github.com/nickcecere/lgrep/internal/indexer"
	"github.com/nickcecere/lgrep/internal/store"
	"github.com/nickcecere/lgrep/internal/ui"
)

var (
	indexForce      bool
	indexDryRun     bool
	indexStore      string
	indexExtensions []string
	indexIgnore     []string
)

// indexCmd represents the index command
var indexCmd = &cobra.Command{
	Use:   "index [path]",
	Short: "Index files for semantic search",
	Long: `Index files in the specified directory (or current directory) for semantic search.

This command will:
1. Discover all text files in the directory
2. Split files into chunks
3. Generate embeddings for each chunk
4. Store embeddings in the local SQLite database

Examples:
  # Index current directory
  lgrep index

  # Index a specific directory
  lgrep index ./src

  # Force re-index (clear existing)
  lgrep index --force

  # Index only specific extensions
  lgrep index --ext .go --ext .ts

  # Preview what would be indexed
  lgrep index --dry-run`,
	Args: cobra.MaximumNArgs(1),
	RunE: runIndex,
}

func init() {
	indexCmd.Flags().BoolVarP(&indexForce, "force", "f", false, "force re-index all files")
	indexCmd.Flags().BoolVarP(&indexDryRun, "dry-run", "d", false, "preview without indexing")
	indexCmd.Flags().StringVar(&indexStore, "store", "", "store name (defaults to directory name)")
	indexCmd.Flags().StringSliceVarP(&indexExtensions, "ext", "e", nil, "file extensions to include (e.g., .go, .ts)")
	indexCmd.Flags().StringSliceVarP(&indexIgnore, "ignore", "i", nil, "additional patterns to ignore")
}

func runIndex(cmd *cobra.Command, args []string) error {
	// Get path to index
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	// Resolve to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Verify path exists and is a directory
	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("path does not exist: %s", absPath)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", absPath)
	}

	// Get configuration
	cfg := config.Get()

	// Determine store name
	storeName := indexStore
	if storeName == "" {
		storeName = filepath.Base(absPath)
	}

	log.Debug("Starting index",
		"path", absPath,
		"store", storeName,
		"force", indexForce,
		"dry-run", indexDryRun,
	)

	// Dry run mode - just show what would be indexed
	if indexDryRun {
		return runDryRun(absPath, cfg)
	}

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nInterrupted, cleaning up...")
		cancel()
	}()

	// Open store
	st, err := store.NewSQLiteStore(cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("failed to open store: %w", err)
	}
	defer st.Close()

	// Create embedding service
	emb, err := embeddings.NewService(cfg)
	if err != nil {
		return fmt.Errorf("failed to create embedding service: %w", err)
	}

	// Create indexer
	idx := indexer.New(st, emb, cfg)

	// Show progress
	fmt.Println(ui.Header.Render("Indexing " + storeName))
	fmt.Printf("Path: %s\n", absPath)
	fmt.Printf("Provider: %s (%s)\n", cfg.Embeddings.Provider, emb.ModelName())
	fmt.Println()

	startTime := time.Now()
	lastUpdate := time.Now()

	// Index with progress callback
	opts := indexer.IndexOptions{
		StoreName:      storeName,
		Path:           absPath,
		Extensions:     indexExtensions,
		IgnorePatterns: indexIgnore,
		Force:          indexForce,
		BatchSize:      50,
		OnProgress: func(p indexer.Progress) {
			// Throttle updates to every 100ms
			if time.Since(lastUpdate) < 100*time.Millisecond {
				return
			}
			lastUpdate = time.Now()

			// Clear line and print progress
			fmt.Printf("\r\033[K")
			if p.TotalFiles > 0 {
				pct := float64(p.ProcessedFiles) / float64(p.TotalFiles) * 100
				fmt.Printf("Progress: %d/%d files (%.0f%%) | Chunks: %d | %s",
					p.ProcessedFiles, p.TotalFiles, pct, p.ProcessedChunks,
					truncatePath(p.CurrentFile, 40))
			}
		},
	}

	err = idx.Index(ctx, opts)

	// Clear progress line
	fmt.Printf("\r\033[K")

	if err != nil {
		if ctx.Err() != nil {
			fmt.Println(ui.Warning.Render("Indexing cancelled"))
			return nil
		}
		return fmt.Errorf("indexing failed: %w", err)
	}

	// Show final stats
	duration := time.Since(startTime).Round(time.Millisecond)
	stats, err := idx.Stats(storeName)
	if err != nil {
		log.Warn("Failed to get stats", "error", err)
	} else {
		fmt.Println(ui.Success.Render("Indexing complete!"))
		fmt.Println()
		fmt.Printf("  Files:    %d\n", stats.FileCount)
		fmt.Printf("  Chunks:   %d\n", stats.ChunkCount)
		fmt.Printf("  Size:     %s\n", formatBytes(stats.TotalSize))
		fmt.Printf("  Duration: %s\n", duration)
	}

	return nil
}

// runDryRun shows what would be indexed without actually indexing.
func runDryRun(path string, cfg *config.Config) error {
	fmt.Println(ui.Header.Render("Dry Run - Preview"))
	fmt.Printf("Path: %s\n\n", path)

	walker, err := fs.NewFileWalker(fs.WalkOptions{
		Root:           path,
		MaxFileSize:    int64(cfg.Indexing.MaxFileSize),
		MaxFileCount:   cfg.Indexing.MaxFileCount,
		IgnorePatterns: append(cfg.Ignore, indexIgnore...),
		UseGitignore:   true,
		Extensions:     indexExtensions,
	})
	if err != nil {
		return fmt.Errorf("failed to create file walker: %w", err)
	}

	var files []fs.FileInfo
	err = walker.Walk(func(fi fs.FileInfo) error {
		files = append(files, fi)
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to walk directory: %w", err)
	}

	stats := walker.Stats()

	// Show files by language
	byLang := make(map[string]int)
	var totalSize int64
	for _, f := range files {
		lang := f.Language
		if lang == "" {
			lang = "other"
		}
		byLang[lang]++
		totalSize += f.Size
	}

	fmt.Println("Files to index:")
	for lang, count := range byLang {
		fmt.Printf("  %-15s %d\n", lang+":", count)
	}
	fmt.Println()
	fmt.Printf("Total files:   %d\n", len(files))
	fmt.Printf("Total size:    %s\n", formatBytes(totalSize))
	fmt.Printf("Skipped:       %d files, %d directories\n", stats.FilesSkipped, stats.DirsSkipped)

	if len(files) > 0 {
		fmt.Println("\nFirst 10 files:")
		for i, f := range files {
			if i >= 10 {
				fmt.Printf("  ... and %d more\n", len(files)-10)
				break
			}
			fmt.Printf("  %s (%s)\n", f.RelPath, formatBytes(f.Size))
		}
	}

	return nil
}

// truncatePath shortens a path for display.
func truncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return "..." + path[len(path)-maxLen+3:]
}

// formatBytes formats bytes as human-readable string.
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// listCmd represents the list command for stores
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List indexed stores",
	Long:  `List all indexed stores with their statistics.`,
	RunE:  runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	cfg := config.Get()

	st, err := store.NewSQLiteStore(cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("failed to open store: %w", err)
	}
	defer st.Close()

	stores, err := st.ListStores()
	if err != nil {
		return fmt.Errorf("failed to list stores: %w", err)
	}

	if len(stores) == 0 {
		fmt.Println("No indexed stores found.")
		fmt.Println("\nRun 'lgrep index [path]' to create one.")
		return nil
	}

	fmt.Println(ui.Header.Render("Indexed Stores"))
	fmt.Println()

	for _, s := range stores {
		stats, err := st.GetStats(s.ID)
		if err != nil {
			log.Warn("Failed to get stats", "store", s.Name, "error", err)
			continue
		}

		fmt.Printf("%s\n", ui.Highlight.Render(s.Name))
		fmt.Printf("  Path:     %s\n", s.RootPath)
		fmt.Printf("  Model:    %s (%s)\n", s.EmbeddingModel, s.EmbeddingProvider)
		fmt.Printf("  Files:    %d\n", stats.FileCount)
		fmt.Printf("  Chunks:   %d\n", stats.ChunkCount)
		fmt.Printf("  Size:     %s\n", formatBytes(stats.TotalSize))
		fmt.Printf("  Updated:  %s\n", s.UpdatedAt.Format("2006-01-02 15:04:05"))
		fmt.Println()
	}

	return nil
}

// deleteCmd represents the delete command for stores
var deleteCmd = &cobra.Command{
	Use:   "delete <store>",
	Short: "Delete an indexed store",
	Long:  `Delete an indexed store and all its data.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runDelete,
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}

func runDelete(cmd *cobra.Command, args []string) error {
	storeName := args[0]
	cfg := config.Get()

	st, err := store.NewSQLiteStore(cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("failed to open store: %w", err)
	}
	defer st.Close()

	// Check if store exists
	_, err = st.GetStore(storeName)
	if err != nil {
		return fmt.Errorf("store not found: %s", storeName)
	}

	// Confirm deletion
	fmt.Printf("Delete store '%s'? This will remove all indexed data. [y/N]: ", storeName)
	var confirm string
	fmt.Scanln(&confirm)
	if strings.ToLower(confirm) != "y" {
		fmt.Println("Cancelled.")
		return nil
	}

	if err := st.DeleteStore(storeName); err != nil {
		return fmt.Errorf("failed to delete store: %w", err)
	}

	fmt.Println(ui.Success.Render(fmt.Sprintf("Store '%s' deleted.", storeName)))
	return nil
}
