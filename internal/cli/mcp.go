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
	"github.com/nickcecere/lgrep/internal/mcp"
	"github.com/nickcecere/lgrep/internal/store"
	"github.com/nickcecere/lgrep/internal/watcher"
)

var (
	mcpNoWatch bool
)

// mcpCmd represents the MCP server command.
var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server for AI agent integration",
	Long: `Start a Model Context Protocol (MCP) server for integration with AI coding agents.

The server communicates via stdin/stdout using JSON-RPC 2.0 and provides tools for:
  - lgrep_search: Semantic code search
  - lgrep_index: Index a directory

By default, the server also starts a background file watcher to keep the index
up-to-date. Use --no-watch to disable this.

This command is typically invoked by AI agents (Claude Code, OpenCode, Codex) and
not run directly by users.`,
	RunE: runMcpCmd,
}

func init() {
	mcpCmd.Flags().BoolVar(&mcpNoWatch, "no-watch", false, "disable background file watching")
}

func runMcpCmd(cmd *cobra.Command, args []string) error {
	// MCP server uses stdin/stdout for communication, so redirect logs to stderr
	log.SetOutput(os.Stderr)
	log.SetLevel(log.InfoLevel)

	// Get configuration
	cfg := config.Get()

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Info("Received signal, shutting down", "signal", sig)
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

	// Start background file watcher if enabled
	if !mcpNoWatch {
		go startBackgroundWatcher(ctx, st, emb, cfg)
	}

	// Create and run MCP server
	server := mcp.NewServer(st, emb, cfg)
	return server.Run(ctx)
}

// startBackgroundWatcher starts a file watcher for the current directory.
func startBackgroundWatcher(ctx context.Context, st store.Store, emb embeddings.Service, cfg *config.Config) {
	// Wait a bit before starting to let the MCP server initialize
	select {
	case <-ctx.Done():
		return
	case <-time.After(2 * time.Second):
	}

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		log.Error("Failed to get working directory", "error", err)
		return
	}

	absPath, err := filepath.Abs(cwd)
	if err != nil {
		log.Error("Failed to resolve path", "error", err)
		return
	}

	storeName := filepath.Base(absPath)

	log.Info("Starting background file watcher", "path", absPath)

	// Create watcher
	w, err := watcher.New(
		absPath,
		storeName,
		st,
		emb,
		cfg,
		watcher.WithDebounceTime(1*time.Second),
		watcher.WithEventCallback(func(event, path string) {
			log.Debug("Background watcher event", "event", event, "path", path)
		}),
	)
	if err != nil {
		log.Error("Failed to create watcher", "error", err)
		return
	}

	// Start watching (blocks until context is cancelled)
	if err := w.Start(ctx); err != nil && ctx.Err() == nil {
		log.Error("Watcher error", "error", err)
	}
}
