// Package cli implements the command-line interface for lgrep.
package cli

import (
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/nickcecere/lgrep/internal/config"
	"github.com/nickcecere/lgrep/internal/ui"
)

var (
	// Version information set at build time
	version = "dev"
	commit  = "none"
	date    = "unknown"

	// Global flags
	cfgFile string
	debug   bool
)

// SetVersionInfo sets the version information from build flags.
func SetVersionInfo(v, c, d string) {
	version = v
	commit = c
	date = d
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "lgrep [query] [path]",
	Short: "Local semantic code search",
	Long: `lgrep is a privacy-focused local semantic search tool for codebases.

It indexes your code using local embeddings (Ollama) or cloud providers (OpenAI),
stores vectors in SQLite, and enables natural language search with optional
LLM-powered Q&A.

Examples:
  # Index the current directory
  lgrep index

  # Search for relevant code
  lgrep "how does authentication work"

  # Search with Q&A mode (generates an answer using LLM)
  lgrep "how does authentication work" -a

  # Search a specific directory
  lgrep "database queries" ./src`,
	Args: cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		// If no args, show help
		if len(args) == 0 {
			return cmd.Help()
		}

		// Otherwise, run search command
		return runSearch(cmd, args)
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Set up logging based on debug flag
		if debug {
			log.SetLevel(log.DebugLevel)
			log.Debug("Debug logging enabled")
		}

		// Load configuration
		if err := config.Load(cfgFile); err != nil {
			log.Warn("Failed to load config", "error", err)
		}

		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Initialize UI styles and logger
	ui.InitLogger()

	// Persistent flags (available to all commands)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/lgrep/config.yaml)")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug logging")

	// Bind flags to viper
	_ = viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))

	// Add subcommands
	rootCmd.AddCommand(indexCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(watchCmd)
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(uninstallCmd)
}

// versionCmd shows version information
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("lgrep %s\n", version)
		fmt.Printf("  commit: %s\n", commit)
		fmt.Printf("  built:  %s\n", date)
	},
}

// runSearch is a convenience wrapper that delegates to the search command
func runSearch(cmd *cobra.Command, args []string) error {
	// Copy flags to module-level variables used by searchCmd
	if answer, _ := cmd.Flags().GetBool("answer"); answer {
		searchAnswer = true
	}
	if content, _ := cmd.Flags().GetBool("content"); content {
		searchContent = true
	}
	if limit, _ := cmd.Flags().GetString("limit"); limit != "" {
		searchLimit = limit
	}

	// Call the search handler directly instead of executing the command
	return runSearchCmd(cmd, args)
}

func init() {
	// Add search flags to root command for convenience
	rootCmd.Flags().BoolP("answer", "a", false, "generate an answer using LLM")
	rootCmd.Flags().BoolP("content", "c", false, "show content snippets in results")
	rootCmd.Flags().StringP("limit", "m", "10", "maximum number of results")
}
