package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"

	"github.com/nickcecere/lgrep/internal/config"
	"github.com/nickcecere/lgrep/internal/store"
	"github.com/nickcecere/lgrep/internal/ui"
)

var (
	statusStore string
	statusAll   bool
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show index status and statistics",
	Long: `Display information about indexed stores including:
- Number of indexed files
- Number of chunks
- Embedding provider and model used
- Last indexing time

Examples:
  # Show status for current directory's store
  lgrep status

  # Show status for a specific store
  lgrep status --store myproject

  # Show all stores
  lgrep status --all`,
	RunE: runStatus,
}

func init() {
	statusCmd.Flags().StringVar(&statusStore, "store", "", "specific store to show status for")
	statusCmd.Flags().BoolVar(&statusAll, "all", false, "show all stores")
}

func runStatus(cmd *cobra.Command, args []string) error {
	log.Debug("Showing status", "store", statusStore, "all", statusAll)

	cfg := config.Get()

	// Open store
	st, err := store.NewSQLiteStore(cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("failed to open store: %w", err)
	}
	defer st.Close()

	// Get all stores
	stores, err := st.ListStores()
	if err != nil {
		return fmt.Errorf("failed to list stores: %w", err)
	}

	if len(stores) == 0 {
		fmt.Println("No indexed stores found.")
		fmt.Println()
		fmt.Println("Run 'lgrep index [path]' to create one.")
		return nil
	}

	// Filter stores if needed
	var displayStores []store.StoreRecord
	if statusAll {
		displayStores = stores
	} else if statusStore != "" {
		for _, s := range stores {
			if s.Name == statusStore {
				displayStores = append(displayStores, s)
				break
			}
		}
		if len(displayStores) == 0 {
			return fmt.Errorf("store not found: %s", statusStore)
		}
	} else {
		// Try to find store for current directory
		cwd, _ := os.Getwd()
		cwdName := filepath.Base(cwd)

		// First try exact match on current directory name
		for _, s := range stores {
			if s.Name == cwdName || s.RootPath == cwd {
				displayStores = append(displayStores, s)
				break
			}
		}

		// If not found, show all stores
		if len(displayStores) == 0 {
			displayStores = stores
		}
	}

	// Display stores
	fmt.Println(ui.Header.Render("Index Status"))
	fmt.Println()

	for i, s := range displayStores {
		stats, err := st.GetStats(s.ID)
		if err != nil {
			log.Warn("Failed to get stats", "store", s.Name, "error", err)
			continue
		}

		// Store header
		fmt.Printf("%s %s\n",
			ui.Highlight.Render("Store:"),
			ui.Bold.Render(s.Name),
		)

		// Path
		fmt.Printf("  %s %s\n",
			ui.Dim.Render("Path:"),
			s.RootPath,
		)

		// Check if path exists
		if _, err := os.Stat(s.RootPath); os.IsNotExist(err) {
			fmt.Printf("  %s\n", ui.Warning.Render("(path no longer exists)"))
		}

		// Embedding info
		fmt.Printf("  %s %s (%s)\n",
			ui.Dim.Render("Model:"),
			s.EmbeddingModel,
			s.EmbeddingProvider,
		)
		fmt.Printf("  %s %d\n",
			ui.Dim.Render("Dimensions:"),
			s.EmbeddingDimensions,
		)

		// Stats
		fmt.Printf("  %s %d files, %d chunks\n",
			ui.Dim.Render("Indexed:"),
			stats.FileCount,
			stats.ChunkCount,
		)
		fmt.Printf("  %s %s\n",
			ui.Dim.Render("Size:"),
			formatBytes(stats.TotalSize),
		)

		// Timestamps
		fmt.Printf("  %s %s\n",
			ui.Dim.Render("Created:"),
			formatTime(s.CreatedAt),
		)
		fmt.Printf("  %s %s\n",
			ui.Dim.Render("Updated:"),
			formatTime(s.UpdatedAt),
		)

		// Health status
		health := getHealthStatus(stats)
		fmt.Printf("  %s %s\n",
			ui.Dim.Render("Health:"),
			health,
		)

		if i < len(displayStores)-1 {
			fmt.Println()
		}
	}

	// Show summary if multiple stores
	if len(displayStores) > 1 {
		fmt.Println()
		fmt.Println(ui.Dim.Render(fmt.Sprintf("Total: %d stores", len(displayStores))))
	}

	// Show config info
	fmt.Println()
	fmt.Println(ui.Dim.Render("Configuration:"))
	fmt.Printf("  Database: %s\n", cfg.Database.Path)
	fmt.Printf("  Embedding Provider: %s\n", cfg.Embeddings.Provider)

	return nil
}

// formatTime formats a time for display.
func formatTime(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}

	// If today, show time only
	now := time.Now()
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return "today at " + t.Format("15:04")
	}

	// If this year, omit year
	if t.Year() == now.Year() {
		return t.Format("Jan 2 at 15:04")
	}

	return t.Format("Jan 2, 2006 at 15:04")
}

// getHealthStatus returns a health indicator based on stats.
func getHealthStatus(stats *store.StoreStats) string {
	if stats.FileCount == 0 {
		return ui.Warning.Render("empty (no files indexed)")
	}
	if stats.ChunkCount == 0 {
		return ui.Warning.Render("no chunks (re-index may be needed)")
	}

	// Calculate average chunks per file
	avgChunks := float64(stats.ChunkCount) / float64(stats.FileCount)
	if avgChunks < 0.5 {
		return ui.Warning.Render("low chunk count (check file filters)")
	}

	return ui.Success.Render("healthy")
}
