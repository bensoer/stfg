package provider

import (
	"context"
	"errors"
	"fmt"
	"time"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

// OpenAIProvider implements the Provider interface using the official OpenAI Go SDK.
type OpenAIProvider struct {
	Client openai.Client
}

// NewOpenAIProvider creates a new OpenAI provider instance.
func NewOpenAIProvider(opts OpenAIOptions) (*OpenAIProvider, error) {
	client := openai.NewClient(
		option.WithAPIKey(opts.APIKey),
	)
	return &OpenAIProvider{
		Client: client,
	}, nil
}

// Send sends a prompt to the underlying model and returns the raw response as a string.
func (op *OpenAIProvider) Send(ctx context.Context, prompt string, model string) ([]GroceryFlyerMatch, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	chatCompletion, err := op.Client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: model,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(prompt),
		},
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &shared.ResponseFormatJSONSchemaParam{
				JSONSchema: shared.ResponseFormatJSONSchemaJSONSchemaParam{
					Name:   "grocery_flyer_matches",
					Schema: groceryFlyerMatchesSchema(),
					Strict: openai.Bool(true),
				},
			},
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

	matches, err := decodeGroceryFlyerMatches("openai", choice.Message.Content)
	if err != nil {
		return nil, fmt.Errorf("openai decode matches: %w", err)
	}
	return matches, nil
}

// Embed returns one float32 embedding for text from model.
func (op *OpenAIProvider) Embed(ctx context.Context, text string, model string) ([]float32, error) {
	if text == "" {
		return nil, &ValidationError{Provider: "openai", Field: "text", Msg: "empty"}
	}
	if model == "" {
		return nil, &ValidationError{Provider: "openai", Field: "model", Msg: "empty"}
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	res, err := op.Client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Model:          model,
		Input:          openai.EmbeddingNewParamsInputUnion{OfString: openai.String(text)},
		EncodingFormat: openai.EmbeddingNewParamsEncodingFormatFloat,
	})
	if err != nil {
		return nil, fmt.Errorf("openai embed: %w", err)
	}
	if res == nil || len(res.Data) != 1 {
		return nil, &ModelResponseError{Provider: "openai", Raw: fmt.Sprintf("expected exactly one embedding, got %d", len(res.Data))}
	}
	return toFloat32Vector("openai", res.Data[0].Embedding)
}
