package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"

	"github.com/nickcecere/lgrep/internal/config"
	"github.com/nickcecere/lgrep/internal/embeddings"
	"github.com/nickcecere/lgrep/internal/indexer"
	"github.com/nickcecere/lgrep/internal/llm"
	"github.com/nickcecere/lgrep/internal/search"
	"github.com/nickcecere/lgrep/internal/store"
	"github.com/nickcecere/lgrep/internal/ui"
)

var (
	searchAnswer   bool
	searchContent  bool
	searchLimit    string
	searchStore    string
	searchMinScore float64
	searchContext  int
	searchJSON     bool
	searchNoSync   bool
)

// searchCmd represents the search command
var searchCmd = &cobra.Command{
	Use:   "search <query> [path]",
	Short: "Search indexed files using semantic similarity",
	Long: `Search for code using natural language queries.

The search uses vector similarity to find relevant code snippets
that match your query semantically, not just by keywords.

Examples:
  # Basic search
  lgrep search "how does authentication work"

  # Search with content preview
  lgrep search "database connection" -c

  # Search with LLM-generated answer (Q&A mode)
  lgrep search "how are errors handled" -a

  # Limit results
  lgrep search "api endpoints" -m 5
  
  # Filter by minimum similarity score
  lgrep search "error handling" --min-score 0.5`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runSearchCmd,
}

func init() {
	searchCmd.Flags().BoolVarP(&searchAnswer, "answer", "a", false, "generate an answer using LLM")
	searchCmd.Flags().BoolVarP(&searchContent, "content", "c", false, "show content snippets in results")
	searchCmd.Flags().StringVarP(&searchLimit, "limit", "m", "10", "maximum number of results")
	searchCmd.Flags().StringVar(&searchStore, "store", "", "store name (auto-detected if not specified)")
	searchCmd.Flags().Float64Var(&searchMinScore, "min-score", 0.0, "minimum similarity score (0-1)")
	searchCmd.Flags().IntVar(&searchContext, "context", 0, "lines of context to show")
	searchCmd.Flags().BoolVar(&searchJSON, "json", false, "output results as JSON")
	searchCmd.Flags().BoolVar(&searchNoSync, "no-sync", false, "skip auto-indexing if store not found")
}

func runSearchCmd(cmd *cobra.Command, args []string) error {
	query := args[0]
	path := "."
	if len(args) > 1 {
		path = args[1]
	}

	// Parse limit
	limit, err := strconv.Atoi(searchLimit)
	if err != nil || limit <= 0 {
		limit = 10
	}

	log.Debug("Starting search",
		"query", query,
		"path", path,
		"limit", limit,
		"store", searchStore,
	)

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
		fmt.Println("\nInterrupted")
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

	// Create searcher
	searcher := search.New(st, emb)

	// Determine store name
	storeName := searchStore
	if storeName == "" {
		// Try to auto-detect from path
		absPath, _ := filepath.Abs(path)
		storeRecord, _ := searcher.GetStoreForPath(absPath)
		if storeRecord != nil {
			storeName = storeRecord.Name
		} else {
			// Use directory name
			storeName = filepath.Base(absPath)
		}
	}

	// Verify store exists
	storeRecord, err := st.GetStore(storeName)
	if err != nil {
		return fmt.Errorf("failed to check store: %w", err)
	}
	if storeRecord == nil {
		// Store doesn't exist - auto-index if --no-sync is not set
		if searchNoSync {
			return fmt.Errorf("store '%s' not found. Run 'lgrep index' first or remove --no-sync", storeName)
		}

		// Auto-index the directory
		absPath, _ := filepath.Abs(path)
		if err := autoIndex(ctx, st, emb, cfg, storeName, absPath); err != nil {
			return fmt.Errorf("auto-index failed: %w", err)
		}

		// Re-fetch the store record
		storeRecord, err = st.GetStore(storeName)
		if err != nil || storeRecord == nil {
			return fmt.Errorf("failed to get store after indexing: %w", err)
		}
	}

	// Perform search
	opts := search.SearchOptions{
		StoreName:      storeName,
		TopK:           limit,
		MinScore:       searchMinScore,
		IncludeContent: searchContent || searchAnswer,
		ContextLines:   searchContext,
	}

	results, err := searcher.Search(ctx, query, opts)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		fmt.Println("No results found.")
		return nil
	}

	// Output results
	if searchJSON {
		return outputJSON(results)
	}

	// Q&A mode with LLM
	if searchAnswer {
		return runQA(ctx, query, results, cfg)
	}

	// Display results
	displayResults(results, storeRecord.RootPath, searchContent)

	return nil
}

// displayResults formats and displays search results.
func displayResults(results []search.Result, rootPath string, showContent bool) {
	fmt.Printf("Found %d results:\n\n", len(results))

	for i, r := range results {
		// Format file path (show relative path if possible)
		displayPath := r.RelativePath
		if displayPath == "" {
			displayPath = r.FilePath
		}

		// Score as percentage
		scoreStr := fmt.Sprintf("%.1f%%", r.Score*100)

		// Header line
		fmt.Printf("%s %s %s\n",
			ui.Highlight.Render(fmt.Sprintf("[%d]", i+1)),
			ui.FilePath.Render(displayPath),
			ui.ResultScore.Render(scoreStr),
		)

		// Line numbers
		if r.StartLine > 0 {
			lineInfo := fmt.Sprintf("Lines %d-%d", r.StartLine, r.EndLine)
			fmt.Printf("    %s\n", ui.LineNum.Render(lineInfo))
		}

		// Content preview
		if showContent && r.Content != "" {
			fmt.Println()
			displayContentHighlighted(r.Content, r.StartLine, displayPath)
		}

		fmt.Println()
	}
}

// displayContentHighlighted formats and displays code content with syntax highlighting.
func displayContentHighlighted(content string, startLine int, filename string) {
	// Get lexer based on filename
	lexer := lexers.Match(filename)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	// Use a terminal-friendly style
	style := styles.Get("dracula")
	if style == nil {
		style = styles.Fallback
	}

	// Use terminal16m (true color) formatter for best color support
	formatter := formatters.Get("terminal16m")
	if formatter == nil {
		formatter = formatters.Get("terminal256")
	}
	if formatter == nil {
		formatter = formatters.Fallback
	}

	lines := strings.Split(content, "\n")
	maxLines := 15 // Maximum lines to show

	if len(lines) > maxLines {
		// Show first and last few lines with highlighting
		showLines := maxLines / 2

		// Highlight first section
		firstContent := strings.Join(lines[:showLines], "\n")
		displayHighlightedLines(firstContent, startLine, lexer, style, formatter)

		fmt.Printf("    %s\n", ui.Dim.Render(fmt.Sprintf("    ... (%d lines omitted)", len(lines)-maxLines)))

		// Highlight last section
		lastContent := strings.Join(lines[len(lines)-showLines:], "\n")
		displayHighlightedLines(lastContent, startLine+len(lines)-showLines, lexer, style, formatter)
	} else {
		displayHighlightedLines(content, startLine, lexer, style, formatter)
	}
}

// displayHighlightedLines highlights and displays code with line numbers.
func displayHighlightedLines(content string, startLine int, lexer chroma.Lexer, style *chroma.Style, formatter chroma.Formatter) {
	// Tokenize the content
	iterator, err := lexer.Tokenise(nil, content)
	if err != nil {
		// Fallback to plain display
		displayPlainLines(content, startLine)
		return
	}

	// Render highlighted content to buffer
	var buf bytes.Buffer
	if err := formatter.Format(&buf, style, iterator); err != nil {
		displayPlainLines(content, startLine)
		return
	}

	// Split highlighted output by lines and add line numbers
	highlightedLines := strings.Split(buf.String(), "\n")
	for i, line := range highlightedLines {
		lineNum := startLine + i
		fmt.Printf("    %s %s\n",
			ui.LineNum.Render(fmt.Sprintf("%4d│", lineNum)),
			line,
		)
	}
}

// displayPlainLines displays content without highlighting (fallback).
func displayPlainLines(content string, startLine int) {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lineNum := startLine + i
		fmt.Printf("    %s %s\n",
			ui.LineNum.Render(fmt.Sprintf("%4d│", lineNum)),
			truncateLine(line, 80),
		)
	}
}

// truncateLine shortens a line for display.
func truncateLine(line string, maxLen int) string {
	// Replace tabs with spaces for consistent display
	line = strings.ReplaceAll(line, "\t", "    ")
	if len(line) <= maxLen {
		return line
	}
	return line[:maxLen-3] + "..."
}

// outputJSON outputs results as JSON.
func outputJSON(results []search.Result) error {
	// Simple JSON output without importing encoding/json to keep it simple
	fmt.Println("[")
	for i, r := range results {
		comma := ","
		if i == len(results)-1 {
			comma = ""
		}
		fmt.Printf(`  {"file": %q, "lines": [%d, %d], "score": %.4f}%s
`,
			r.RelativePath, r.StartLine, r.EndLine, r.Score, comma)
	}
	fmt.Println("]")
	return nil
}

// runQA generates an answer using the LLM with search results as context.
func runQA(ctx context.Context, query string, results []search.Result, cfg *config.Config) error {
	// Create LLM service
	llmService, err := llm.NewService(cfg)
	if err != nil {
		return fmt.Errorf("failed to create LLM service: %w", err)
	}

	// Create Q&A service
	qaService := llm.NewQAService(llmService)

	// Start spinner while generating (no Answer header yet)
	stopSpinner := make(chan struct{})
	spinnerDone := make(chan struct{})
	go showSpinner("Generating answer", stopSpinner, spinnerDone)

	// Use non-streaming mode
	opts := llm.DefaultQAOptions()
	opts.Stream = true // Still use stream internally for the channel API

	contentCh, errCh, sources := qaService.AnswerStream(ctx, query, results, opts)

	// Collect all content silently
	var contentBuilder strings.Builder
	for content := range contentCh {
		contentBuilder.WriteString(content)
	}

	// Stop spinner
	close(stopSpinner)
	<-spinnerDone

	// Check for errors
	if err := <-errCh; err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("answer generation failed: %w", err)
	}

	// Now show the Answer header
	fmt.Println(ui.Header.Render("Answer"))
	fmt.Println()

	// Render markdown with glamour
	rendered, err := renderMarkdown(contentBuilder.String())
	if err != nil {
		// Fallback to raw output if rendering fails
		fmt.Println(contentBuilder.String())
	} else {
		fmt.Print(rendered)
	}

	// Show sources
	if len(sources) > 0 {
		fmt.Println(ui.Dim.Render("Sources:"))
		for i, s := range sources {
			fmt.Printf("  [%d] %s (lines %d-%d)\n",
				i+1, s.RelativePath, s.StartLine, s.EndLine)
		}
	}

	return nil
}

// showSpinner displays an animated spinner until stopCh is closed.
func showSpinner(message string, stopCh <-chan struct{}, doneCh chan<- struct{}) {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()
	defer close(doneCh)

	i := 0
	for {
		select {
		case <-stopCh:
			// Clear spinner line
			fmt.Print("\r\033[2K")
			return
		case <-ticker.C:
			fmt.Printf("\r%s %s", ui.Highlight.Render(frames[i]), message)
			i = (i + 1) % len(frames)
		}
	}
}

// renderMarkdown renders markdown content using glamour.
func renderMarkdown(content string) (string, error) {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(100),
	)
	if err != nil {
		return "", err
	}
	return renderer.Render(content)
}

// autoIndex automatically indexes a directory before searching.
func autoIndex(ctx context.Context, st store.Store, emb embeddings.Service, cfg *config.Config, storeName, absPath string) error {
	fmt.Printf("Store '%s' not found. Auto-indexing...\n\n", storeName)

	// Start spinner
	stopSpinner := make(chan struct{})
	spinnerDone := make(chan struct{})
	go showSpinner("Indexing files", stopSpinner, spinnerDone)

	// Create indexer and run
	idx := indexer.New(st, emb, cfg)
	opts := indexer.IndexOptions{
		StoreName: storeName,
		Path:      absPath,
		Force:     false,
		BatchSize: 50,
	}

	err := idx.Index(ctx, opts)

	// Stop spinner
	close(stopSpinner)
	<-spinnerDone

	if err != nil {
		return err
	}

	// Show stats
	storeRecord, _ := st.GetStore(storeName)
	if storeRecord != nil {
		stats, _ := st.GetStats(storeRecord.ID)
		if stats != nil {
			fmt.Printf("Indexed %d files, %d chunks\n\n", stats.FileCount, stats.ChunkCount)
		}
	}

	return nil
}
