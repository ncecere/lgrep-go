package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/log"
)

// Task prefixes for specific models
var taskPrefixes = map[string]struct {
	document string
	query    string
}{
	"nomic-embed-text": {
		document: "search_document: ",
		query:    "search_query: ",
	},
	"mxbai-embed-large": {
		document: "", // No prefix for documents
		query:    "Represent this sentence for searching relevant passages: ",
	},
}

// OllamaService implements the embedding service using Ollama.
type OllamaService struct {
	baseURL    string
	model      string
	dimensions int
	client     *http.Client
}

// ollamaEmbedRequest is the request body for the Ollama embed API.
type ollamaEmbedRequest struct {
	Model     string   `json:"model"`
	Input     []string `json:"input"`
	KeepAlive string   `json:"keep_alive,omitempty"`
	Truncate  bool     `json:"truncate,omitempty"`
}

// ollamaEmbedResponse is the response from the Ollama embed API.
type ollamaEmbedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

// NewOllamaService creates a new Ollama embedding service.
func NewOllamaService(baseURL, model string) (*OllamaService, error) {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	// Get dimensions for the model
	dimensions := GetModelDimensions(model)
	if dimensions == 0 {
		// Default to 768 if unknown, will be corrected on first embed
		dimensions = 768
		log.Debug("Unknown model dimensions, defaulting", "model", model, "dimensions", dimensions)
	}

	return &OllamaService{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		model:      model,
		dimensions: dimensions,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}, nil
}

// Embed generates an embedding for document text.
func (s *OllamaService) Embed(ctx context.Context, text string) ([]float32, error) {
	// Apply document task prefix if applicable
	prefixedText := s.applyPrefix(text, false)

	embeddings, err := s.embedTexts(ctx, []string{prefixedText})
	if err != nil {
		return nil, err
	}

	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	return embeddings[0], nil
}

// EmbedQuery generates an embedding for query text.
func (s *OllamaService) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	// Apply query task prefix if applicable
	prefixedText := s.applyPrefix(text, true)

	embeddings, err := s.embedTexts(ctx, []string{prefixedText})
	if err != nil {
		return nil, err
	}

	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	return embeddings[0], nil
}

// EmbedBatch generates embeddings for multiple document texts.
func (s *OllamaService) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// Apply document task prefix to all texts
	prefixedTexts := make([]string, len(texts))
	for i, text := range texts {
		prefixedTexts[i] = s.applyPrefix(text, false)
	}

	return s.embedTexts(ctx, prefixedTexts)
}

// Dimensions returns the embedding dimensions.
func (s *OllamaService) Dimensions() int {
	return s.dimensions
}

// Provider returns the provider name.
func (s *OllamaService) Provider() Provider {
	return ProviderOllama
}

// ModelName returns the model name.
func (s *OllamaService) ModelName() string {
	return s.model
}

// applyPrefix applies the appropriate task prefix for the model.
func (s *OllamaService) applyPrefix(text string, isQuery bool) string {
	prefixes, ok := taskPrefixes[s.model]
	if !ok {
		return text
	}

	if isQuery {
		return prefixes.query + text
	}
	return prefixes.document + text
}

// embedTexts performs the actual embedding request.
func (s *OllamaService) embedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	reqBody := ollamaEmbedRequest{
		Model:    s.model,
		Input:    texts,
		Truncate: true,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := s.baseURL + "/api/embed"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	log.Debug("Requesting embeddings from Ollama", "model", s.model, "count", len(texts))

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	var result ollamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Update dimensions if we got a response
	if len(result.Embeddings) > 0 && len(result.Embeddings[0]) > 0 {
		s.dimensions = len(result.Embeddings[0])
	}

	return result.Embeddings, nil
}
