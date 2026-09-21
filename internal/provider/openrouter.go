package provider

import (
	"context"
	"errors"
	"fmt"
	"time"

	openrouter "github.com/OpenRouterTeam/go-sdk"
	"github.com/OpenRouterTeam/go-sdk/models/components"
)

// OpenRouterProvider implements the Provider interface using the official OpenRouter SDK.
type OpenRouterProvider struct {
	client *openrouter.OpenRouter
}

// NewOpenRouterProvider creates a new OpenRouter provider instance.
func NewOpenRouterProvider(apiKey string) (*OpenRouterProvider, error) {
	client := openrouter.New(
		openrouter.WithSecurity(apiKey),
	)
	return &OpenRouterProvider{
		client: client,
	}, nil
}

// Send sends a prompt to the underlying model and returns the raw response as a string.
func (op *OpenRouterProvider) Send(prompt string, model string) (*string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	res, err := op.client.Chat.Send(ctx, components.ChatRequest{
		Model: openrouter.Pointer(model),
		Messages: []components.ChatMessages{
			components.CreateChatMessagesUser(
				components.ChatUserMessage{
					Content: components.CreateChatUserMessageContentStr(prompt),
					Role:    components.ChatUserMessageRoleUser,
				},
			),
		},
	}, nil)
	if err != nil {
		return nil, err
	}
	if res == nil || res.ChatResult == nil || len(res.ChatResult.Choices) == 0 {
		return nil, errors.New("empty response from openrouter")
	}

	content, ok := res.ChatResult.Choices[0].GetMessage().Content.GetOrZero()
	if !ok {
		return nil, fmt.Errorf("unexpected response format: %v", res)
	}

	return content.Str, nil
}

// NewClient creates a backward-compatible provider for findDeals and promptwriter.
func NewClient(apiKey string) *OpenRouterProvider {
	p, _ := NewOpenRouterProvider(apiKey)
	return p
}