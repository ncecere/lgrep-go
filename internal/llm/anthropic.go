package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/charmbracelet/log"
)

const anthropicAPIURL = "https://api.anthropic.com/v1/messages"

// AnthropicService implements the LLM service using Anthropic Claude.
type AnthropicService struct {
	apiKey string
	model  string
	client *http.Client
}

// anthropicRequest is the request body for the Anthropic API.
type anthropicRequest struct {
	Model       string             `json:"model"`
	Messages    []anthropicMessage `json:"messages"`
	System      string             `json:"system,omitempty"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// anthropicResponse is the response from the Anthropic API.
type anthropicResponse struct {
	ID         string             `json:"id"`
	Type       string             `json:"type"`
	Role       string             `json:"role"`
	Content    []anthropicContent `json:"content"`
	StopReason string             `json:"stop_reason"`
}

type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// anthropicStreamEvent represents a streaming event.
type anthropicStreamEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index,omitempty"`
	Delta *struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta,omitempty"`
}

// NewAnthropicService creates a new Anthropic LLM service.
func NewAnthropicService(apiKey, model string) (*AnthropicService, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("Anthropic API key is required")
	}

	return &AnthropicService{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}, nil
}

// Complete generates a completion for the given messages.
func (s *AnthropicService) Complete(ctx context.Context, messages []Message, opts CompletionOptions) (string, error) {
	log.Debug("Requesting completion from Anthropic", "model", s.model)

	// Extract system message if present
	var systemMsg string
	var userMessages []anthropicMessage

	for _, m := range messages {
		if m.Role == "system" {
			systemMsg = m.Content
		} else {
			userMessages = append(userMessages, anthropicMessage{
				Role:    m.Role,
				Content: m.Content,
			})
		}
	}

	reqBody := anthropicRequest{
		Model:       s.model,
		Messages:    userMessages,
		System:      systemMsg,
		MaxTokens:   opts.MaxTokens,
		Temperature: opts.Temperature,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", anthropicAPIURL, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", s.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("anthropic returned status %d: %s", resp.StatusCode, string(body))
	}

	var result anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Content) == 0 {
		return "", fmt.Errorf("no content in response")
	}

	return result.Content[0].Text, nil
}

// CompleteStream generates a streaming completion.
func (s *AnthropicService) CompleteStream(ctx context.Context, messages []Message, opts CompletionOptions) (<-chan string, <-chan error) {
	contentCh := make(chan string, 100)
	errCh := make(chan error, 1)

	go func() {
		defer close(contentCh)
		defer close(errCh)

		// Extract system message if present
		var systemMsg string
		var userMessages []anthropicMessage

		for _, m := range messages {
			if m.Role == "system" {
				systemMsg = m.Content
			} else {
				userMessages = append(userMessages, anthropicMessage{
					Role:    m.Role,
					Content: m.Content,
				})
			}
		}

		reqBody := anthropicRequest{
			Model:       s.model,
			Messages:    userMessages,
			System:      systemMsg,
			MaxTokens:   opts.MaxTokens,
			Temperature: opts.Temperature,
			Stream:      true,
		}

		jsonBody, err := json.Marshal(reqBody)
		if err != nil {
			errCh <- fmt.Errorf("failed to marshal request: %w", err)
			return
		}

		req, err := http.NewRequestWithContext(ctx, "POST", anthropicAPIURL, bytes.NewReader(jsonBody))
		if err != nil {
			errCh <- fmt.Errorf("failed to create request: %w", err)
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", s.apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")

		resp, err := s.client.Do(req)
		if err != nil {
			errCh <- fmt.Errorf("failed to make request: %w", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			errCh <- fmt.Errorf("anthropic returned status %d: %s", resp.StatusCode, string(body))
			return
		}

		// Read SSE stream
		decoder := json.NewDecoder(resp.Body)
		for {
			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			default:
			}

			var event anthropicStreamEvent
			if err := decoder.Decode(&event); err != nil {
				if err == io.EOF {
					return
				}
				// Skip non-JSON lines (SSE format)
				continue
			}

			if event.Type == "content_block_delta" && event.Delta != nil && event.Delta.Text != "" {
				contentCh <- event.Delta.Text
			}

			if event.Type == "message_stop" {
				return
			}
		}
	}()

	return contentCh, errCh
}

// Provider returns the provider name.
func (s *AnthropicService) Provider() Provider {
	return ProviderAnthropic
}

// ModelName returns the model name.
func (s *AnthropicService) ModelName() string {
	return s.model
}
