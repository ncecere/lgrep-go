package llm

import (
	"context"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// OpenAIService implements the LLM service using OpenAI.
type OpenAIService struct {
	client openai.Client
	model  string
}

// NewOpenAIService creates a new OpenAI LLM service.
func NewOpenAIService(apiKey, model, baseURL string) (*OpenAIService, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}

	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}

	client := openai.NewClient(opts...)

	return &OpenAIService{
		client: client,
		model:  model,
	}, nil
}

// Complete generates a completion for the given messages.
func (s *OpenAIService) Complete(ctx context.Context, messages []Message, opts CompletionOptions) (string, error) {
	log.Debug("Requesting completion from OpenAI", "model", s.model)

	// Convert messages to OpenAI format
	openaiMessages := make([]openai.ChatCompletionMessageParamUnion, len(messages))
	for i, m := range messages {
		switch m.Role {
		case "system":
			openaiMessages[i] = openai.SystemMessage(m.Content)
		case "user":
			openaiMessages[i] = openai.UserMessage(m.Content)
		case "assistant":
			openaiMessages[i] = openai.AssistantMessage(m.Content)
		default:
			openaiMessages[i] = openai.UserMessage(m.Content)
		}
	}

	resp, err := s.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:       openai.ChatModel(s.model),
		Messages:    openaiMessages,
		Temperature: openai.Float(opts.Temperature),
		MaxTokens:   openai.Int(int64(opts.MaxTokens)),
	})
	if err != nil {
		return "", fmt.Errorf("failed to create completion: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no completion returned")
	}

	return resp.Choices[0].Message.Content, nil
}

// CompleteStream generates a streaming completion.
func (s *OpenAIService) CompleteStream(ctx context.Context, messages []Message, opts CompletionOptions) (<-chan string, <-chan error) {
	contentCh := make(chan string, 100)
	errCh := make(chan error, 1)

	go func() {
		defer close(contentCh)
		defer close(errCh)

		// Convert messages to OpenAI format
		openaiMessages := make([]openai.ChatCompletionMessageParamUnion, len(messages))
		for i, m := range messages {
			switch m.Role {
			case "system":
				openaiMessages[i] = openai.SystemMessage(m.Content)
			case "user":
				openaiMessages[i] = openai.UserMessage(m.Content)
			case "assistant":
				openaiMessages[i] = openai.AssistantMessage(m.Content)
			default:
				openaiMessages[i] = openai.UserMessage(m.Content)
			}
		}

		stream := s.client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
			Model:       openai.ChatModel(s.model),
			Messages:    openaiMessages,
			Temperature: openai.Float(opts.Temperature),
			MaxTokens:   openai.Int(int64(opts.MaxTokens)),
		})

		for stream.Next() {
			chunk := stream.Current()
			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				contentCh <- chunk.Choices[0].Delta.Content
			}
		}

		if err := stream.Err(); err != nil {
			errCh <- err
		}
	}()

	return contentCh, errCh
}

// Provider returns the provider name.
func (s *OpenAIService) Provider() Provider {
	return ProviderOpenAI
}

// ModelName returns the model name.
func (s *OpenAIService) ModelName() string {
	return s.model
}
