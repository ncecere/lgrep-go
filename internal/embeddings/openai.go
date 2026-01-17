package embeddings

import (
	"context"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// OpenAIService implements the embedding service using OpenAI API.
type OpenAIService struct {
	client     openai.Client
	model      string
	dimensions int
}

// NewOpenAIService creates a new OpenAI embedding service.
func NewOpenAIService(apiKey, model, baseURL string, dimensions int) (*OpenAIService, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}

	// Build client options
	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}

	client := openai.NewClient(opts...)

	// Get dimensions for the model if not specified
	if dimensions == 0 {
		dimensions = GetModelDimensions(model)
		if dimensions == 0 {
			// Default for unknown models
			dimensions = 1536
			log.Debug("Unknown model dimensions, defaulting", "model", model, "dimensions", dimensions)
		}
	}

	return &OpenAIService{
		client:     client,
		model:      model,
		dimensions: dimensions,
	}, nil
}

// Embed generates an embedding for document text.
func (s *OpenAIService) Embed(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := s.embedTexts(ctx, []string{text})
	if err != nil {
		return nil, err
	}

	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	return embeddings[0], nil
}

// EmbedQuery generates an embedding for query text.
// OpenAI doesn't use task prefixes, so this is the same as Embed.
func (s *OpenAIService) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	return s.Embed(ctx, text)
}

// EmbedBatch generates embeddings for multiple texts.
func (s *OpenAIService) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	return s.embedTexts(ctx, texts)
}

// Dimensions returns the embedding dimensions.
func (s *OpenAIService) Dimensions() int {
	return s.dimensions
}

// Provider returns the provider name.
func (s *OpenAIService) Provider() Provider {
	return ProviderOpenAI
}

// ModelName returns the model name.
func (s *OpenAIService) ModelName() string {
	return s.model
}

// embedTexts performs the actual embedding request.
func (s *OpenAIService) embedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	log.Debug("Requesting embeddings from OpenAI", "model", s.model, "count", len(texts))

	// Build input union - convert strings to the union type
	inputUnion := openai.EmbeddingNewParamsInputUnion{
		OfArrayOfStrings: texts,
	}

	resp, err := s.client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Model: openai.EmbeddingModel(s.model),
		Input: inputUnion,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create embeddings: %w", err)
	}

	// Extract embeddings in order
	embeddings := make([][]float32, len(texts))
	for _, data := range resp.Data {
		idx := int(data.Index)
		if idx >= len(embeddings) {
			continue
		}
		// Convert float64 to float32
		embedding := make([]float32, len(data.Embedding))
		for i, v := range data.Embedding {
			embedding[i] = float32(v)
		}
		embeddings[idx] = embedding
	}

	// Update dimensions from response
	if len(embeddings) > 0 && len(embeddings[0]) > 0 {
		s.dimensions = len(embeddings[0])
	}

	return embeddings, nil
}
