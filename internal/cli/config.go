package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nickcecere/lgrep/internal/config"
	"github.com/nickcecere/lgrep/internal/ui"
)

var configShowPath bool

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show or manage configuration",
	Long: `Display current configuration settings and config file locations.

Examples:
  # Show current configuration
  lgrep config

  # Show config file paths
  lgrep config --path`,
	RunE: runConfig,
}

func init() {
	configCmd.Flags().BoolVar(&configShowPath, "path", false, "show config file paths")
}

func runConfig(cmd *cobra.Command, args []string) error {
	if configShowPath {
		fmt.Println(ui.SectionTitle.Render("Configuration Paths"))
		fmt.Println()
		fmt.Printf("Global config: %s\n", config.GlobalConfigPath())
		fmt.Printf("Local config:  .lgreprc.yaml (searched from cwd upward)\n")
		fmt.Printf("Active config: %s\n", config.ConfigFilePath())
		fmt.Printf("Database:      %s\n", config.Get().Database.Path)
		return nil
	}

	// Show current configuration
	cfg := config.Get()

	fmt.Println(ui.SectionTitle.Render("Current Configuration"))
	fmt.Println()

	fmt.Println(ui.Bold.Render("Embeddings:"))
	fmt.Printf("  Provider: %s\n", cfg.Embeddings.Provider)
	fmt.Printf("  Ollama URL: %s\n", cfg.Embeddings.Ollama.URL)
	fmt.Printf("  Ollama Model: %s\n", cfg.Embeddings.Ollama.Model)
	fmt.Printf("  OpenAI Model: %s\n", cfg.Embeddings.OpenAI.Model)
	if cfg.Embeddings.OpenAI.BaseURL != "" {
		fmt.Printf("  OpenAI Base URL: %s\n", cfg.Embeddings.OpenAI.BaseURL)
	}
	fmt.Println()

	fmt.Println(ui.Bold.Render("LLM:"))
	fmt.Printf("  Provider: %s\n", cfg.LLM.Provider)
	fmt.Printf("  Ollama URL: %s\n", cfg.LLM.Ollama.URL)
	fmt.Printf("  Ollama Model: %s\n", cfg.LLM.Ollama.Model)
	fmt.Printf("  OpenAI Model: %s\n", cfg.LLM.OpenAI.Model)
	fmt.Printf("  Anthropic Model: %s\n", cfg.LLM.Anthropic.Model)
	fmt.Println()

	fmt.Println(ui.Bold.Render("Indexing:"))
	fmt.Printf("  Max File Size: %d bytes\n", cfg.Indexing.MaxFileSize)
	fmt.Printf("  Max File Count: %d\n", cfg.Indexing.MaxFileCount)
	fmt.Printf("  Chunk Size: %d\n", cfg.Indexing.ChunkSize)
	fmt.Printf("  Chunk Overlap: %d\n", cfg.Indexing.ChunkOverlap)
	fmt.Println()

	fmt.Println(ui.Bold.Render("Database:"))
	fmt.Printf("  Path: %s\n", cfg.Database.Path)
	fmt.Println()

	fmt.Println(ui.Bold.Render("Ignore Patterns:"))
	fmt.Printf("  %d patterns configured\n", len(cfg.Ignore))

	return nil
}
