// Package llm provides LLM services for Q&A functionality.
package llm

import (
	"context"
	"fmt"

	"github.com/nickcecere/lgrep/internal/config"
)

// Provider represents an LLM provider type.
type Provider string

const (
	ProviderOllama    Provider = "ollama"
	ProviderOpenAI    Provider = "openai"
	ProviderAnthropic Provider = "anthropic"
)

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"` // "system", "user", or "assistant"
	Content string `json:"content"`
}

// CompletionOptions configures the completion request.
type CompletionOptions struct {
	// Temperature controls randomness (0-1).
	Temperature float64

	// MaxTokens limits the response length.
	MaxTokens int

	// Stream enables streaming responses.
	Stream bool
}

// DefaultCompletionOptions returns sensible defaults.
func DefaultCompletionOptions() CompletionOptions {
	return CompletionOptions{
		Temperature: 0.7,
		MaxTokens:   2048,
		Stream:      false,
	}
}

// Service defines the interface for LLM services.
type Service interface {
	// Complete generates a completion for the given messages.
	Complete(ctx context.Context, messages []Message, opts CompletionOptions) (string, error)

	// CompleteStream generates a streaming completion.
	CompleteStream(ctx context.Context, messages []Message, opts CompletionOptions) (<-chan string, <-chan error)

	// Provider returns the provider name.
	Provider() Provider

	// ModelName returns the model name.
	ModelName() string
}

// NewService creates an LLM service based on the configuration.
func NewService(cfg *config.Config) (Service, error) {
	switch cfg.LLM.Provider {
	case "ollama":
		return NewOllamaService(
			cfg.LLM.Ollama.URL,
			cfg.LLM.Ollama.Model,
		)
	case "openai":
		return NewOpenAIService(
			cfg.LLM.OpenAI.APIKey,
			cfg.LLM.OpenAI.Model,
			cfg.LLM.OpenAI.BaseURL,
		)
	case "anthropic":
		return NewAnthropicService(
			cfg.LLM.Anthropic.APIKey,
			cfg.LLM.Anthropic.Model,
		)
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", cfg.LLM.Provider)
	}
}
