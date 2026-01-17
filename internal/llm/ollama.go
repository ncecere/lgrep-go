package llm

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

// OllamaService implements the LLM service using Ollama.
type OllamaService struct {
	baseURL string
	model   string
	client  *http.Client
}

// ollamaChatRequest is the request body for the Ollama chat API.
type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  *ollamaOptions  `json:"options,omitempty"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
}

// ollamaChatResponse is the response from the Ollama chat API.
type ollamaChatResponse struct {
	Message       ollamaMessage `json:"message"`
	Done          bool          `json:"done"`
	DoneReason    string        `json:"done_reason,omitempty"`
	TotalDuration int64         `json:"total_duration,omitempty"`
}

// NewOllamaService creates a new Ollama LLM service.
func NewOllamaService(baseURL, model string) (*OllamaService, error) {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	return &OllamaService{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		model:   model,
		client: &http.Client{
			Timeout: 5 * time.Minute, // LLM calls can be slow
		},
	}, nil
}

// Complete generates a completion for the given messages.
func (s *OllamaService) Complete(ctx context.Context, messages []Message, opts CompletionOptions) (string, error) {
	// Convert messages
	ollamaMessages := make([]ollamaMessage, len(messages))
	for i, m := range messages {
		ollamaMessages[i] = ollamaMessage{
			Role:    m.Role,
			Content: m.Content,
		}
	}

	reqBody := ollamaChatRequest{
		Model:    s.model,
		Messages: ollamaMessages,
		Stream:   false,
		Options: &ollamaOptions{
			Temperature: opts.Temperature,
			NumPredict:  opts.MaxTokens,
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := s.baseURL + "/api/chat"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	log.Debug("Requesting completion from Ollama", "model", s.model)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	var result ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Message.Content, nil
}

// CompleteStream generates a streaming completion.
func (s *OllamaService) CompleteStream(ctx context.Context, messages []Message, opts CompletionOptions) (<-chan string, <-chan error) {
	contentCh := make(chan string, 100)
	errCh := make(chan error, 1)

	go func() {
		defer close(contentCh)
		defer close(errCh)

		// Convert messages
		ollamaMessages := make([]ollamaMessage, len(messages))
		for i, m := range messages {
			ollamaMessages[i] = ollamaMessage{
				Role:    m.Role,
				Content: m.Content,
			}
		}

		reqBody := ollamaChatRequest{
			Model:    s.model,
			Messages: ollamaMessages,
			Stream:   true,
			Options: &ollamaOptions{
				Temperature: opts.Temperature,
				NumPredict:  opts.MaxTokens,
			},
		}

		jsonBody, err := json.Marshal(reqBody)
		if err != nil {
			errCh <- fmt.Errorf("failed to marshal request: %w", err)
			return
		}

		url := s.baseURL + "/api/chat"
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
		if err != nil {
			errCh <- fmt.Errorf("failed to create request: %w", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := s.client.Do(req)
		if err != nil {
			errCh <- fmt.Errorf("failed to make request: %w", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			errCh <- fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(body))
			return
		}

		decoder := json.NewDecoder(resp.Body)
		for {
			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			default:
			}

			var chunk ollamaChatResponse
			if err := decoder.Decode(&chunk); err != nil {
				if err == io.EOF {
					return
				}
				errCh <- fmt.Errorf("failed to decode chunk: %w", err)
				return
			}

			if chunk.Message.Content != "" {
				contentCh <- chunk.Message.Content
			}

			if chunk.Done {
				return
			}
		}
	}()

	return contentCh, errCh
}

// Provider returns the provider name.
func (s *OllamaService) Provider() Provider {
	return ProviderOllama
}

// ModelName returns the model name.
func (s *OllamaService) ModelName() string {
	return s.model
}
