// Package embeddings provides text embedding services for semantic search.
package embeddings

import (
	"context"
	"fmt"

	"github.com/nickcecere/lgrep/internal/config"
)

// Provider represents an embedding provider type.
type Provider string

const (
	ProviderOllama Provider = "ollama"
	ProviderOpenAI Provider = "openai"
)

// Service defines the interface for embedding services.
type Service interface {
	// Embed generates an embedding for the given text (for documents).
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedQuery generates an embedding for a query (may use different task prefix).
	EmbedQuery(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch generates embeddings for multiple texts.
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)

	// Dimensions returns the embedding dimensions for this model.
	Dimensions() int

	// Provider returns the provider name.
	Provider() Provider

	// ModelName returns the model name.
	ModelName() string
}

// Known model dimensions
var modelDimensions = map[string]int{
	// Ollama models
	"nomic-embed-text":       768,
	"mxbai-embed-large":      1024,
	"all-minilm":             384,
	"snowflake-arctic-embed": 1024,

	// OpenAI models
	"text-embedding-3-small": 1536,
	"text-embedding-3-large": 3072,
	"text-embedding-ada-002": 1536,
}

// GetModelDimensions returns the known dimensions for a model, or 0 if unknown.
func GetModelDimensions(model string) int {
	return modelDimensions[model]
}

// NewService creates an embedding service based on the configuration.
func NewService(cfg *config.Config) (Service, error) {
	switch cfg.Embeddings.Provider {
	case "ollama":
		return NewOllamaService(
			cfg.Embeddings.Ollama.URL,
			cfg.Embeddings.Ollama.Model,
		)
	case "openai":
		return NewOpenAIService(
			cfg.Embeddings.OpenAI.APIKey,
			cfg.Embeddings.OpenAI.Model,
			cfg.Embeddings.OpenAI.BaseURL,
			cfg.Embeddings.OpenAI.Dimensions,
		)
	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s", cfg.Embeddings.Provider)
	}
}

// NewServiceForStore creates an embedding service matching a store's configuration.
func NewServiceForStore(provider, model string, cfg *config.Config) (Service, error) {
	switch provider {
	case "ollama":
		return NewOllamaService(
			cfg.Embeddings.Ollama.URL,
			model,
		)
	case "openai":
		return NewOpenAIService(
			cfg.Embeddings.OpenAI.APIKey,
			model,
			cfg.Embeddings.OpenAI.BaseURL,
			cfg.Embeddings.OpenAI.Dimensions,
		)
	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s", provider)
	}
}
