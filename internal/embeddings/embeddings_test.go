package embeddings

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nickcecere/lgrep/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetModelDimensions tests known model dimension lookups.
func TestGetModelDimensions(t *testing.T) {
	tests := []struct {
		model    string
		expected int
	}{
		// Ollama models
		{"nomic-embed-text", 768},
		{"mxbai-embed-large", 1024},
		{"all-minilm", 384},
		{"snowflake-arctic-embed", 1024},

		// OpenAI models
		{"text-embedding-3-small", 1536},
		{"text-embedding-3-large", 3072},
		{"text-embedding-ada-002", 1536},

		// Unknown model
		{"unknown-model", 0},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			dims := GetModelDimensions(tt.model)
			assert.Equal(t, tt.expected, dims)
		})
	}
}

// TestNewOllamaService tests Ollama service creation.
func TestNewOllamaService(t *testing.T) {
	t.Run("with default URL", func(t *testing.T) {
		svc, err := NewOllamaService("", "nomic-embed-text")
		require.NoError(t, err)

		assert.Equal(t, "http://localhost:11434", svc.baseURL)
		assert.Equal(t, "nomic-embed-text", svc.model)
		assert.Equal(t, 768, svc.dimensions)
		assert.Equal(t, ProviderOllama, svc.Provider())
		assert.Equal(t, "nomic-embed-text", svc.ModelName())
	})

	t.Run("with custom URL", func(t *testing.T) {
		svc, err := NewOllamaService("http://custom:8080/", "mxbai-embed-large")
		require.NoError(t, err)

		assert.Equal(t, "http://custom:8080", svc.baseURL) // trailing slash removed
		assert.Equal(t, "mxbai-embed-large", svc.model)
		assert.Equal(t, 1024, svc.dimensions)
	})

	t.Run("with unknown model defaults to 768", func(t *testing.T) {
		svc, err := NewOllamaService("", "custom-model")
		require.NoError(t, err)

		assert.Equal(t, 768, svc.dimensions)
	})
}

// TestNewOpenAIService tests OpenAI service creation.
func TestNewOpenAIService(t *testing.T) {
	t.Run("requires API key", func(t *testing.T) {
		_, err := NewOpenAIService("", "text-embedding-3-small", "", 0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "API key is required")
	})

	t.Run("with known model dimensions", func(t *testing.T) {
		svc, err := NewOpenAIService("sk-test", "text-embedding-3-small", "", 0)
		require.NoError(t, err)

		assert.Equal(t, "text-embedding-3-small", svc.model)
		assert.Equal(t, 1536, svc.dimensions)
		assert.Equal(t, ProviderOpenAI, svc.Provider())
		assert.Equal(t, "text-embedding-3-small", svc.ModelName())
	})

	t.Run("with custom dimensions", func(t *testing.T) {
		svc, err := NewOpenAIService("sk-test", "text-embedding-3-large", "", 512)
		require.NoError(t, err)

		assert.Equal(t, 512, svc.dimensions)
	})

	t.Run("with unknown model defaults to 1536", func(t *testing.T) {
		svc, err := NewOpenAIService("sk-test", "custom-model", "", 0)
		require.NoError(t, err)

		assert.Equal(t, 1536, svc.dimensions)
	})
}

// TestOllamaTaskPrefixes tests task prefix application.
func TestOllamaTaskPrefixes(t *testing.T) {
	t.Run("nomic-embed-text prefixes", func(t *testing.T) {
		svc, _ := NewOllamaService("", "nomic-embed-text")

		// Document prefix
		doc := svc.applyPrefix("test document", false)
		assert.Equal(t, "search_document: test document", doc)

		// Query prefix
		query := svc.applyPrefix("test query", true)
		assert.Equal(t, "search_query: test query", query)
	})

	t.Run("mxbai-embed-large prefixes", func(t *testing.T) {
		svc, _ := NewOllamaService("", "mxbai-embed-large")

		// Document has no prefix
		doc := svc.applyPrefix("test document", false)
		assert.Equal(t, "test document", doc)

		// Query has prefix
		query := svc.applyPrefix("test query", true)
		assert.Equal(t, "Represent this sentence for searching relevant passages: test query", query)
	})

	t.Run("unknown model has no prefix", func(t *testing.T) {
		svc, _ := NewOllamaService("", "unknown-model")

		doc := svc.applyPrefix("test", false)
		query := svc.applyPrefix("test", true)

		assert.Equal(t, "test", doc)
		assert.Equal(t, "test", query)
	})
}

// mockOllamaServer creates a test server that simulates Ollama's embed API.
func mockOllamaServer(t *testing.T, dims int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/embed", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req ollamaEmbedRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		// Generate fake embeddings
		embeddings := make([][]float32, len(req.Input))
		for i := range req.Input {
			embedding := make([]float32, dims)
			for j := range embedding {
				embedding[j] = float32(i+1) * 0.1 // Predictable values
			}
			embeddings[i] = embedding
		}

		resp := ollamaEmbedResponse{Embeddings: embeddings}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

// TestOllamaEmbed tests the Ollama embedding methods with a mock server.
func TestOllamaEmbed(t *testing.T) {
	server := mockOllamaServer(t, 768)
	defer server.Close()

	svc, err := NewOllamaService(server.URL, "nomic-embed-text")
	require.NoError(t, err)

	t.Run("Embed single text", func(t *testing.T) {
		embedding, err := svc.Embed(context.Background(), "test document")
		require.NoError(t, err)

		assert.Len(t, embedding, 768)
		assert.Equal(t, float32(0.1), embedding[0])
	})

	t.Run("EmbedQuery single text", func(t *testing.T) {
		embedding, err := svc.EmbedQuery(context.Background(), "test query")
		require.NoError(t, err)

		assert.Len(t, embedding, 768)
	})

	t.Run("EmbedBatch multiple texts", func(t *testing.T) {
		texts := []string{"doc1", "doc2", "doc3"}
		embeddings, err := svc.EmbedBatch(context.Background(), texts)
		require.NoError(t, err)

		assert.Len(t, embeddings, 3)
		for i, emb := range embeddings {
			assert.Len(t, emb, 768)
			// Each embedding should have predictable values
			assert.Equal(t, float32(i+1)*0.1, emb[0])
		}
	})

	t.Run("EmbedBatch empty returns nil", func(t *testing.T) {
		embeddings, err := svc.EmbedBatch(context.Background(), []string{})
		require.NoError(t, err)
		assert.Nil(t, embeddings)
	})
}

// TestOllamaErrorHandling tests error cases.
func TestOllamaErrorHandling(t *testing.T) {
	t.Run("server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("model not found"))
		}))
		defer server.Close()

		svc, _ := NewOllamaService(server.URL, "nomic-embed-text")
		_, err := svc.Embed(context.Background(), "test")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "status 500")
		assert.Contains(t, err.Error(), "model not found")
	})

	t.Run("connection error", func(t *testing.T) {
		svc, _ := NewOllamaService("http://localhost:99999", "nomic-embed-text")
		_, err := svc.Embed(context.Background(), "test")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to make request")
	})

	t.Run("invalid JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("not json"))
		}))
		defer server.Close()

		svc, _ := NewOllamaService(server.URL, "nomic-embed-text")
		_, err := svc.Embed(context.Background(), "test")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to decode response")
	})
}

// TestOllamaDimensionUpdate tests that dimensions are updated from response.
func TestOllamaDimensionUpdate(t *testing.T) {
	// Server returns 512 dimensions instead of expected 768
	server := mockOllamaServer(t, 512)
	defer server.Close()

	svc, _ := NewOllamaService(server.URL, "nomic-embed-text")
	assert.Equal(t, 768, svc.Dimensions()) // Initial expectation

	_, err := svc.Embed(context.Background(), "test")
	require.NoError(t, err)

	// Dimensions should be updated from response
	assert.Equal(t, 512, svc.Dimensions())
}

// TestNewService tests the factory function.
func TestNewService(t *testing.T) {
	t.Run("creates Ollama service", func(t *testing.T) {
		cfg := &config.Config{
			Embeddings: config.EmbeddingsConfig{
				Provider: "ollama",
				Ollama: config.OllamaEmbedConfig{
					URL:   "http://localhost:11434",
					Model: "nomic-embed-text",
				},
			},
		}

		svc, err := NewService(cfg)
		require.NoError(t, err)

		assert.Equal(t, ProviderOllama, svc.Provider())
		assert.Equal(t, "nomic-embed-text", svc.ModelName())
	})

	t.Run("creates OpenAI service", func(t *testing.T) {
		cfg := &config.Config{
			Embeddings: config.EmbeddingsConfig{
				Provider: "openai",
				OpenAI: config.OpenAIEmbedConfig{
					APIKey: "sk-test",
					Model:  "text-embedding-3-small",
				},
			},
		}

		svc, err := NewService(cfg)
		require.NoError(t, err)

		assert.Equal(t, ProviderOpenAI, svc.Provider())
		assert.Equal(t, "text-embedding-3-small", svc.ModelName())
	})

	t.Run("returns error for unsupported provider", func(t *testing.T) {
		cfg := &config.Config{
			Embeddings: config.EmbeddingsConfig{
				Provider: "unsupported",
			},
		}

		_, err := NewService(cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported embedding provider")
	})
}

// TestNewServiceForStore tests the store-based factory function.
func TestNewServiceForStore(t *testing.T) {
	cfg := &config.Config{
		Embeddings: config.EmbeddingsConfig{
			Provider: "ollama",
			Ollama: config.OllamaEmbedConfig{
				URL:   "http://localhost:11434",
				Model: "nomic-embed-text",
			},
			OpenAI: config.OpenAIEmbedConfig{
				APIKey: "sk-test",
				Model:  "text-embedding-3-small",
			},
		},
	}

	t.Run("creates Ollama service for store", func(t *testing.T) {
		svc, err := NewServiceForStore("ollama", "mxbai-embed-large", cfg)
		require.NoError(t, err)

		assert.Equal(t, ProviderOllama, svc.Provider())
		assert.Equal(t, "mxbai-embed-large", svc.ModelName())
	})

	t.Run("creates OpenAI service for store", func(t *testing.T) {
		svc, err := NewServiceForStore("openai", "text-embedding-ada-002", cfg)
		require.NoError(t, err)

		assert.Equal(t, ProviderOpenAI, svc.Provider())
		assert.Equal(t, "text-embedding-ada-002", svc.ModelName())
	})

	t.Run("returns error for unsupported provider", func(t *testing.T) {
		_, err := NewServiceForStore("unsupported", "model", cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported embedding provider")
	})
}

// TestContextCancellation tests that operations respect context cancellation.
func TestContextCancellation(t *testing.T) {
	// Server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if context was cancelled
		select {
		case <-r.Context().Done():
			return
		default:
			// Respond normally (in real test we'd add delay)
			resp := ollamaEmbedResponse{Embeddings: [][]float32{{0.1}}}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	svc, _ := NewOllamaService(server.URL, "nomic-embed-text")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := svc.Embed(ctx, "test")
	assert.Error(t, err)
}
