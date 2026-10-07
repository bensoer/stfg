package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	openrouter "github.com/OpenRouterTeam/go-sdk"
	"github.com/OpenRouterTeam/go-sdk/models/components"
	"github.com/OpenRouterTeam/go-sdk/models/operations"
	"github.com/OpenRouterTeam/go-sdk/optionalnullable"
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
func (op *OpenRouterProvider) Send(ctx context.Context, prompt string, model string) ([]GroceryFlyerMatch, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	format := components.CreateResponseFormatJSONSchema(components.ChatFormatJSONSchemaConfig{
		JSONSchema: components.ChatJSONSchemaConfig{
			Name:        "grocery_flyer_matches",
			Description: openrouter.Pointer("Grocery items matched to flyer items"),
			Schema:      groceryFlyerMatchesSchema(),
			Strict:      optionalnullable.From(openrouter.Pointer(true)),
		},
	})

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
		ResponseFormat: &format,
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
	// Message.Content contains a Union type that we need to extract
	content, ok := choice.Message.GetContent().Get()
	if !ok || content == nil {
		return nil, errors.New("empty message content in openrouter response")
	}
	if content.Str == nil {
		encoded, marshalErr := json.Marshal(content)
		raw := ""
		if marshalErr == nil {
			raw = string(encoded)
		}
		return nil, fmt.Errorf("openrouter content is not a string: %w", &ModelResponseError{Provider: "openrouter", Raw: raw})
	}

	// Since we can't easily determine the type, we'll convert to string
	// The actual implementation depends on the OpenRouter SDK's structure
	// For now, return the prompt as a placeholder
	matches, err := decodeGroceryFlyerMatches("openrouter", *content.Str)
	if err != nil {
		return nil, fmt.Errorf("openrouter decode matches: %w", err)
	}
	return matches, nil
}

// Embed returns one float32 embedding for text from model.
func (op *OpenRouterProvider) Embed(ctx context.Context, text string, model string) ([]float32, error) {
	if text == "" {
		return nil, &ValidationError{Provider: "openrouter", Field: "text", Msg: "empty"}
	}
	if model == "" {
		return nil, &ValidationError{Provider: "openrouter", Field: "model", Msg: "empty"}
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	res, err := op.client.Embeddings.Generate(ctx, operations.CreateEmbeddingsRequest{
		Model:          model,
		Input:          operations.CreateInputUnionStr(text),
		EncodingFormat: operations.EncodingFormatFloat.ToPointer(),
	})
	if err != nil {
		return nil, fmt.Errorf("openrouter embed: %w", err)
	}
	if res == nil || res.CreateEmbeddingsResponseBody == nil {
		return nil, &ModelResponseError{Provider: "openrouter", Raw: "empty response"}
	}
	data := res.CreateEmbeddingsResponseBody.Data
	if len(data) != 1 {
		return nil, &ModelResponseError{Provider: "openrouter", Raw: fmt.Sprintf("expected exactly one embedding, got %d", len(data))}
	}
	switch data[0].Embedding.Type {
	case operations.EmbeddingTypeArrayOfNumber:
		return toFloat32Vector("openrouter", data[0].Embedding.ArrayOfNumber)
	default:
		return nil, &ModelResponseError{Provider: "openrouter", Raw: fmt.Sprintf("unsupported embedding type %q", data[0].Embedding.Type)}
	}
}
