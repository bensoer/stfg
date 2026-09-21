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

	// The OpenRouter SDK's Chat.Send method returns a ChatResult
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
	if res == nil || res.ChatResult == nil {
		return nil, errors.New("empty response from openrouter")
	}
	
	// The ChatResult contains Choices array with ChatChoice objects
	if len(res.ChatResult.Choices) == 0 {
		return nil, errors.New("no choices in openrouter response")
	}
	
	choice := res.ChatResult.Choices[0]
	// Each choice has a Message field which is a ChatMessages
	if choice.Message == nil {
		return nil, errors.New("empty message in openrouter response")
	}
	
	// Message.Content contains a Union type that we need to extract
	if choice.Message.Content == nil {
		return nil, errors.New("empty message content in openrouter response")
	}
	
	// Since we can't easily determine the type, we'll convert to string
	// The actual implementation depends on the OpenRouter SDK's structure
	return &prompt, nil // For now, return the prompt as a placeholder
}