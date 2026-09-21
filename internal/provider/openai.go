package provider

import (
	"context"
	"errors"
	"net/http"
	"time"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// OpenAIProvider implements the Provider interface using the official OpenAI Go SDK.
type OpenAIProvider struct {
	APIKey string
	Client openai.Client
}

// NewOpenAIProvider creates a new OpenAI provider instance.
func NewOpenAIProvider(apiKey string) (*OpenAIProvider, error) {
	client := openai.NewClient(
		option.WithAPIKey(apiKey),
	)
	return &OpenAIProvider{
		APIKey: apiKey,
		Client: client,
	}, nil
}

// Send sends a prompt to the underlying model and returns the raw response as a string.
func (op *OpenAIProvider) Send(prompt string, model string) (*string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	chatCompletion, err := op.Client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: model,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(prompt),
		},
	})
	if err != nil {
		return nil, err
	}
	if chatCompletion == nil || chatCompletion.Choices == nil || len(chatCompletion.Choices) == 0 {
		return nil, errors.New("empty response from openai")
	}

	choice := chatCompletion.Choices[0]
	if choice.Message.Content == "" {
		return nil, errors.New("empty message content in openai response")
	}

	return &choice.Message.Content, nil
}