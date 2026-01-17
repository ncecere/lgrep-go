package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/nickcecere/lgrep/internal/search"
)

// QAService generates answers to questions using search results as context.
type QAService struct {
	llm Service
}

// QAOptions configures the Q&A generation.
type QAOptions struct {
	// Temperature controls creativity (0-1).
	Temperature float64

	// MaxTokens limits the response length.
	MaxTokens int

	// Stream enables streaming responses.
	Stream bool

	// MaxContextChunks limits how many search results to include.
	MaxContextChunks int
}

// DefaultQAOptions returns sensible defaults.
func DefaultQAOptions() QAOptions {
	return QAOptions{
		Temperature:      0.3, // Lower for more focused answers
		MaxTokens:        2048,
		Stream:           false,
		MaxContextChunks: 5,
	}
}

// QAResult contains the answer and its sources.
type QAResult struct {
	Answer  string          `json:"answer"`
	Sources []search.Result `json:"sources"`
}

// NewQAService creates a new Q&A service.
func NewQAService(llm Service) *QAService {
	return &QAService{llm: llm}
}

// Answer generates an answer to the question using the search results as context.
func (qa *QAService) Answer(ctx context.Context, question string, results []search.Result, opts QAOptions) (*QAResult, error) {
	if len(results) == 0 {
		return &QAResult{
			Answer:  "I couldn't find any relevant code to answer your question. Try rephrasing your query or indexing more files.",
			Sources: nil,
		}, nil
	}

	// Limit context chunks
	contextResults := results
	if opts.MaxContextChunks > 0 && len(results) > opts.MaxContextChunks {
		contextResults = results[:opts.MaxContextChunks]
	}

	// Build context from search results
	context := buildContext(contextResults)

	// Create the prompt
	messages := []Message{
		{
			Role:    "system",
			Content: systemPrompt,
		},
		{
			Role:    "user",
			Content: fmt.Sprintf("Question: %s\n\n%s", question, context),
		},
	}

	// Generate answer
	answer, err := qa.llm.Complete(ctx, messages, CompletionOptions{
		Temperature: opts.Temperature,
		MaxTokens:   opts.MaxTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate answer: %w", err)
	}

	return &QAResult{
		Answer:  answer,
		Sources: contextResults,
	}, nil
}

// AnswerStream generates a streaming answer.
func (qa *QAService) AnswerStream(ctx context.Context, question string, results []search.Result, opts QAOptions) (<-chan string, <-chan error, []search.Result) {
	if len(results) == 0 {
		contentCh := make(chan string, 1)
		errCh := make(chan error, 1)
		contentCh <- "I couldn't find any relevant code to answer your question. Try rephrasing your query or indexing more files."
		close(contentCh)
		close(errCh)
		return contentCh, errCh, nil
	}

	// Limit context chunks
	contextResults := results
	if opts.MaxContextChunks > 0 && len(results) > opts.MaxContextChunks {
		contextResults = results[:opts.MaxContextChunks]
	}

	// Build context from search results
	context := buildContext(contextResults)

	// Create the prompt
	messages := []Message{
		{
			Role:    "system",
			Content: systemPrompt,
		},
		{
			Role:    "user",
			Content: fmt.Sprintf("Question: %s\n\n%s", question, context),
		},
	}

	// Stream answer
	contentCh, errCh := qa.llm.CompleteStream(ctx, messages, CompletionOptions{
		Temperature: opts.Temperature,
		MaxTokens:   opts.MaxTokens,
		Stream:      true,
	})

	return contentCh, errCh, contextResults
}

// buildContext creates the context string from search results.
func buildContext(results []search.Result) string {
	var sb strings.Builder

	sb.WriteString("Here is the relevant code context:\n\n")

	for i, r := range results {
		sb.WriteString(fmt.Sprintf("--- Source [%d]: %s (lines %d-%d, %.0f%% match) ---\n",
			i+1, r.RelativePath, r.StartLine, r.EndLine, r.Score*100))
		sb.WriteString(r.Content)
		sb.WriteString("\n\n")
	}

	return sb.String()
}

// System prompt for Q&A.
const systemPrompt = `You are a helpful code assistant that answers questions about codebases.

Your role is to:
1. Analyze the provided code context carefully
2. Answer the user's question accurately based on the code
3. Reference specific files and line numbers when citing code
4. Be concise but thorough
5. If the code context doesn't contain enough information to answer, say so

When referencing code:
- Use [Source N] notation to cite specific sources
- Mention the file path when relevant
- Quote small code snippets when helpful

Format your answer in markdown when appropriate.`
