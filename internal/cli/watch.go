package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"

	"github.com/nickcecere/lgrep/internal/config"
	"github.com/nickcecere/lgrep/internal/embeddings"
	"github.com/nickcecere/lgrep/internal/indexer"
	"github.com/nickcecere/lgrep/internal/store"
	"github.com/nickcecere/lgrep/internal/ui"
	"github.com/nickcecere/lgrep/internal/watcher"
)

var (
	watchNoInitial bool
)

// watchCmd represents the watch command.
var watchCmd = &cobra.Command{
	Use:   "watch [path]",
	Short: "Watch for file changes and auto-reindex",
	Long: `Watch a directory for file changes and automatically re-index modified files.

This command first performs an initial index of the directory (unless --no-initial
is specified), then watches for changes and updates the index in real-time.

Examples:
  # Watch current directory
  lgrep watch

  # Watch a specific directory
  lgrep watch ./src

  # Skip initial sync (assumes already indexed)
  lgrep watch --no-initial`,
	Args: cobra.MaximumNArgs(1),
	RunE: runWatchCmd,
}

func init() {
	watchCmd.Flags().BoolVar(&watchNoInitial, "no-initial", false, "skip initial index sync")
}

func runWatchCmd(cmd *cobra.Command, args []string) error {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	// Resolve absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Check path exists and is a directory
	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("path does not exist: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", absPath)
	}

	// Get configuration
	cfg := config.Get()

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nShutting down...")
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

	// Determine store name
	storeName := filepath.Base(absPath)

	// Create indexer for initial sync
	idx := indexer.New(st, emb, cfg)

	// Perform initial sync unless --no-initial is set
	if !watchNoInitial {
		fmt.Println(ui.Header.Render("Initial Index"))
		fmt.Printf("Path: %s\n", absPath)
		fmt.Printf("Provider: %s (%s)\n\n", cfg.Embeddings.Provider, cfg.Embeddings.Ollama.Model)

		stopSpinner := make(chan struct{})
		spinnerDone := make(chan struct{})
		go showSpinner("Indexing files", stopSpinner, spinnerDone)

		opts := indexer.IndexOptions{
			StoreName: storeName,
			Path:      absPath,
			Force:     false,
			BatchSize: 50, // Default batch size
			OnProgress: func(p indexer.Progress) {
				// Progress is shown via spinner
			},
		}

		err = idx.Index(ctx, opts)

		close(stopSpinner)
		<-spinnerDone

		if err != nil {
			if ctx.Err() != nil {
				return nil // User cancelled
			}
			return fmt.Errorf("initial index failed: %w", err)
		}

		// Show stats
		storeRecord, _ := st.GetStore(storeName)
		if storeRecord != nil {
			stats, _ := st.GetStats(storeRecord.ID)
			if stats != nil {
				fmt.Printf("Initial index complete: %d files, %d chunks\n\n",
					stats.FileCount, stats.ChunkCount)
			}
		}
	}

	// Create watcher
	w, err := watcher.New(
		absPath,
		storeName,
		st,
		emb,
		cfg,
		watcher.WithDebounceTime(500*time.Millisecond),
		watcher.WithEventCallback(func(event, path string) {
			log.Debug("File event", "event", event, "path", path)
		}),
	)
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}

	// Start watching
	fmt.Println(ui.Header.Render("Watching for Changes"))
	fmt.Printf("Directory: %s\n", absPath)
	fmt.Println("Press Ctrl+C to stop.")
	fmt.Println()

	return w.Start(ctx)
}
