package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nickcecere/lgrep/internal/config"
	"github.com/nickcecere/lgrep/internal/search"
)

// TestNewService tests the factory function.
func TestNewService(t *testing.T) {
	t.Run("creates Ollama service", func(t *testing.T) {
		cfg := &config.Config{
			LLM: config.LLMConfig{
				Provider: "ollama",
				Ollama: config.OllamaLLMConfig{
					URL:   "http://localhost:11434",
					Model: "llama2",
				},
			},
		}

		svc, err := NewService(cfg)
		require.NoError(t, err)
		assert.Equal(t, ProviderOllama, svc.Provider())
		assert.Equal(t, "llama2", svc.ModelName())
	})

	t.Run("creates OpenAI service", func(t *testing.T) {
		cfg := &config.Config{
			LLM: config.LLMConfig{
				Provider: "openai",
				OpenAI: config.OpenAILLMConfig{
					APIKey: "sk-test",
					Model:  "gpt-4",
				},
			},
		}

		svc, err := NewService(cfg)
		require.NoError(t, err)
		assert.Equal(t, ProviderOpenAI, svc.Provider())
		assert.Equal(t, "gpt-4", svc.ModelName())
	})

	t.Run("creates Anthropic service", func(t *testing.T) {
		cfg := &config.Config{
			LLM: config.LLMConfig{
				Provider: "anthropic",
				Anthropic: config.AnthropicConfig{
					APIKey: "sk-ant-test",
					Model:  "claude-3-sonnet",
				},
			},
		}

		svc, err := NewService(cfg)
		require.NoError(t, err)
		assert.Equal(t, ProviderAnthropic, svc.Provider())
		assert.Equal(t, "claude-3-sonnet", svc.ModelName())
	})

	t.Run("returns error for unsupported provider", func(t *testing.T) {
		cfg := &config.Config{
			LLM: config.LLMConfig{
				Provider: "unsupported",
			},
		}

		_, err := NewService(cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported")
	})
}

// TestNewOllamaService tests Ollama service creation.
func TestNewOllamaService(t *testing.T) {
	t.Run("with default URL", func(t *testing.T) {
		svc, err := NewOllamaService("", "llama2")
		require.NoError(t, err)
		assert.Equal(t, "http://localhost:11434", svc.baseURL)
		assert.Equal(t, "llama2", svc.model)
	})

	t.Run("with custom URL", func(t *testing.T) {
		svc, err := NewOllamaService("http://custom:8080/", "mistral")
		require.NoError(t, err)
		assert.Equal(t, "http://custom:8080", svc.baseURL)
	})
}

// TestNewOpenAIService tests OpenAI service creation.
func TestNewOpenAIService(t *testing.T) {
	t.Run("requires API key", func(t *testing.T) {
		_, err := NewOpenAIService("", "gpt-4", "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "API key")
	})

	t.Run("with valid API key", func(t *testing.T) {
		svc, err := NewOpenAIService("sk-test", "gpt-4", "")
		require.NoError(t, err)
		assert.Equal(t, "gpt-4", svc.model)
	})
}

// TestNewAnthropicService tests Anthropic service creation.
func TestNewAnthropicService(t *testing.T) {
	t.Run("requires API key", func(t *testing.T) {
		_, err := NewAnthropicService("", "claude-3")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "API key")
	})

	t.Run("with valid API key", func(t *testing.T) {
		svc, err := NewAnthropicService("sk-ant-test", "claude-3")
		require.NoError(t, err)
		assert.Equal(t, "claude-3", svc.model)
	})
}

// mockOllamaServer creates a test server that simulates Ollama's chat API.
func mockOllamaServer(t *testing.T, response string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/chat", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

		var req ollamaChatRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		resp := ollamaChatResponse{
			Message: ollamaMessage{
				Role:    "assistant",
				Content: response,
			},
			Done: true,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

// TestOllamaComplete tests Ollama completion.
func TestOllamaComplete(t *testing.T) {
	server := mockOllamaServer(t, "Hello! How can I help you?")
	defer server.Close()

	svc, err := NewOllamaService(server.URL, "llama2")
	require.NoError(t, err)

	messages := []Message{
		{Role: "user", Content: "Hello"},
	}

	response, err := svc.Complete(context.Background(), messages, DefaultCompletionOptions())
	require.NoError(t, err)
	assert.Equal(t, "Hello! How can I help you?", response)
}

// TestOllamaCompleteError tests error handling.
func TestOllamaCompleteError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("model not found"))
	}))
	defer server.Close()

	svc, err := NewOllamaService(server.URL, "llama2")
	require.NoError(t, err)

	_, err = svc.Complete(context.Background(), []Message{{Role: "user", Content: "test"}}, DefaultCompletionOptions())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status 500")
}

// TestDefaultCompletionOptions tests default options.
func TestDefaultCompletionOptions(t *testing.T) {
	opts := DefaultCompletionOptions()
	assert.Equal(t, 0.7, opts.Temperature)
	assert.Equal(t, 2048, opts.MaxTokens)
	assert.False(t, opts.Stream)
}

// TestMessage tests Message struct.
func TestMessage(t *testing.T) {
	m := Message{
		Role:    "user",
		Content: "Hello",
	}
	assert.Equal(t, "user", m.Role)
	assert.Equal(t, "Hello", m.Content)
}

// TestQAService tests Q&A functionality.
func TestQAService(t *testing.T) {
	server := mockOllamaServer(t, "The authentication is handled in auth.go using JWT tokens.")
	defer server.Close()

	llmSvc, err := NewOllamaService(server.URL, "llama2")
	require.NoError(t, err)

	qaSvc := NewQAService(llmSvc)

	results := []search.Result{
		{
			FilePath:     "/path/to/auth.go",
			RelativePath: "auth.go",
			Content:      "func authenticate(token string) error { ... }",
			StartLine:    10,
			EndLine:      20,
			Score:        0.85,
		},
	}

	answer, err := qaSvc.Answer(context.Background(), "How does authentication work?", results, DefaultQAOptions())
	require.NoError(t, err)

	assert.NotEmpty(t, answer.Answer)
	assert.Len(t, answer.Sources, 1)
	assert.Equal(t, "auth.go", answer.Sources[0].RelativePath)
}

// TestQAServiceNoResults tests Q&A with no results.
func TestQAServiceNoResults(t *testing.T) {
	server := mockOllamaServer(t, "some response")
	defer server.Close()

	llmSvc, err := NewOllamaService(server.URL, "llama2")
	require.NoError(t, err)

	qaSvc := NewQAService(llmSvc)

	answer, err := qaSvc.Answer(context.Background(), "test question", nil, DefaultQAOptions())
	require.NoError(t, err)

	assert.Contains(t, answer.Answer, "couldn't find")
	assert.Nil(t, answer.Sources)
}

// TestQAServiceMaxContextChunks tests chunk limiting.
func TestQAServiceMaxContextChunks(t *testing.T) {
	server := mockOllamaServer(t, "Response based on limited context")
	defer server.Close()

	llmSvc, err := NewOllamaService(server.URL, "llama2")
	require.NoError(t, err)

	qaSvc := NewQAService(llmSvc)

	// Create many results
	results := make([]search.Result, 10)
	for i := range results {
		results[i] = search.Result{
			RelativePath: "file.go",
			Content:      "code content",
			StartLine:    i * 10,
			EndLine:      i*10 + 5,
			Score:        0.8,
		}
	}

	opts := DefaultQAOptions()
	opts.MaxContextChunks = 3

	answer, err := qaSvc.Answer(context.Background(), "test", results, opts)
	require.NoError(t, err)

	// Should only include 3 sources
	assert.Len(t, answer.Sources, 3)
}

// TestDefaultQAOptions tests default Q&A options.
func TestDefaultQAOptions(t *testing.T) {
	opts := DefaultQAOptions()
	assert.Equal(t, 0.3, opts.Temperature)
	assert.Equal(t, 2048, opts.MaxTokens)
	assert.False(t, opts.Stream)
	assert.Equal(t, 5, opts.MaxContextChunks)
}

// TestProviderConstants tests provider constants.
func TestProviderConstants(t *testing.T) {
	assert.Equal(t, Provider("ollama"), ProviderOllama)
	assert.Equal(t, Provider("openai"), ProviderOpenAI)
	assert.Equal(t, Provider("anthropic"), ProviderAnthropic)
}
